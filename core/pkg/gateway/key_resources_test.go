package gateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Orama-Network/orama/core/pkg/gateway/auth"
)

// An API key narrowed to named functions holds those functions on every route
// that runs one, and nothing else of its scopes; a key with no resources is
// what it always was. The owner's case is one key for exactly these five,
// which share no single pattern.
var fiveFunctions = []string{
	"subscription-read",
	"subscription-intent-create",
	"subscription-record-payment",
	"user-find-by-username",
	"user-find-by-wallet",
}

// otherFunctions are names a key narrowed to fiveFunctions must not reach,
// including ones that only start like an allowed name.
var otherFunctions = []string{"user-delete", "subscription-read-all", "subscription", "admin-reset"}

func fiveFunctionResources() auth.KeyResources {
	out := make(auth.KeyResources, 0, len(fiveFunctions))
	for _, fn := range fiveFunctions {
		out = append(out, "fn:"+fn)
	}
	return out
}

// bareKeyHop is a request as the main gateway forwards it for a caller that
// presented an API key itself: the namespace, the key's scopes and its
// resources, and no token subject.
func bareKeyHop(t *testing.T, g *Gateway, method, path, scopes string, resources auth.KeyResources) *http.Request {
	t.Helper()
	r := httptest.NewRequest(method, path, nil)
	r.Header.Set(HeaderInternalAuthValidated, "true")
	r.Header.Set(HeaderInternalAuthNamespace, hopNamespace)
	setInternalAuthKeyHeaders(r.Header, keyAuthority{scopes: scopes, resources: resources})
	if err := signInternalAuthHeaders(g.internalAuthKey, r.Header, method, path, time.Now()); err != nil {
		t.Fatalf("sign: %v", err)
	}
	return r
}

// exchangedKeyHop is the same for a key exchanged for a token: the subject is
// the key, and its grant is resolved on routes that ask for one.
func exchangedKeyHop(t *testing.T, g *Gateway, method, path, scopes string, resources auth.KeyResources) *http.Request {
	t.Helper()
	r := httptest.NewRequest(method, path, nil)
	r.Header.Set(HeaderInternalAuthValidated, "true")
	r.Header.Set(HeaderInternalAuthNamespace, hopNamespace)
	r.Header.Set(HeaderInternalAuthJWTSub, "ak_exchanged_key")
	setInternalAuthKeyHeaders(r.Header, keyAuthority{scopes: scopes, resources: resources})
	if err := signInternalAuthHeaders(g.internalAuthKey, r.Header, method, path, time.Now()); err != nil {
		t.Fatalf("sign: %v", err)
	}
	return r
}

// invokePaths are every route that runs a function by name: both HTTP invoke
// routes and the function WebSocket.
func invokePaths(fn string) map[string]string {
	return map[string]string{
		"/v1/functions/<fn>/invoke": "/v1/functions/" + fn + "/invoke",
		"/v1/invoke/<ns>/<fn>":      "/v1/invoke/" + hopNamespace + "/" + fn,
		"/v1/functions/<fn>/ws":     "/v1/functions/" + fn + "/ws",
	}
}

func TestKeyResources_fiveFunctionKeyBehindTheHop(t *testing.T) {
	resources := fiveFunctionResources()
	for presented, build := range map[string]func(*testing.T, *Gateway, string, string, string, auth.KeyResources) *http.Request{
		"bare key":      bareKeyHop,
		"exchanged key": exchangedKeyHop,
	} {
		t.Run(presented, func(t *testing.T) {
			g, _ := namespaceGatewayForHops(t, "runtime")
			check := func(fn string, want int) {
				for route, path := range invokePaths(fn) {
					method := http.MethodPost
					if strings.HasSuffix(path, "/ws") {
						method = http.MethodGet
					}
					r := build(t, g, method, path, "invoke", resources)
					status, reached := serveHopAuthorizing(g, r, auth.Resource{Domain: auth.SelectorFn, Name: fn})
					if !reached || status != want {
						t.Errorf("%s %s (%s): reached %v, status %d, want %d", route, fn, presented, reached, status, want)
					}
				}
			}
			for _, fn := range fiveFunctions {
				check(fn, http.StatusOK)
			}
			for _, fn := range otherFunctions {
				check(fn, http.StatusForbidden)
			}
		})
	}
}

// A key with no resources is the whole of its scopes, on every invoke route.
func TestKeyResources_anUnnarrowedKeyIsUnchanged(t *testing.T) {
	g, _ := namespaceGatewayForHops(t, "runtime")
	for _, fn := range append(append([]string{}, fiveFunctions...), otherFunctions...) {
		for route, path := range invokePaths(fn) {
			method := http.MethodPost
			if strings.HasSuffix(path, "/ws") {
				method = http.MethodGet
			}
			r := bareKeyHop(t, g, method, path, "invoke", nil)
			if status, reached := serveHopAuthorizing(g, r, auth.Resource{Domain: auth.SelectorFn, Name: fn}); !reached || status != http.StatusOK {
				t.Errorf("%s %s: reached %v, status %d, want 200", route, fn, reached, status)
			}
		}
	}
}

// A glob reaches what it matches and nothing else.
func TestKeyResources_aPatternBehindTheHop(t *testing.T) {
	g, _ := namespaceGatewayForHops(t, "runtime")
	for fn, want := range map[string]int{
		"premium-status": http.StatusOK,
		"premium-":       http.StatusOK,
		"status-premium": http.StatusForbidden,
		"premium":        http.StatusForbidden,
	} {
		r := bareKeyHop(t, g, http.MethodPost, "/v1/functions/"+fn+"/invoke", "invoke", auth.KeyResources{"fn:premium-*"})
		if status, reached := serveHopAuthorizing(g, r, auth.Resource{Domain: auth.SelectorFn, Name: fn}); !reached || status != want {
			t.Errorf("%s: reached %v, status %d, want %d", fn, reached, status, want)
		}
	}
}

// Managing functions is fn:manage, which a narrowed key never holds: the
// trigger routes, deploys and logs are refused at the gate.
func TestKeyResources_aNarrowedKeyManagesNothing(t *testing.T) {
	g, _ := namespaceGatewayForHops(t, "runtime")
	for _, path := range []string{"/v1/functions/subscription-read/triggers", "/v1/functions"} {
		r := bareKeyHop(t, g, http.MethodPost, path, "invoke", fiveFunctionResources())
		if rec, reached := serveHop(g, r); reached || rec.Code != http.StatusForbidden {
			t.Errorf("POST %s: reached %v, status %d, want 403", path, reached, rec.Code)
		}
	}
}

// The resources are covered by the hop MAC: one edited or removed in flight
// fails verification, so a namespace gateway believes exactly what the main
// gateway read off the key's row.
func TestKeyResources_theHopMACCoversThem(t *testing.T) {
	g, _ := namespaceGatewayForHops(t, "runtime")
	for name, edit := range map[string]func(http.Header){
		"widened": func(h http.Header) { h.Set(HeaderInternalAuthKeyResources, "fn:*") },
		"removed": func(h http.Header) { h.Del(HeaderInternalAuthKeyResources) },
	} {
		r := bareKeyHop(t, g, http.MethodPost, "/v1/functions/user-delete/invoke", "invoke", fiveFunctionResources())
		edit(r.Header)
		if verifyInternalAuthHeaders(g.internalAuthKey, r, time.Now()) {
			t.Errorf("%s resources still verified", name)
		}
	}
}

// The main gateway's own chain, for a key presented to it directly: the row's
// resources narrow the key on the open invoke route as on the hop.
func TestKeyResources_aBareKeyOnTheMainGateway(t *testing.T) {
	g := chainGateway(t, "alice", &stubKeyDatabase{
		namespace: "alice", scopes: "invoke", resources: fiveFunctionResources().Stored(), found: true,
	})
	serveInvoke := func(fn string) int {
		rec := httptest.NewRecorder()
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := auth.AuthorizeResource(r.Context(), auth.Resource{Domain: auth.SelectorFn, Name: fn}); err != nil {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			w.WriteHeader(http.StatusOK)
		})
		r := chainRequest(http.MethodPost, "/v1/invoke/alice/"+fn, map[string]string{"X-API-Key": "ak_narrowed:alice"})
		g.routePolicyMiddleware(g.authMiddleware(g.authorizationMiddleware(g.scopeMiddleware(next)))).ServeHTTP(rec, r)
		return rec.Code
	}
	for _, fn := range fiveFunctions {
		if got := serveInvoke(fn); got != http.StatusOK {
			t.Errorf("%s: status %d, want 200", fn, got)
		}
	}
	for _, fn := range otherFunctions {
		if got := serveInvoke(fn); got != http.StatusForbidden {
			t.Errorf("%s: status %d, want 403", fn, got)
		}
	}
}

// What the main gateway forwards for a key: its scopes and its resources, read
// in the same lookup, for a bare key and for an exchanged one.
func TestKeyResources_theMainGatewayForwardsThem(t *testing.T) {
	stored := fiveFunctionResources().Stored()
	g := chainGateway(t, "alice", &stubKeyDatabase{namespace: "alice", scopes: "invoke", resources: stored, found: true})

	_, _, key, errMsg := g.validateAuthForNamespaceProxy(chainRequest(http.MethodPost, "/v1/invoke/alice/x",
		map[string]string{"X-API-Key": "ak_narrowed:alice"}))
	if errMsg != "" || key.scopes != "invoke" || key.resources.Stored() != stored {
		t.Errorf("bare key forwarded scopes=%q resources=%q err=%q", key.scopes, key.resources.Stored(), errMsg)
	}

	token, _, err := g.authService.GenerateJWT("alice", "ak_narrowed:alice", time.Hour, map[string]string{"scopes": "invoke"})
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	_, _, key, errMsg = g.validateAuthForNamespaceProxy(chainRequest(http.MethodPost, "/v1/invoke/alice/x",
		map[string]string{"Authorization": "Bearer " + token}))
	if errMsg != "" || key.resources.Stored() != stored {
		t.Errorf("exchanged key forwarded resources=%q err=%q", key.resources.Stored(), errMsg)
	}

	h := http.Header{}
	setInternalAuthKeyHeaders(h, key)
	if h.Get(HeaderInternalAuthKeyResources) != stored {
		t.Errorf("hop header = %q, want %q", h.Get(HeaderInternalAuthKeyResources), stored)
	}
	h = http.Header{}
	setInternalAuthKeyHeaders(h, keyAuthority{scopes: "invoke"})
	if _, present := h[http.CanonicalHeaderKey(HeaderInternalAuthKeyResources)]; present {
		t.Error("an unnarrowed key sent a resources header")
	}
}

// Creating a key refuses a resource the key could not be narrowed to, naming
// why, before anything is minted — the refusals `members add --resource` gives.
func TestCreateNamespaceKey_refusesResourcesTheKeyCannotBeNarrowedTo(t *testing.T) {
	g := &Gateway{}
	for name, body := range map[string]string{
		"a domain the key does not hold": `{"scope":"invoke-only","resource":"storage:avatars/*"}`,
		"a domain nothing applies":       `{"scope":"push","resources":["push:topic=x"]}`,
		"an admin key":                   `{"scope":"admin","resource":"fn:a"}`,
		"a scope no resource narrows":    `{"scope":"app-runtime","resources":["fn:a"]}`,
		"not a selector":                 `{"scope":"invoke-only","resource":"premium-status"}`,
	} {
		t.Run(name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/v1/namespace/keys", strings.NewReader(body))
			r = r.WithContext(context.WithValue(r.Context(), CtxKeyNamespaceOverride, "anchat"))
			rec := httptest.NewRecorder()
			g.createNamespaceKey(rec, r)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("%s: %d %s, want 400", body, rec.Code, rec.Body.String())
			}
		})
	}
}
