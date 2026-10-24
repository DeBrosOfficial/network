//go:build e2e_fleet

package webrtc

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Orama-Network/orama/e2e/features/internal/services"
	"github.com/Orama-Network/orama/e2e/harness/ns"
	"github.com/Orama-Network/orama/e2e/harness/wallet"
)

// A participant is named by the account the namespace gives it
// (website/src/docs/developer/webrtc.mdx#identity).
const (
	claimsFixtureDir = "testdata/claimsfn"
	// claimsProvider is the reserved name the gateway runs at sign-in.
	claimsProvider = "auth-claims-provider"
	// accountPrefix and accountHexDigits are how the fixture names an account.
	accountPrefix    = "acct-"
	accountHexDigits = 16
)

// accountUser is a member signed in at the namespace's gateway, whose claims
// provider named the account the session belongs to.
type accountUser struct {
	rtcUser
	account string
}

// fixtureAccount is the account the claims fixture names wallet's.
func fixtureAccount(wallet string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(wallet)))
	return accountPrefix + hex.EncodeToString(sum[:])[:accountHexDigits]
}

// newAccountUser adds a runtime member and signs it in at the namespace's
// gateway, which runs the namespace's claims provider (the index gateway runs
// no tenant's), and checks the token names the fixture's account.
func newAccountUser(t *testing.T, fx *fixture, n *ns.Namespace) accountUser {
	t.Helper()
	w, err := wallet.NewEVM()
	if err != nil {
		t.Fatal(err)
	}
	n.CLI.MustOK(t, "members", "add", w.Address(), "--role", "runtime")
	s, err := fx.c.SignIn(t.Context(), w, n.Name, nil)
	if err != nil {
		t.Fatal(err)
	}
	u := accountUser{rtcUser: rtcUser{subject: s.Subject, token: s.AccessToken}, account: fixtureAccount(w.Address())}
	if got := customClaim(t, s.AccessToken, "account_id"); got != u.account {
		t.Fatalf("the token of %s carries account_id %q, want %q from the claims provider", s.Subject, got, u.account)
	}
	return u
}

// customClaim reads a custom claim from token's payload, which the test only
// inspects: the gateway verified the token when it minted it.
func customClaim(t *testing.T, token, name string) string {
	t.Helper()
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("access token is not a JWT")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("JWT payload: %v", err)
	}
	var claims struct {
		Custom map[string]string `json:"custom"`
	}
	if err := json.Unmarshal(raw, &claims); err != nil {
		t.Fatalf("JWT payload: %v", err)
	}
	return claims.Custom[name]
}

// TestAdmission_participantIsTheNamespacesAccount: when the namespace's claims
// provider names an account, the room, the membership events, admissions and
// the kick all know the participant by it, and no peer is shown a wallet.
func TestAdmission_participantIsTheNamespacesAccount(t *testing.T) {
	t.Parallel()
	fx := setup(t)
	waitPlaced(t, fx)
	deployFixture(t, fx.n, claimsFixtureDir, claimsProvider)
	deployRTCFunction(t, fx.n)
	admin := newRTCUser(t, fx.n, "admin")
	alice, bob := newAccountUser(t, fx, fx.n), newAccountUser(t, fx, fx.n)
	room := roomPrefix + "account-" + fx.n.Name
	wallets := []string{alice.subject, bob.subject}

	setRequireAdmission(t, fx, admin.token, true)
	admit(t, fx, alice.token, room, alice.account, admitTTLSeconds)
	admit(t, fx, alice.token, room, bob.account, admitTTLSeconds)
	watch := watchMembership(t, fx, admin.token, room)

	a := rtcJoin(t, fx, alice.rtcUser, room, false)
	watch("webrtc.join", alice.account)
	b := rtcJoin(t, fx, bob.rtcUser, room, false)
	watch("webrtc.join", bob.account)

	seen := map[string]bool{}
	for _, p := range b.Participants {
		seen[p.UserID] = true
		for _, w := range wallets {
			if strings.EqualFold(p.UserID, w) {
				t.Fatalf("bob's welcome names a participant by the wallet %s: %+v", w, b.Participants)
			}
		}
	}
	if len(seen) != 2 || !seen[alice.account] || !seen[bob.account] {
		t.Fatalf("bob's welcome lists %+v, want the accounts %s and %s", b.Participants, alice.account, bob.account)
	}
	waitFrame(t, a, "bob joining, named by his account", func(m services.SignalMsg) bool {
		var j struct {
			Participant services.Participant `json:"participant"`
		}
		return m.Type == services.MsgParticipantJoined && json.Unmarshal(m.Data, &j) == nil && j.Participant.UserID == bob.account
	})

	callRTC(t, fx, alice.token, map[string]any{"op": "kick", "room": room, "user": bob.account})
	waitFrame(t, b, "the kick by account to close bob's socket", func(m services.SignalMsg) bool { return m.Type == services.MsgKicked })
	if ev := watch("webrtc.leave", bob.account); ev.Reason != "kicked" {
		t.Fatalf("leave event = %+v, want bob's account leaving, kicked", ev)
	}
	expectRefused(t, fx, bob.token, room, "WEBRTC_ADMISSION_REVOKED")
}
