package auth

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// memCounter is WindowCounter in memory, with Olric's semantics: a member
// keeps the place it was first given.
type memCounter struct {
	mu     sync.Mutex
	counts map[string]int
	places map[string]int
	err    error
}

func newMemCounter() *memCounter {
	return &memCounter{counts: map[string]int{}, places: map[string]int{}}
}

func (c *memCounter) Position(_ context.Context, key, member string, _ time.Duration) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err != nil {
		return 0, c.err
	}
	if place, ok := c.places[key+"|"+member]; ok {
		return place, nil
	}
	c.counts[key]++
	c.places[key+"|"+member] = c.counts[key]
	return c.counts[key], nil
}

func intPtr(n int) *int    { return &n }
func boolPtr(b bool) *bool { return &b }

func TestPerIPLimitsOf_offWithTheDefaultsByDefault(t *testing.T) {
	s, _, _ := realRegistry(t)
	got, err := s.PerIPLimitsOf(context.Background(), "anchat")
	want := PerIPLimits{NewWalletsPerHour: DefaultNewWalletsPerIPHour, InvokesPerMinute: DefaultInvokesPerIPMinute}
	if err != nil || got != want {
		t.Errorf("a namespace with no row has %+v, %v; want %+v", got, err, want)
	}
	if n, err := s.AuthenticatedInvokesPerIP(context.Background(), "anchat"); err != nil || n != 0 {
		t.Errorf("the invoke ceiling while off is %d, %v; want 0", n, err)
	}
}

// A field left out keeps its value, the ceilings survive turning the switch
// off and on, and setting them leaves the other session settings alone.
func TestSetPerIPLimits_mergesAndKeepsTheOtherSettings(t *testing.T) {
	s, _, _ := openNamespace(t)
	ctx := context.Background()

	got, err := s.SetPerIPLimits(ctx, "anchat", PerIPLimitsChange{Enabled: boolPtr(true), NewWalletsPerHour: intPtr(5)}, "0xowner")
	if err != nil || got != (PerIPLimits{Enabled: true, NewWalletsPerHour: 5, InvokesPerMinute: DefaultInvokesPerIPMinute}) {
		t.Fatalf("turn on with 5: %+v, %v", got, err)
	}
	if _, err := s.SetPerIPLimits(ctx, "anchat", PerIPLimitsChange{Enabled: boolPtr(false)}, "0xowner"); err != nil {
		t.Fatal(err)
	}
	got, err = s.SetPerIPLimits(ctx, "anchat", PerIPLimitsChange{Enabled: boolPtr(true), InvokesPerMinute: intPtr(300)}, "0xowner")
	if err != nil || got != (PerIPLimits{Enabled: true, NewWalletsPerHour: 5, InvokesPerMinute: 300}) {
		t.Fatalf("back on: %+v, %v", got, err)
	}
	if read, err := s.PerIPLimitsOf(ctx, "anchat"); err != nil || read != got {
		t.Errorf("read back %+v, %v; want %+v", read, err, got)
	}
	if n, err := s.AuthenticatedInvokesPerIP(ctx, "anchat"); err != nil || n != 300 {
		t.Errorf("the invoke ceiling is %d, %v; want 300", n, err)
	}
	if p, _ := s.SignInPolicyOf(ctx, "anchat"); p != SignInOpen {
		t.Errorf("setting the limits changed sign-in to %q", p)
	}
}

func TestSetPerIPLimits_refusesTheLobbyAndValuesOutOfRange(t *testing.T) {
	s, _, _ := realRegistry(t)
	ctx := context.Background()
	if _, err := s.SetPerIPLimits(ctx, LobbyNamespace, PerIPLimitsChange{Enabled: boolPtr(true)}, "0xowner"); !errors.Is(err, ErrLobbyHasNoSignInPolicy) {
		t.Errorf("the lobby: %v", err)
	}
	for name, change := range map[string]PerIPLimitsChange{
		"no new wallets":       {NewWalletsPerHour: intPtr(0)},
		"too many new wallets": {NewWalletsPerHour: intPtr(MaxNewWalletsPerIPHour + 1)},
		"no invokes":           {InvokesPerMinute: intPtr(0)},
		"too many invokes":     {InvokesPerMinute: intPtr(MaxInvokesPerIPMinute + 1)},
	} {
		if _, err := s.SetPerIPLimits(ctx, "anchat", change, "0xowner"); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
	for _, raw := range []string{"", "yes", "enabled"} {
		if _, err := ParsePerIPLimitsSwitch(raw); err == nil {
			t.Errorf("%q was accepted as a switch", raw)
		}
	}
}

// limitedNamespace is an open namespace whose per-IP limits are on, admitting
// limit new wallets per network per hour, counted by c.
func limitedNamespace(t *testing.T, limit int) (*Service, *sqliteDatabase, *memCounter) {
	t.Helper()
	s, db, _ := openNamespace(t)
	c := newMemCounter()
	s.SetNewWalletCounter(c)
	if _, err := s.SetPerIPLimits(context.Background(), "anchat",
		PerIPLimitsChange{Enabled: boolPtr(true), NewWalletsPerHour: intPtr(limit)}, "0xowner"); err != nil {
		t.Fatal(err)
	}
	return s, db, c
}

func TestAdmitSignIn_refusesANetworkPastItsNewWallets(t *testing.T) {
	s, _, _ := limitedNamespace(t, 2)
	ctx := context.Background()
	for _, w := range []string{"0xa", "0xb"} {
		if err := s.AdmitSignIn(ctx, w, "anchat", "203.0.113.7"); err != nil {
			t.Fatalf("%s within the ceiling: %v", w, err)
		}
	}
	var exceeded *ErrNewWalletsPerIPExceeded
	err := s.AdmitSignIn(ctx, "0xc", "anchat", "203.0.113.7")
	if !errors.As(err, &exceeded) || exceeded.Limit != 2 || exceeded.RetryAfter <= 0 || exceeded.RetryAfter > NewWalletWindow {
		t.Fatalf("the third new wallet: %v", err)
	}
	// A wallet keeps its place: the admitted ones still are, the refused one
	// still is not, however often they retry.
	if err := s.AdmitSignIn(ctx, "0xa", "anchat", "203.0.113.7"); err != nil {
		t.Errorf("an admitted wallet retrying was refused: %v", err)
	}
	if err := s.AdmitSignIn(ctx, "0xc", "anchat", "203.0.113.7"); !errors.As(err, &exceeded) {
		t.Errorf("a refused wallet retrying was let in: %v", err)
	}
	if err := s.AdmitSignIn(ctx, "0xc", "anchat", "198.51.100.9"); err != nil {
		t.Errorf("another network was refused: %v", err)
	}
}

// Only a fresh identity is counted: not a member, not a wallet that held a
// session before, and nothing at all while the limits are off.
func TestAdmitSignIn_countsOnlyNewEndUsers(t *testing.T) {
	s, db, c := limitedNamespace(t, 1)
	ctx := context.Background()
	if err := s.AdmitSignIn(ctx, "0xowner", "anchat", "203.0.113.7"); err != nil {
		t.Fatalf("the owner: %v", err)
	}
	if _, err := db.db.Exec(`INSERT INTO refresh_tokens(namespace_id, subject, token) VALUES (10, '0xreturning', 'h')`); err != nil {
		t.Fatal(err)
	}
	if err := s.AdmitSignIn(ctx, "0xreturning", "anchat", "203.0.113.7"); err != nil {
		t.Fatalf("a returning end user: %v", err)
	}
	if len(c.counts) != 0 {
		t.Fatalf("a member or a returning wallet was counted: %v", c.counts)
	}
	if err := s.AdmitSignIn(ctx, "0xnew", "anchat", "203.0.113.7"); err != nil {
		t.Fatalf("the first new wallet: %v", err)
	}

	if _, err := s.SetPerIPLimits(ctx, "anchat", PerIPLimitsChange{Enabled: boolPtr(false)}, "0xowner"); err != nil {
		t.Fatal(err)
	}
	if err := s.AdmitSignIn(ctx, "0xnewer", "anchat", "203.0.113.7"); err != nil {
		t.Errorf("a new wallet with the limits off: %v", err)
	}
}

// A ceiling the owner turned on is not skipped because the count cannot be
// had; the refusal is retryable, not a verdict on the wallet.
func TestAdmitSignIn_refusesWhatItCannotCount(t *testing.T) {
	s, _, c := limitedNamespace(t, 5)
	c.err = errors.New("olric down")
	if err := s.AdmitSignIn(context.Background(), "0xa", "anchat", "203.0.113.7"); !errors.Is(err, ErrNewWalletCountUnavailable) {
		t.Errorf("an unreachable counter: %v", err)
	}
	s.SetNewWalletCounter(nil)
	if err := s.AdmitSignIn(context.Background(), "0xa", "anchat", "203.0.113.7"); !errors.Is(err, ErrNewWalletCountUnavailable) {
		t.Errorf("no counter: %v", err)
	}
}

// The window is the clock hour: a wallet refused late in one is told to wait
// until the next starts, and the next has a count of its own.
func TestChargeNewWallet_windowsByTheHour(t *testing.T) {
	s, _, c := limitedNamespace(t, 1)
	ctx := context.Background()
	late := time.Date(2026, 10, 23, 14, 59, 30, 0, time.UTC)
	if err := s.chargeNewWallet(ctx, "anchat", "203.0.113.7", "0xa", 1, late); err != nil {
		t.Fatal(err)
	}
	var exceeded *ErrNewWalletsPerIPExceeded
	if err := s.chargeNewWallet(ctx, "anchat", "203.0.113.7", "0xb", 1, late); !errors.As(err, &exceeded) || exceeded.RetryAfter != 30*time.Second {
		t.Fatalf("late in the hour: %v", err)
	}
	if err := s.chargeNewWallet(ctx, "anchat", "203.0.113.7", "0xb", 1, late.Add(time.Minute)); err != nil {
		t.Errorf("the next hour: %v", err)
	}
	if len(c.counts) != 2 {
		t.Errorf("want one count per hour, got %v", c.counts)
	}
}
