package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Orama-Network/orama/core/pkg/gateway/auth"
)

// bugboard bug-55: the node routes declared operator:read and operator:write,
// a namespace owner's or admin's `*:*:*` held both, and the node proxy handlers
// checked nothing, so any namespace owner or admin key could command a node,
// read its logs or drop it from the mesh. The invite asked for `*:*:*` itself.
// Every operator route now asks the operator list in the scope gate, and
// nothing a namespace grants is an answer to it.

const (
	gateOperator  = "0x00000000000000000000000000000000000000aa"
	gateTenant    = "0x00000000000000000000000000000000000000bb"
	gateNamespace = "operatorsns"
)

// operatorRoutePaths is a concrete path for every route the table declares an
// operator's, read from the table so a route added later is covered here too.
func operatorRoutePaths(t *testing.T) []string {
	t.Helper()
	var paths []string
	for _, pattern := range gatewayRoutes.Patterns() {
		path := pattern
		if strings.HasSuffix(path, "/") {
			path += "x"
		}
		if gatewayRoutes.For(httptest.NewRequest(http.MethodPost, path, nil)).Operator {
			paths = append(paths, path)
		}
	}
	if len(paths) == 0 {
		t.Fatal("the route table declares no operator route")
	}
	return paths
}

// TestOperatorRoutes_theClusterRoutesAreAllOperatorRoutes pins which routes
// are the operator's: every node-management, network-mutation and
// /v1/operator route. A route left out here would be judged by a namespace's
// permissions again.
func TestOperatorRoutes_theClusterRoutesAreAllOperatorRoutes(t *testing.T) {
	for _, path := range []string{
		"/v1/node/status", "/v1/node/logs", "/v1/node/command", "/v1/node/leave",
		"/v1/network/status", "/v1/network/peers", "/v1/network/connect", "/v1/network/disconnect",
		"/v1/operator/health", "/v1/operator/telemetry", "/v1/operator/telemetry/stream",
		"/v1/operator/nodes", "/v1/operator/node/register",
		"/v1/operator/operators", "/v1/operator/operators/0xabc",
		"/v1/operator/rotate-signing-key", "/v1/operator/rotate-secrets",
		"/v1/operator/invite", "/v1/operator/settings", "/v1/operator/settings/max-namespaces-per-wallet",
		"/v1/operator/creators", "/v1/operator/creators/0xabc", "/v1/operator/namespaces/remove",
	} {
		p := gatewayRoutes.For(httptest.NewRequest(http.MethodPost, path, nil))
		if !p.Operator {
			t.Errorf("%s is not an operator route: a namespace's permissions decide who reaches it", path)
		}
		if p.Domain != "" || p.Action != "" {
			t.Errorf("%s still names the permission %s:%s, which some namespace credential can hold", path, p.Domain, p.Action)
		}
		if p.Access.Anonymous() {
			t.Errorf("%s is reachable without a credential", path)
		}
	}
}

// serveOperatorGate runs one request through the scope gate and reports what
// came back; the handler behind it answers 418.
func serveOperatorGate(t *testing.T, g *Gateway, path string, ctx func(context.Context) context.Context) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, path, nil)
	r = r.WithContext(ctx(r.Context()))
	rec := httptest.NewRecorder()
	g.scopeMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})).ServeHTTP(rec, r)
	return rec
}

// Every credential a namespace hands out is refused on every operator route,
// though each holds `*:*:*` in its namespace, and though that namespace is
// owned by an operator.
func TestOperatorRoutes_noNamespaceCredentialIsAnOperator(t *testing.T) {
	g, _ := registryGateway(t, "index", gateOperator)
	callers := map[string]func(context.Context) context.Context{
		"the namespace owner's session": func(ctx context.Context) context.Context {
			ctx = context.WithValue(ctx, CtxKeyNamespaceOverride, gateNamespace)
			ctx = context.WithValue(ctx, ctxKeyJWT, &auth.JWTClaims{Sub: gateTenant, Namespace: gateNamespace})
			return context.WithValue(ctx, ctxKeyGrant, &auth.Grant{
				PrincipalType: auth.PrincipalWallet, Identifier: gateTenant, Role: auth.RoleOwner})
		},
		"a namespace admin's session": func(ctx context.Context) context.Context {
			ctx = context.WithValue(ctx, CtxKeyNamespaceOverride, gateNamespace)
			ctx = context.WithValue(ctx, ctxKeyJWT, &auth.JWTClaims{Sub: gateTenant, Namespace: gateNamespace})
			return context.WithValue(ctx, ctxKeyGrant, &auth.Grant{
				PrincipalType: auth.PrincipalWallet, Identifier: gateTenant, Role: auth.RoleAdmin})
		},
		"an admin key of an operator's namespace": func(ctx context.Context) context.Context {
			ctx = context.WithValue(ctx, CtxKeyNamespaceOverride, gateNamespace)
			return context.WithValue(ctx, ctxKeyScopes, auth.ScopeSet{auth.ScopeAdmin: {}})
		},
		"that admin key exchanged for a token": func(ctx context.Context) context.Context {
			ctx = context.WithValue(ctx, CtxKeyNamespaceOverride, gateNamespace)
			ctx = context.WithValue(ctx, ctxKeyScopes, auth.ScopeSet{auth.ScopeAdmin: {}})
			return context.WithValue(ctx, ctxKeyJWT, &auth.JWTClaims{Sub: "orama_sk_abc_def", Namespace: gateNamespace})
		},
		"an app's workload token": func(ctx context.Context) context.Context {
			ctx = context.WithValue(ctx, CtxKeyNamespaceOverride, gateNamespace)
			return context.WithValue(ctx, ctxKeyJWT, &auth.JWTClaims{Sub: "app:" + gateNamespace + "/web", Namespace: gateNamespace})
		},
	}
	for _, path := range operatorRoutePaths(t) {
		for who, ctx := range callers {
			rec := serveOperatorGate(t, g, path, ctx)
			if rec.Code != http.StatusForbidden {
				t.Errorf("%s as %s: %d %s, want 403", path, who, rec.Code, rec.Body)
				continue
			}
			var body struct {
				Code string `json:"code"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Code != CodeOperatorRequired {
				t.Errorf("%s as %s: code %q (%v), want %s", path, who, body.Code, err, CodeOperatorRequired)
			}
		}
	}
}

// The regression the finding is about, stated on the permission set itself:
// what an owner and an admin hold is everything, and it is not the operator's.
func TestOperatorRoutes_everythingInANamespaceIsNotTheCluster(t *testing.T) {
	for _, role := range []auth.Role{auth.RoleOwner, auth.RoleAdmin} {
		if !role.Permissions().IsAdmin() {
			t.Fatalf("%s no longer holds *:*:*; this test no longer models the finding", role)
		}
	}
	g, _ := registryGateway(t, "index", gateOperator)
	rec := serveOperatorGate(t, g, "/v1/node/command", func(ctx context.Context) context.Context {
		ctx = context.WithValue(ctx, CtxKeyNamespaceOverride, gateNamespace)
		ctx = context.WithValue(ctx, ctxKeyJWT, &auth.JWTClaims{Sub: gateTenant, Namespace: gateNamespace})
		return context.WithValue(ctx, ctxKeyGrant, &auth.Grant{
			PrincipalType: auth.PrincipalWallet, Identifier: gateTenant, Role: auth.RoleOwner})
	})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("an owner's *:*:* reached /v1/node/command: %d %s", rec.Code, rec.Body)
	}
}

// A wallet on the operator list reaches every operator route, whether its
// session stands in the lobby, where it holds nothing, or in a namespace it
// owns.
func TestOperatorRoutes_aRegisteredOperatorReachesThem(t *testing.T) {
	g, _ := registryGateway(t, "index", gateOperator)
	sessions := map[string]func(context.Context) context.Context{
		"a lobby session": func(ctx context.Context) context.Context {
			ctx = context.WithValue(ctx, CtxKeyNamespaceOverride, auth.LobbyNamespace)
			return context.WithValue(ctx, ctxKeyJWT, &auth.JWTClaims{Sub: gateOperator, Namespace: auth.LobbyNamespace})
		},
		"a session in a namespace it owns": func(ctx context.Context) context.Context {
			ctx = context.WithValue(ctx, CtxKeyNamespaceOverride, gateNamespace)
			ctx = context.WithValue(ctx, ctxKeyJWT, &auth.JWTClaims{Sub: gateOperator, Namespace: gateNamespace})
			return context.WithValue(ctx, ctxKeyGrant, &auth.Grant{
				PrincipalType: auth.PrincipalWallet, Identifier: gateOperator, Role: auth.RoleOwner})
		},
	}
	for _, path := range operatorRoutePaths(t) {
		for who, ctx := range sessions {
			if rec := serveOperatorGate(t, g, path, ctx); rec.Code != http.StatusTeapot {
				t.Errorf("%s as an operator in %s: %d %s, want the handler", path, who, rec.Code, rec.Body)
			}
		}
	}
}

// Not knowing the operator list is not an answer: the gate refuses with a
// retryable 503 and never reaches the handler.
func TestOperatorRoutes_anUnreadableListRefuses(t *testing.T) {
	g, db := registryGateway(t, "index", gateOperator)
	db.failQuery = true
	rec := serveOperatorGate(t, g, "/v1/node/logs", func(ctx context.Context) context.Context {
		return context.WithValue(ctx, ctxKeyJWT, &auth.JWTClaims{Sub: gateOperator})
	})
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("an unreadable operator list answered %d %s, want 503", rec.Code, rec.Body)
	}
}
