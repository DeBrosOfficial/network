package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	authsvc "github.com/Orama-Network/orama/core/pkg/gateway/auth"
)

// placeCounter is authsvc.WindowCounter in memory: a member keeps the place
// it was first given.
type placeCounter struct {
	mu     sync.Mutex
	counts map[string]int
	places map[string]int
}

func (c *placeCounter) Position(_ context.Context, key, member string, _ time.Duration) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if place, ok := c.places[key+"|"+member]; ok {
		return place, nil
	}
	c.counts[key]++
	c.places[key+"|"+member] = c.counts[key]
	return c.counts[key], nil
}

// The session policy reads and sets the per-IP limits: off with the defaults
// until turned on, a field left out kept, a value out of range refused.
func TestSessionPolicyHandler_perIPLimits(t *testing.T) {
	f := newFlow(t)
	f.owner("0xowner")
	operator := &authsvc.JWTClaims{Sub: "0xowner", Namespace: flowNamespace}

	code, body := f.do(f.h.SessionPolicyHandler, http.MethodGet, sessionPolicyPath, nil, operator)
	if code != http.StatusOK || body["per_ip_limits"] != "off" ||
		body["new_wallets_per_ip_hour"] != float64(authsvc.DefaultNewWalletsPerIPHour) ||
		body["invokes_per_ip_minute"] != float64(authsvc.DefaultInvokesPerIPMinute) {
		t.Fatalf("a namespace with no row: %d %v", code, body)
	}
	if code, body := f.setPolicy(map[string]any{"per_ip_limits": "on", "new_wallets_per_ip_hour": 3}); code != http.StatusOK ||
		body["per_ip_limits"] != "on" || body["new_wallets_per_ip_hour"] != float64(3) ||
		body["invokes_per_ip_minute"] != float64(authsvc.DefaultInvokesPerIPMinute) {
		t.Fatalf("turn on with 3: %d %v", code, body)
	}
	if code, body := f.setPolicy(map[string]any{"invokes_per_ip_minute": 90}); code != http.StatusOK ||
		body["per_ip_limits"] != "on" || body["new_wallets_per_ip_hour"] != float64(3) || body["invokes_per_ip_minute"] != float64(90) {
		t.Fatalf("a field left out was not kept: %d %v", code, body)
	}
	for name, bad := range map[string]map[string]any{
		"switch":      {"per_ip_limits": "maybe"},
		"zero":        {"new_wallets_per_ip_hour": 0},
		"too many":    {"invokes_per_ip_minute": authsvc.MaxInvokesPerIPMinute + 1},
		"not integer": {"invokes_per_ip_minute": "lots"},
	} {
		if code, body := f.setPolicy(bad); code != http.StatusBadRequest {
			t.Errorf("%s: %d %v", name, code, body)
		}
	}
}

// Past its ceiling a network's next new wallet is refused 429
// SIGN_IN_RATE_LIMITED, and a wallet it already signed in is not.
func TestVerifyHandler_refusesANetworkPastItsNewWallets(t *testing.T) {
	f := newFlow(t)
	f.owner("0xowner")
	f.svc.SetNewWalletCounter(&placeCounter{counts: map[string]int{}, places: map[string]int{}})
	if code, body := f.setPolicy(map[string]any{"sign_in": "open", "per_ip_limits": "on", "new_wallets_per_ip_hour": 1}); code != http.StatusOK {
		t.Fatalf("open with a ceiling of 1: %d %v", code, body)
	}
	first := newWallet(t)
	if code, body := f.signIn(first, nil); code != http.StatusOK {
		t.Fatalf("the first new wallet: %d %v", code, body)
	}
	if code, body := f.signIn(newWallet(t), nil); code != http.StatusTooManyRequests || body["code"] != ErrCodeSignInRateLimited {
		t.Fatalf("the second new wallet: %d %v", code, body)
	}
	if code, body := f.signIn(first, nil); code != http.StatusOK {
		t.Errorf("the first wallet signing in again: %d %v", code, body)
	}
}

// With no counter the ceiling cannot be applied, and the sign-in is refused
// as unavailable, not let past it.
func TestVerifyHandler_refusesWhatItCannotCount(t *testing.T) {
	f := newFlow(t)
	f.owner("0xowner")
	if code, body := f.setPolicy(map[string]any{"sign_in": "open", "per_ip_limits": "on"}); code != http.StatusOK {
		t.Fatalf("open: %d %v", code, body)
	}
	if code, body := f.signIn(newWallet(t), nil); code != http.StatusServiceUnavailable {
		t.Errorf("no counter: %d %v", code, body)
	}
}

func TestWriteCredentialError_newWalletCeilingCarriesRetryAfter(t *testing.T) {
	rec := httptest.NewRecorder()
	writeCredentialError(rec, flowNamespace, &authsvc.ErrNewWalletsPerIPExceeded{Limit: 20, RetryAfter: 90500 * time.Millisecond})
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") != "91" {
		t.Errorf("got %d, Retry-After %q", rec.Code, rec.Header().Get("Retry-After"))
	}
	if !isSignInRefusal(&authsvc.ErrNewWalletsPerIPExceeded{}) || !isSignInRefusal(authsvc.ErrNewWalletCountUnavailable) {
		t.Error("the device flow would not answer the ceiling as a sign-in refusal")
	}
}

func TestSignInClient_foldsIPv6AndIgnoresForgedForwarding(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/v1/auth/verify", nil)
	r.RemoteAddr = "[2001:db8:1:2::7]:4433"
	r.Header.Set("X-Forwarded-For", "198.51.100.1")
	if got := signInClient(r); got != "2001:db8:1:2::/64" {
		t.Errorf("got %q", got)
	}
}
