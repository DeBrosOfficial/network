//go:build e2e_fleet

package dnstls

import (
	"net"
	"slices"
	"testing"

	"golang.org/x/net/dns/dnsmessage"

	"github.com/Orama-Network/orama/e2e/features/internal/edge"
	"github.com/Orama-Network/orama/e2e/harness"
	"github.com/Orama-Network/orama/e2e/harness/oramacli"
)

// delegationFinding is one finding of `orama node dns delegation --json`.
type delegationFinding struct {
	Kind string `json:"kind"`
	Want string `json:"want"`
}

// TestDelegation_verdictIsWhatTheParentZoneServes (bug-930): the command's
// NS verdict is what the parent zone's own nameservers hand out right now,
// asked non-recursively, not what a caching resolver remembers: every slot
// the parent's referral names has no missing-ns finding, and every slot it
// does not name has one (website/src/docs/operator/nameserver.mdx "Seeing
// which address holds which slot").
func TestDelegation_verdictIsWhatTheParentZoneServes(t *testing.T) {
	t.Parallel()
	f := harness.Fleet(t)
	parent := parentOf(f.State.BaseDomain)
	nss, err := net.DefaultResolver.LookupNS(t.Context(), parent)
	if err != nil || len(nss) == 0 {
		t.Fatalf("the nameservers of the parent zone %s: %v %v", parent, nss, err)
	}
	addrs, err := net.DefaultResolver.LookupIP(t.Context(), "ip4", nss[0].Host)
	if err != nil || len(addrs) == 0 {
		t.Fatalf("the address of %s: %v %v", nss[0].Host, addrs, err)
	}
	referral := ask(t, addrs[0].String(), f.State.BaseDomain, dnsmessage.TypeNS)
	var served []string
	for _, rr := range append(referral.Answers, referral.Authority...) {
		if rr.Type == dnsmessage.TypeNS && rr.Name == edge.Fqdn(f.State.BaseDomain) {
			served = append(served, rr.Value)
		}
	}

	res := harness.CLI(t).MustOK(t, "node", "dns", "delegation", "--env", f.State.Env, "--json")
	var reports []struct {
		Domain      string `json:"domain"`
		CheckError  string `json:"check_error"`
		Nameservers []struct {
			Hostname string `json:"hostname"`
		} `json:"nameservers"`
		Findings []delegationFinding `json:"findings"`
	}
	if err := oramacli.DecodeJSON(res, &reports); err != nil {
		t.Fatal(err)
	}
	if len(reports) != 1 || reports[0].CheckError != "" {
		t.Fatalf("reports %+v, want one verdict for %s", reports, f.State.BaseDomain)
	}
	r := reports[0]
	for _, n := range r.Nameservers {
		fqdn := edge.Fqdn(n.Hostname + "." + f.State.BaseDomain)
		missing := slices.ContainsFunc(r.Findings, func(fd delegationFinding) bool {
			return fd.Kind == "missing-ns" && edge.Fqdn(fd.Want) == fqdn
		})
		if slices.Contains(served, fqdn) == missing {
			t.Errorf("%s: the parent zone serves NS %v, and the command reports missing-ns=%v", fqdn, served, missing)
		}
	}
}
