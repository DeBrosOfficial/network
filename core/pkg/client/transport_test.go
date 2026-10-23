package client

import (
	"context"
	"crypto/x509"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// tlsGateway is a gateway behind a certificate no system root signed, as a
// staging (Let's Encrypt staging) or private-CA gateway is. It answers the
// storage status and database query routes.
func tlsGateway(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/storage/status/QmStatus":
			_, _ = w.Write([]byte(`{"cid":"QmStatus","status":"pinned"}`))
		case "/v1/rqlite/query":
			_, _ = w.Write([]byte(`{"columns":["n"],"items":[{"n":1}],"count":1}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func clientFor(cfg *ClientConfig) *Client {
	c := &Client{config: cfg, connected: true}
	c.storage = &StorageClientImpl{client: c}
	c.gatewayDatabase = newGatewayDatabaseClient(c)
	return c
}

// bug-716: every request to the gateway built its own http.Client, so a Go
// program could not trust a private or staging CA. HTTPClient's Transport now
// carries storage and the database alike.
func TestHTTPClient_reachesAGatewayBehindAPrivateCA(t *testing.T) {
	gw := tlsGateway(t)
	c := clientFor(&ClientConfig{AppName: "app", APIKey: "ak_test:app", GatewayURL: gw.URL, HTTPClient: gw.Client()})
	ctx := context.Background()

	status, err := c.Storage().Status(ctx, "QmStatus")
	if err != nil {
		t.Fatalf("storage through the injected transport: %v", err)
	}
	if status.Status != "pinned" {
		t.Errorf("status %q, want pinned", status.Status)
	}
	res, err := c.Database().Query(ctx, "SELECT 1 AS n")
	if err != nil {
		t.Fatalf("database through the injected transport: %v", err)
	}
	if res.Count != 1 {
		t.Errorf("count %d, want 1", res.Count)
	}
}

// Without it the system roots decide, and they do not hold the gateway's CA.
func TestHTTPClient_unsetTrustsTheSystemRootsOnly(t *testing.T) {
	gw := tlsGateway(t)
	c := clientFor(&ClientConfig{AppName: "app", APIKey: "ak_test:app", GatewayURL: gw.URL})
	ctx := context.Background()

	var unknownAuthority x509.UnknownAuthorityError
	if _, err := c.Storage().Status(ctx, "QmStatus"); !errors.As(err, &unknownAuthority) {
		t.Errorf("storage: err %v, want an unknown-authority TLS error", err)
	}
	if _, err := c.Database().Query(ctx, "SELECT 1 AS n"); !errors.As(err, &unknownAuthority) {
		t.Errorf("database: err %v, want an unknown-authority TLS error", err)
	}
}

// The injected client's CheckRedirect does not apply: a redirect is refused
// for storage as for the database, because Go forwards X-API-Key on a
// cross-host hop.
func TestHTTPClient_storageRefusesARedirect(t *testing.T) {
	followed := false
	elsewhere := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { followed = true }))
	defer elsewhere.Close()
	gw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, elsewhere.URL+r.URL.Path, http.StatusTemporaryRedirect)
	}))
	defer gw.Close()
	c := clientFor(&ClientConfig{AppName: "app", APIKey: "ak_secret:app", GatewayURL: gw.URL, HTTPClient: &http.Client{}})

	_, err := c.Storage().Status(context.Background(), "QmStatus")
	var statusErr *gatewayStatusError
	if !errors.As(err, &statusErr) || statusErr.status != http.StatusTemporaryRedirect || !strings.Contains(err.Error(), "redirected") {
		t.Fatalf("err %v, want the redirect refused as a status error", err)
	}
	if followed {
		t.Fatal("the request, with its key, followed the redirect")
	}
}
