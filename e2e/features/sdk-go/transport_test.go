//go:build e2e_fleet

package sdkgo

import (
	"crypto/x509"
	"errors"
	"testing"

	"github.com/Orama-Network/orama/core/pkg/client"
	"github.com/Orama-Network/orama/e2e/features/internal/tenancy"
	"github.com/Orama-Network/orama/e2e/harness"
	"github.com/Orama-Network/orama/e2e/harness/ns"
)

// TestGoClient_httpClientCarriesTheGatewayCA: the fleet's gateway presents a
// Let's Encrypt staging certificate, which no system store trusts. Without
// ClientConfig.HTTPClient the storage and database clients refuse it; every
// other test of this package sends through runHTTPClient and reaches it, so
// the one option carries both sub-clients (bug-716, website/src/docs/developer/go-sdk.mdx
// "Gateway TLS").
func TestGoClient_httpClientCarriesTheGatewayCA(t *testing.T) {
	t.Parallel()
	n := tenancy.Namespace(t, harness.Fleet(t), ns.Options{})
	cfg := client.DefaultClientConfig(appName)
	cfg.GatewayURL, cfg.JWT = n.URL, n.Owner.Token()
	cfg.BootstrapPeers, cfg.QuietMode = nil, true
	c, err := client.NewClient(cfg)
	if err != nil {
		t.Fatalf("client.NewClient: %v", err)
	}
	if err := c.Connect(); err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	t.Cleanup(func() { _ = c.Disconnect() })

	var unknownAuthority x509.UnknownAuthorityError
	if _, err := c.Storage().Status(t.Context(), "QmUnknownAuthorityProbe"); !errors.As(err, &unknownAuthority) {
		t.Errorf("storage with no HTTPClient: err %v, want the staging CA refused", err)
	}
	if _, err := c.Database().Query(t.Context(), "SELECT 1"); !errors.As(err, &unknownAuthority) {
		t.Errorf("database with no HTTPClient: err %v, want the staging CA refused", err)
	}

	trusted := newClient(t, n, owner(n))
	if _, err := trusted.Database().Query(t.Context(), "SELECT 1"); err != nil {
		t.Errorf("database through the run's HTTPClient: %v", err)
	}
}
