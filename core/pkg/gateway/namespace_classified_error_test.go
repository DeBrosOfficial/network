package gateway

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Orama-Network/orama/core/pkg/httputil"
)

// bug-429: a storage read answers a classified 503 SERVICE_UNAVAILABLE while
// the node's IPFS Cluster peer restarts, or 504 TIMEOUT. Five of them opened
// the namespace's breaker on that member, and its database and function
// traffic failed over or got 503 for 30 s. A classified error is the
// gateway's verdict on one request, not the gateway failing.
func TestNamespaceProxy_classifiedStorageErrorsDoNotOpenTheCircuit(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/storage/get/bafy-restarting":
			httputil.WriteRPCError(w, http.StatusServiceUnavailable, httputil.ErrCodeServiceUnavailable, "the cluster peer is restarting")
		case "/v1/storage/get/bafy-slow":
			httputil.WriteRPCError(w, http.StatusGatewayTimeout, httputil.ErrCodeTimeout, "retrieval timed out")
		default:
			// The readiness gate: a gateway that cannot serve at all.
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = io.WriteString(w, `{"status":"blocked","reason":"schema-version","error":"gateway is blocked"}`)
		}
	}))
	defer upstream.Close()
	g := proxyGateway(t, gatewayTarget{ip: "127.0.0.1", port: serverPort(upstream)})

	for i := 0; i < 3*defaultFailureThreshold; i++ {
		path := "/v1/storage/get/bafy-restarting"
		if i%2 == 1 {
			path = "/v1/storage/get/bafy-slow"
		}
		rec := proxyOnce(g, "acme", get(path))
		if rec.Code != http.StatusServiceUnavailable && rec.Code != http.StatusGatewayTimeout {
			t.Fatalf("request %d: status %d, want the storage error passed through", i, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), `"code":"`) {
			t.Fatalf("request %d: the client got %q, not the whole envelope", i, rec.Body.String())
		}
	}
	if st := g.circuitBreakers.ForNamespaceGateway("acme", "127.0.0.1").State(); st != CircuitClosed {
		t.Fatalf("circuit is %v after %d classified storage errors, want closed", st, 3*defaultFailureThreshold)
	}

	// A 503 outside the envelope still says the gateway cannot serve.
	for i := 0; i < defaultFailureThreshold; i++ {
		rec := proxyOnce(g, "acme", get("/v1/functions"))
		if !strings.Contains(rec.Body.String(), "gateway is blocked") {
			t.Fatalf("the readiness refusal did not reach the client intact: %q", rec.Body.String())
		}
	}
	if st := g.circuitBreakers.ForNamespaceGateway("acme", "127.0.0.1").State(); st != CircuitOpen {
		t.Fatalf("circuit is %v after the gateway refused %d times outside the envelope, want open", st, defaultFailureThreshold)
	}
}

func TestIsNamespaceGatewayFailure(t *testing.T) {
	mk := func(status int, body string) *http.Response {
		return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}
	}
	envelope := `{"ok":false,"error":{"code":"SERVICE_UNAVAILABLE","message":"x","retryable":true}}`
	for _, tc := range []struct {
		name string
		resp *http.Response
		want bool
	}{
		{"classified 503", mk(503, envelope), false},
		{"classified 504", mk(504, `{"ok":false,"error":{"code":"TIMEOUT","message":"x"}}`), false},
		{"readiness 503", mk(503, `{"status":"starting","error":"gateway is starting"}`), true},
		{"plain-text 502", mk(502, "Bad Gateway"), true},
		{"envelope without a code", mk(503, `{"ok":false,"error":{"message":"x"}}`), true},
		{"empty 503", mk(503, ""), true},
		{"oversized 503", mk(503, strings.Repeat(" ", maxClassifiedErrorBytes)+envelope), true},
		{"200", mk(200, envelope), false},
	} {
		if got := isNamespaceGatewayFailure(tc.resp); got != tc.want {
			t.Errorf("%s: isNamespaceGatewayFailure = %v, want %v", tc.name, got, tc.want)
		}
	}
	// The replayed body is the original, byte for byte.
	resp := mk(503, envelope)
	isNamespaceGatewayFailure(resp)
	if got, _ := io.ReadAll(resp.Body); string(got) != envelope {
		t.Fatalf("replayed body = %q, want %q", got, envelope)
	}
}
