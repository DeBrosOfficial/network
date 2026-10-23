//go:build e2e_fleet

package vault

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Orama-Network/orama/e2e/harness"
	"github.com/Orama-Network/orama/e2e/harness/eventually"
)

const (
	vaultUnit = "orama-namespace-vault@index.service"
	// guardianPort is the guardian's client port in the code
	// (core/pkg/constants VaultHTTPPort = 10106); the binary's own default is 7500.
	guardianPort = 10106
	// peerPort is the peer protocol port; docs/whitepaper/technical-reference/vol1/28-vault.md
	// (Known gaps) says nothing listens on it yet.
	peerPort = 7501
)

// TestGuardian_unitAndOverlayOnly: every node runs the guardian, which is
// not reachable on a public address (docs/whitepaper/technical-reference/vol1/28-vault.md, Trust and security: overlay only).
func TestGuardian_unitAndOverlayOnly(t *testing.T) {
	t.Parallel()
	f := harness.Fleet(t)
	for _, n := range f.State.Nodes {
		if s := f.Unit(t, n, vaultUnit); s != "active" {
			t.Errorf("%s: %s is %q", n.Name, vaultUnit, s)
		}
		found := false
		for _, l := range f.Listeners(t, n) {
			if l.Port != guardianPort {
				continue
			}
			found = true
			if l.Public() || l.Addr == n.PublicIP {
				t.Errorf("%s: the guardian listens on %s", n.Name, l.Addr)
			}
		}
		if !found {
			t.Errorf("%s: nothing listens on the guardian port %d", n.Name, guardianPort)
		}
	}
}

// guardianList is a guardian's GET /v1/vault/guardians answer.
type guardianList struct {
	Guardians []struct {
		Address string `json:"address"`
		Port    int    `json:"port"`
	} `json:"guardians"`
	Threshold int `json:"threshold"`
	Total     int `json:"total"`
}

// TestGuardian_discoversTheClusterFromRqlite: every guardian reads the
// cluster's active nodes from the index RQLite (dns_nodes, authenticated as
// the cluster's RQLite user) and counts each as a guardian; it lists itself as
// alive, because liveness of the others needs the peer protocol
// (docs/whitepaper/technical-reference/vol1/28-vault.md, "Guardian membership,
// heartbeats and the peer protocol"). The list is re-read every 30 s.
func TestGuardian_discoversTheClusterFromRqlite(t *testing.T) {
	t.Parallel()
	f := harness.Fleet(t)
	for _, n := range f.State.Nodes {
		base := fmt.Sprintf("http://%s:%d", n.WGIP, guardianPort)
		eventually.Require(t, 5*time.Second, 2*time.Minute, n.Name+": the guardian knows the cluster", func() (bool, error) {
			out := f.MustExec(t, n, "curl -sS --max-time 10 "+base+"/v1/vault/guardians").Stdout
			var list guardianList
			if err := json.Unmarshal([]byte(out), &list); err != nil {
				return false, fmt.Errorf("guardian list %.200s: %w", out, err)
			}
			if list.Total < len(f.State.Nodes) {
				return false, fmt.Errorf("total %d guardians, the fleet has %d nodes", list.Total, len(f.State.Nodes))
			}
			for _, g := range list.Guardians {
				if g.Address == n.WGIP && g.Port == guardianPort {
					return true, nil
				}
			}
			return false, fmt.Errorf("the guardian does not list itself (%s) as alive: %.200s", n.WGIP, out)
		})
	}
}

// TestGuardian_documentedNotDone: what docs/whitepaper/technical-reference/vol1/28-vault.md says is not built
// yet is not: no guardian learns that another is alive (nothing listens on the
// peer port 7501), so guardian health stays "degraded".
func TestGuardian_documentedNotDone(t *testing.T) {
	t.Parallel()
	f := harness.Fleet(t)
	for _, n := range f.State.Nodes {
		base := fmt.Sprintf("http://%s:%d", n.WGIP, guardianPort)
		health := f.MustExec(t, n, "curl -sS --max-time 10 "+base+"/v1/vault/health").Stdout
		if !strings.Contains(health, `"degraded"`) {
			t.Errorf("%s: guardian health %.200s, documented as degraded until the peer protocol runs", n.Name, health)
		}
		for _, l := range f.Listeners(t, n) {
			if l.Port == peerPort {
				t.Errorf("%s: something listens on the peer port %d (%s)", n.Name, peerPort, l.Process)
			}
		}
	}
}

// TestGuardian_directPushNeedsSession: a guardian refuses a push without an
// X-Session-Token from the node itself (401, docs/whitepaper/technical-reference/vol1/28-vault.md, Guardian authentication).
func TestGuardian_directPushNeedsSession(t *testing.T) {
	t.Parallel()
	f := harness.Fleet(t)
	n := f.State.Nodes[0]
	cmd := fmt.Sprintf(`curl -sS --max-time 10 -o /dev/null -w '%%{http_code}' -X POST -H 'Content-Type: application/json' `+
		`-d '{"identity":"%s","share":"AQI=","version":1}' http://%s:%d/v1/vault/push`, strings.Repeat("a", 64), n.WGIP, guardianPort)
	if code := strings.TrimSpace(f.MustExec(t, n, cmd).Stdout); code != "401" {
		t.Errorf("guardian push without a session: %s, want 401", code)
	}
}
