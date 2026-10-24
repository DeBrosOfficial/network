//go:build e2e_fleet

package deployments

import (
	"net/url"
	"testing"

	"github.com/Orama-Network/orama/core/pkg/clustertrust"
	"github.com/Orama-Network/orama/e2e/features/internal/tenancy"
)

// TestDeployTrust_theRuntimeTrustsTheClusterGateway: on a fleet whose
// certificates come from Let's Encrypt staging (the run trusts a CA file), the
// platform points every runtime at the cluster's roots, so an app that brings
// no CA calls ORAMA_GATEWAY_URL with its default TLS client; on a fleet the
// system roots verify, nothing is set (website/src/docs/developer/deployments.mdx
// "What the platform sets for you"; docs/whitepaper/technical-reference/vol1/11-app-deployments.md
// "The cluster's roots"). The probe app carries no ca.pem, so its /renew is the
// call that proves the trust works.
func TestDeployTrust_theRuntimeTrustsTheClusterGateway(t *testing.T) {
	t.Parallel()
	tn := newTenant(t)
	u := tn.deploy(t, "go", tenancy.WriteProbeApp(t, "trust"), "trust")
	c := tn.app(u)
	serving(t, c, "/health", "")

	staging := tn.f.State.CAFile != ""
	for _, key := range []string{"SSL_CERT_FILE", "NODE_EXTRA_CA_CERTS"} {
		a := probe(t, c, "/getenv", url.Values{"k": {key}})
		switch {
		case staging && (!a.OK || a.Detail != clustertrust.NodePath):
			t.Errorf("%s: set %v to %q, want %s", key, a.OK, a.Detail, clustertrust.NodePath)
		case !staging && a.OK:
			t.Errorf("%s = %q on a fleet the system roots verify", key, a.Detail)
		}
	}
	if staging {
		if a := probe(t, c, "/read", url.Values{"p": {clustertrust.NodePath}}); !a.OK {
			t.Errorf("the app cannot read the cluster's roots: %s", a.Detail)
		}
	}
	if a := probe(t, c, "/renew", nil); !a.OK {
		t.Fatalf("an app with no CA of its own could not call its gateway: %s", a.Detail)
	}
}
