//go:build e2e_fleet

package gatewaymiddleware

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Orama-Network/orama/e2e/features/internal/edge"
	"github.com/Orama-Network/orama/e2e/harness"
		"github.com/Orama-Network/orama/e2e/harness/eventually"
	"github.com/Orama-Network/orama/e2e/harness/gw"
	"github.com/Orama-Network/orama/e2e/harness/ns"
)

const (
	// nsCORSReach bounds how long a namespace's new list takes to reach the
	// node a request lands on: at once where it was recorded, within the 30 s
	// reload everywhere else (core/pkg/gateway/corsorigins/cache.go).
	nsCORSReach     = 45 * time.Second
	nsCORSPollEvery = 3 * time.Second
)

// TestCORS_aNamespaceAdmitsItsOwnOrigins: an owner lists an origin with
// `orama namespace cors-origins`; a preflight from it to the namespace's host is
// echoed, the same origin is refused on the cluster's own name, and clearing
// the list refuses it again
// (website/src/docs/developer/namespaces.mdx "Browser origins").
func TestCORS_aNamespaceAdmitsItsOwnOrigins(t *testing.T) {
	t.Parallel()
	f := harness.Fleet(t)
	n := ns.New(t, f, ns.Options{Via: ns.ViaOperator})
	origin := "https://" + edge.RandomLabel(t, "app-") + ".example"
	nsHost := "ns-" + n.Name + "." + f.State.BaseDomain
	c := harness.GW(t)

	allowOrigin := func(host string) string {
		resp := c.MustSend(t, gw.Req{Method: http.MethodOptions, Path: "/v1/functions", Host: host, Header: http.Header{
			"Origin": {origin}, "Access-Control-Request-Method": {"POST"}}})
		return resp.Header.Get("Access-Control-Allow-Origin")
	}
	waitFor := func(want bool, what string) {
		t.Helper()
		err := eventually.Poll(t.Context(), nsCORSPollEvery, nsCORSReach, what, func() (bool, error) {
			return (allowOrigin(nsHost) == origin) == want, nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	out := n.CLI.MustOK(t, "namespace", "cors-origins", "--namespace", n.Name, "--set", origin).Stdout
	if !strings.Contains(out, origin) {
		t.Fatalf("the list after --set does not show %s:\n%s", origin, out)
	}
	waitFor(true, "the namespace's host to admit its listed origin")
	if got := allowOrigin(f.State.BaseDomain); got == origin {
		t.Error("the cluster's own name admits an origin only one namespace listed")
	}

	n.CLI.MustOK(t, "namespace", "cors-origins", "--namespace", n.Name, "--clear")
	waitFor(false, "the namespace's host to refuse the origin once the list is cleared")

	res, err := n.CLI.For(t).Run(t.Context(), "namespace", "cors-origins", "--namespace", n.Name, "--set", "https://*.example")
	if err != nil {
		t.Fatal(err)
	}
	if res.Exit == 0 {
		t.Error("a wildcard origin was accepted")
	}
}
