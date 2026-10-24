package gateway

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// keyHop is a request as the main gateway forwards it for an API key: the
// scopes on the key's row travel with it.
func keyHop(t *testing.T, g *Gateway, method, path, scopes string) *http.Request {
	t.Helper()
	r := httptest.NewRequest(method, path, nil)
	r.Header.Set(HeaderInternalAuthValidated, "true")
	r.Header.Set(HeaderInternalAuthNamespace, hopNamespace)
	r.Header.Set(HeaderInternalAuthJWTSub, "ak_exchanged_key")
	r.Header.Set(HeaderInternalAuthScopes, scopes)
	if err := signInternalAuthHeaders(g.internalAuthKey, r.Header, method, path, time.Now()); err != nil {
		t.Fatalf("sign: %v", err)
	}
	return r
}

// Every key that is not admin is given a runtime grant when it is minted, and
// a runtime role is the whole data plane. On a route that resolves the grant —
// publishing does — a key minted for `invoke`, or for the cache alone, was
// therefore let publish. A key reaches what its own scopes say.
func TestKeyScopes_boundTheGrantOnRoutesThatResolveIt(t *testing.T) {
	const (
		appRuntime = "invoke,proxy,push,webrtc" // the app-runtime profile
		cacheOnly  = "cache"
		admin      = "admin"
	)
	cases := []struct {
		name, scopes, path string
		want               int
	}{
		{"an app-runtime key cannot publish", appRuntime, "/v1/pubsub/publish", http.StatusForbidden},
		{"a cache key cannot publish", cacheOnly, "/v1/pubsub/publish", http.StatusForbidden},
		{"a cache key reaches the cache", cacheOnly, "/v1/cache/put", http.StatusOK},
		{"an app-runtime key reaches push", appRuntime, "/v1/push/devices", http.StatusOK},
		{"an admin key publishes", admin, "/v1/pubsub/publish", http.StatusOK},
		{"an admin key reaches the control plane", admin, "/v1/functions", http.StatusOK},
		{"a runtime key does not reach the control plane", appRuntime, "/v1/functions", http.StatusForbidden},
		{"a key with no scopes reaches nothing", "unknown-word", "/v1/pubsub/publish", http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			role := "runtime"
			if tc.scopes == admin {
				role = "admin"
			}
			g, _ := namespaceGatewayForHops(t, role)

			rec, _ := serveHop(g, keyHop(t, g, http.MethodPost, tc.path, tc.scopes))

			if rec.Code != tc.want {
				t.Errorf("%s %s: status %d, want %d: %s", tc.scopes, tc.path, rec.Code, tc.want, rec.Body.String())
			}
		})
	}
}

// A token exchanged for a key may unpin (bugboard #151), and the handler
// reaches every CID the namespace owns, so the storage grant is the whole of
// what keeps a key out. The key an app ships is the app-runtime profile, which
// holds none; the storage key a function keeps in its namespace's secrets
// does (bugboard #946).
func TestKeyScopes_onlyAStorageKeyUnpins(t *testing.T) {
	const unpinPath = "/v1/storage/unpin/QmE2E"
	cases := []struct {
		name, scopes string
		want         int
	}{
		{"an app-runtime key's token cannot unpin", "invoke,proxy,push,webrtc", http.StatusForbidden},
		{"an invoke-only key's token cannot unpin", "invoke", http.StatusForbidden},
		{"a function's storage key's token unpins", "storage", http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g, _ := namespaceGatewayForHops(t, "runtime")

			rec, reached := serveHop(g, keyHop(t, g, http.MethodDelete, unpinPath, tc.scopes))

			if rec.Code != tc.want || reached != (tc.want == http.StatusOK) {
				t.Errorf("%s: status %d (handler reached %v), want %d: %s",
					tc.scopes, rec.Code, reached, tc.want, rec.Body.String())
			}
		})
	}
}

// An app that needs nothing before sign-in ships no key (bugboard #946), so
// the namespace gateway has to serve it everything it does before then with no
// credential at all: the sign-in handshake, the refresh that keeps the session,
// and a public function. Each of these reaches its handler with nothing on the
// request.
func TestNamespaceGateway_servesAKeylessClient(t *testing.T) {
	for _, req := range []struct{ method, path string }{
		{http.MethodPost, "/v1/auth/challenge"},
		{http.MethodPost, "/v1/auth/verify"},
		{http.MethodPost, "/v1/auth/refresh"},
		{http.MethodPost, "/v1/functions/username-check/invoke"},
		{http.MethodPost, "/v1/invoke/" + hopNamespace + "/username-check"},
	} {
		t.Run(req.path, func(t *testing.T) {
			g, _ := namespaceGatewayForHops(t, "runtime")

			rec, reached := serveHop(g, httptest.NewRequest(req.method, req.path, nil))

			if !reached {
				t.Errorf("%s %s with no credential did not reach its handler: %d %s",
					req.method, req.path, rec.Code, rec.Body.String())
			}
		})
	}

	// The same chain still refuses a keyless caller everything else, so the
	// answers above are the routes' policies and not a chain that lets all in.
	g, _ := namespaceGatewayForHops(t, "runtime")
	if rec, reached := serveHop(g, httptest.NewRequest(http.MethodPost, "/v1/storage/upload", nil)); reached {
		t.Errorf("an upload with no credential reached its handler: %d", rec.Code)
	}
}
