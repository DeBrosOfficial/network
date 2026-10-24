//go:build e2e_fleet

package serverless

import (
	"net/http"
	"testing"
	"time"

	"github.com/Orama-Network/orama/e2e/harness/eventually"
	"github.com/Orama-Network/orama/e2e/harness/wallet"
)

const (
	signInOpen    = "open"
	signInMembers = "members"
	// signInPolicyBudget is how long a gateway may judge a sign-in by the
	// policy it read before a change: the revocation list's 10 s staleness
	// bound plus a round trip (docs/whitepaper/technical-reference/vol1/14-authorization.md#consistency-and-caching).
	signInPolicyBudget = 15 * time.Second
)

// TestInvoke_keylessAppClient: an app that ships no key (bugboard #946) does
// everything it needs before and at sign-in on the namespace gateway with no
// credential: a public function answers it, an end user's wallet signs in
// (challenge, verify) and refreshes, and the session it gets invokes the same
// function. A private function stays refused to the keyless caller
// (website/src/docs/developer/access-control.mdx#api-keys "Where a key belongs").
func TestInvoke_keylessAppClient(t *testing.T) {
	t.Parallel()
	fx := setup(t)
	deploy(t, fx, fnSpec{name: "e2e-keyless-public", public: true})
	deploy(t, fx, fnSpec{name: "e2e-keyless-private"})

	if r := invoke(t, fx.c, "e2e-keyless-public", "", map[string]any{"op": "echo", "value": "before"}); r.Status != http.StatusOK {
		t.Errorf("a public function invoked with no credential: %d %.200s", r.Status, r.Body)
	}
	if r := invoke(t, fx.c, "e2e-keyless-private", "", map[string]any{"op": "echo"}); r.Status != http.StatusUnauthorized && r.Status != http.StatusForbidden {
		t.Errorf("a private function invoked with no credential: %d", r.Status)
	}

	fx.n.CLI.MustOK(t, "namespace", "session-policy", "--sign-in", signInOpen)
	t.Cleanup(func() { fx.n.CLI.MustOK(t, "namespace", "session-policy", "--sign-in", signInMembers) })

	endUser, err := wallet.NewEVM()
	if err != nil {
		t.Fatal(err)
	}
	client := fx.c.For(t)
	var accessToken, refreshToken string
	eventually.Require(t, pollEvery, signInPolicyBudget, "the namespace gateway to sign in a keyless end user", func() (bool, error) {
		s, err := client.SignIn(t.Context(), endUser, fx.n.Name, nil)
		if err != nil {
			return false, err
		}
		accessToken, refreshToken = s.AccessToken, s.RefreshToken
		return true, nil
	})
	refreshed, _, err := client.Refresh(t.Context(), refreshToken, fx.n.Name, nil)
	if err != nil {
		t.Fatalf("refreshing a keyless end user's session on the namespace gateway: %v", err)
	}
	if refreshed.AccessToken == "" || refreshed.AccessToken == accessToken {
		t.Errorf("the refresh returned no new access token")
	}

	if r := invoke(t, fx.c, "e2e-keyless-public", refreshed.AccessToken, map[string]any{"op": "echo", "value": "after"}); r.Status != http.StatusOK {
		t.Errorf("a public function invoked with the end user's session: %d %.200s", r.Status, r.Body)
	}
}
