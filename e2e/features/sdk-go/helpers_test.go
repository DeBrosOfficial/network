//go:build e2e_fleet

package sdkgo

import (
	"crypto/tls"
	"net/http"
	"testing"
	"time"

	"github.com/Orama-Network/orama/e2e/harness"
	"github.com/Orama-Network/orama/e2e/harness/gw"
	"github.com/Orama-Network/orama/e2e/harness/ns"
	"github.com/Orama-Network/orama/core/pkg/client"
)

// Budgets for one SDK call against the fleet.
const (
	callBudget = time.Minute
	pollEvery  = 2 * time.Second
	// pinBudget: a fresh upload is pinned cluster-wide within this.
	pinBudget = 2 * time.Minute
	// failFastBudget bounds a call that must fail because its transport
	// does not exist here, rather than hang.
	failFastBudget = 30 * time.Second
	// appName names the SDK client in its logs and default database.
	appName = "e2e-sdk-go"
)

// runHTTPClient is the client the SDK sends through: one that trusts the
// run's CA roots and nothing else (ClientConfig.HTTPClient). The fleet's
// certificates come from Let's Encrypt staging, which no system store trusts.
func runHTTPClient(t testing.TB) *http.Client {
	t.Helper()
	pool, err := gw.LoadCAPool(harness.Fleet(t).State.CAFile)
	if err != nil {
		t.Fatalf("failed to load the run's CA roots: %v", err)
	}
	return &http.Client{Transport: &http.Transport{
		Proxy:           http.ProxyFromEnvironment,
		TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12},
	}}
}

// credential is how an SDK client authenticates.
type credential struct {
	jwt, apiKey string
}

// newClient is an SDK client for n's gateway with cred, configured the way
// website/src/docs/developer/go-sdk.mdx "Quick Start" does, connected, and disconnected at
// cleanup. No bootstrap peers are dialled: none is reachable from outside.
func newClient(t testing.TB, n *ns.Namespace, cred credential) client.NetworkClient {
	t.Helper()
	cfg := client.DefaultClientConfig(appName)
	cfg.GatewayURL = n.URL
	cfg.HTTPClient = runHTTPClient(t)
	cfg.JWT, cfg.APIKey = cred.jwt, cred.apiKey
	cfg.BootstrapPeers = nil
	cfg.QuietMode = true
	c, err := client.NewClient(cfg)
	if err != nil {
		t.Fatalf("client.NewClient: %v", err)
	}
	if err := c.Connect(); err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	t.Cleanup(func() {
		if err := c.Disconnect(); err != nil {
			t.Errorf("cleanup: client.Disconnect: %v", err)
		}
	})
	return c
}

func owner(n *ns.Namespace) credential { return credential{jwt: n.Owner.Token()} }
