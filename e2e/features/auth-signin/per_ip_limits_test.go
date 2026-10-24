//go:build e2e_fleet

package authsignin

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/Orama-Network/orama/e2e/harness"
	"github.com/Orama-Network/orama/e2e/harness/eventually"
	"github.com/Orama-Network/orama/e2e/harness/gw"
	"github.com/Orama-Network/orama/e2e/harness/ns"
	"github.com/Orama-Network/orama/e2e/harness/oramacli"
)

// codeSignInRateLimited is the refusal of a new wallet past its network's
// ceiling (website/src/docs/developer/sign-in.mdx#per-ip-limits).
const codeSignInRateLimited = "SIGN_IN_RATE_LIMITED"

// setSessionPolicy sends fields to n's session policy as its owner.
func setSessionPolicy(t testing.TB, n *ns.Namespace, fields map[string]any) map[string]any {
	t.Helper()
	body, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	resp := n.Owner.Client.MustSend(t, gw.Req{Method: http.MethodPut, Path: pathSessionPolicy, Bearer: n.Owner.Token(),
		Header: http.Header{"Content-Type": {"application/json"}}, Body: body})
	var out map[string]any
	if err := resp.Expect(t, http.StatusOK).Decode(&out); err != nil {
		t.Fatal(err)
	}
	return out
}

// TestPerIPLimits_capNewWalletsPerNetwork: with sign-in open and the per-IP
// limits on at one new wallet an hour, the runner's network signs in one new
// wallet; the next is refused 429 SIGN_IN_RATE_LIMITED with a Retry-After of
// at most the hour, and the wallet it already signed in may sign in again
// (website/src/docs/developer/sign-in.mdx#per-ip-limits).
func TestPerIPLimits_capNewWalletsPerNetwork(t *testing.T) {
	t.Parallel()
	f := harness.Fleet(t)
	n := ns.New(t, f, ns.Options{})
	t.Cleanup(func() { setSessionPolicy(t, n, map[string]any{"sign_in": signInMembers, "per_ip_limits": "off"}) })
	c := harness.GW(t)

	got := setSessionPolicy(t, n, map[string]any{"sign_in": signInOpen, "per_ip_limits": "on", "new_wallets_per_ip_hour": 1})
	if got["per_ip_limits"] != "on" || got["new_wallets_per_ip_hour"] != float64(1) {
		t.Fatalf("turning the limits on answered %v", got)
	}

	first := newWallet(t)
	eventually.Require(t, pollEvery, signInPolicyBudget, "the open namespace to admit its first new wallet", func() (bool, error) {
		_, err := c.For(t).SignIn(t.Context(), first, n.Name, nil)
		return err == nil, err
	})

	second := newWallet(t)
	ch := challengeFor(t, c, gw.ChallengeRequest{Wallet: second.Address(), Namespace: n.Name})
	resp := postJSON(t, c, gw.PathVerify, "", signed(t, second, ch.Message))
	expectRefusal(t, resp, http.StatusTooManyRequests, codeSignInRateLimited)
	if secs, err := strconv.Atoi(resp.Header.Get("Retry-After")); err != nil || secs < 1 || secs > 3600 {
		t.Errorf("Retry-After %q: want 1 to 3600 seconds", resp.Header.Get("Retry-After"))
	}

	if _, err := c.For(t).SignIn(t.Context(), first, n.Name, nil); err != nil {
		t.Errorf("the wallet the network already signed in was refused: %v", err)
	}
}

// TestPerIPLimits_cliShowsAndSets: `orama namespace session-policy` shows the
// limits off with their defaults, --per-ip-limits on turns them on, a ceiling
// flag keeps the switch, and a ceiling out of range is refused
// (docs/whitepaper/technical-reference/vol1/13-identity.md#per-ip-limits).
func TestPerIPLimits_cliShowsAndSets(t *testing.T) {
	t.Parallel()
	f := harness.Fleet(t)
	n := ns.New(t, f, ns.Options{Via: ns.ViaOperator})
	read := func(args ...string) map[string]any {
		var out map[string]any
		res := n.CLI.MustOK(t, append([]string{"namespace", "session-policy", "--json"}, args...)...)
		if err := oramacli.DecodeJSON(res, &out); err != nil {
			t.Fatalf("session-policy %v printed no JSON: %v", args, err)
		}
		return out
	}
	if got := read(); got["per_ip_limits"] != "off" || got["new_wallets_per_ip_hour"] != float64(20) || got["invokes_per_ip_minute"] != float64(1200) {
		t.Fatalf("a new namespace's limits are %v, want off, 20 and 1200", got)
	}
	if got := read("--per-ip-limits", "on", "--new-wallets-per-ip-hour", "5"); got["per_ip_limits"] != "on" || got["new_wallets_per_ip_hour"] != float64(5) {
		t.Errorf("--per-ip-limits on --new-wallets-per-ip-hour 5 answered %v", got)
	}
	if got := read("--invokes-per-ip-minute", "300"); got["per_ip_limits"] != "on" || got["invokes_per_ip_minute"] != float64(300) || got["new_wallets_per_ip_hour"] != float64(5) {
		t.Errorf("--invokes-per-ip-minute 300 did not keep the rest: %v", got)
	}
	if res, err := n.CLI.For(t).Run(t.Context(), "namespace", "session-policy", "--new-wallets-per-ip-hour", "0"); err != nil || res.Exit == 0 {
		t.Errorf("a ceiling of 0 was accepted: exit %d, %v", res.Exit, err)
	}
	if got := read("--per-ip-limits", "off"); got["per_ip_limits"] != "off" || got["invokes_per_ip_minute"] != float64(300) {
		t.Errorf("--per-ip-limits off did not keep the ceilings: %v", got)
	}
}
