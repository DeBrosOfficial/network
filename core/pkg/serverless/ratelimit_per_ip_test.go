package serverless

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/zap"
)

// fixedLimits answers every namespace's invoke ceiling from a map, and counts
// how often it was asked.
type fixedLimits struct {
	perMinute map[string]int
	err       error
	asked     int
}

func (f *fixedLimits) AuthenticatedInvokesPerIP(_ context.Context, namespace string) (int, error) {
	f.asked++
	return f.perMinute[namespace], f.err
}

func limiterWith(src NamespaceLimits) *MultiTierLimiter {
	l := NewMultiTierLimiter(DefaultLimiterConfig())
	l.SetNamespaceLimits(src)
	return l
}

// The case the tier exists for: one network holding many wallets is held to
// the namespace's ceiling whichever wallet calls, and another network is not.
func TestMultiTier_perIPAuthenticatedSpansWalletsAndKeys(t *testing.T) {
	// 60/min: burst 6, one token a second.
	l := limiterWith(&fixedLimits{perMinute: map[string]int{"anchat": 60}})
	ctx := context.Background()
	for i := 0; i < 6; i++ {
		req := RateLimitRequest{Namespace: "anchat", Wallet: "w" + string(rune('a'+i)), IP: "203.0.113.7"}
		if i%2 == 1 {
			req = RateLimitRequest{Namespace: "anchat", Key: "key" + string(rune('a'+i)), IP: "203.0.113.7"}
		}
		if d, err := l.AllowRequest(ctx, req); err != nil || !d.Allowed {
			t.Fatalf("call %d within the burst: %+v, %v", i+1, d, err)
		}
	}
	d, err := l.AllowRequest(ctx, RateLimitRequest{Namespace: "anchat", Wallet: "wz", IP: "203.0.113.7"})
	if err != nil || d.Allowed || d.Scope != "per_ip_authenticated" || d.RetryAfter <= 0 {
		t.Fatalf("a fresh wallet on the same network: %+v, %v", d, err)
	}
	if d, _ := l.AllowRequest(ctx, RateLimitRequest{Namespace: "anchat", Wallet: "wz", IP: "198.51.100.9"}); !d.Allowed {
		t.Errorf("another network was refused: %+v", d)
	}
}

// An IPv6 client is one network across its /64, so rotating addresses inside
// it does not escape the ceiling.
func TestMultiTier_perIPAuthenticatedFoldsIPv6ToItsPrefix(t *testing.T) {
	l := limiterWith(&fixedLimits{perMinute: map[string]int{"anchat": 10}}) // burst 1
	ctx := context.Background()
	if d, _ := l.AllowRequest(ctx, RateLimitRequest{Namespace: "anchat", Wallet: "w1", IP: "2001:db8:1:2::1"}); !d.Allowed {
		t.Fatalf("first: %+v", d)
	}
	if d, _ := l.AllowRequest(ctx, RateLimitRequest{Namespace: "anchat", Wallet: "w2", IP: "2001:db8:1:2:ffff::9"}); d.Allowed || d.Scope != "per_ip_authenticated" {
		t.Errorf("another address in the same /64: %+v", d)
	}
	if d, _ := l.AllowRequest(ctx, RateLimitRequest{Namespace: "anchat", Wallet: "w3", IP: "2001:db8:1:3::1"}); !d.Allowed {
		t.Errorf("another /64: %+v", d)
	}
}

// The tier is off unless the namespace set a ceiling, never asks about an
// anonymous caller (already bucketed by address) or a call with no address
// (a trigger, a cron job), and each namespace's ceiling is its own.
func TestMultiTier_perIPAuthenticatedOnlyWhereSet(t *testing.T) {
	src := &fixedLimits{perMinute: map[string]int{"anchat": 10}}
	l := limiterWith(src)
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		if d, _ := l.AllowRequest(ctx, RateLimitRequest{Namespace: "other", Wallet: "w" + string(rune('a'+i)), IP: "203.0.113.7"}); !d.Allowed {
			t.Fatalf("a namespace with no ceiling refused call %d: %+v", i+1, d)
		}
	}
	asked := src.asked
	_, _ = l.AllowRequest(ctx, RateLimitRequest{Namespace: "anchat", IP: "203.0.113.7"})
	_, _ = l.AllowRequest(ctx, RateLimitRequest{Namespace: "anchat", Wallet: "w1"})
	if src.asked != asked {
		t.Errorf("the ceiling was asked for an anonymous or address-less call")
	}
	if d, _ := NewMultiTierLimiter(DefaultLimiterConfig()).AllowRequest(ctx, RateLimitRequest{Namespace: "anchat", Wallet: "w1", IP: "203.0.113.7"}); !d.Allowed {
		t.Errorf("a limiter with no namespace settings refused: %+v", d)
	}
}

// One identity meets its own limit before its network's: the per-wallet tier
// comes first.
func TestMultiTier_perWalletBeforePerIPAuthenticated(t *testing.T) {
	cfg := DefaultLimiterConfig()
	cfg.PerWalletBurst = 1
	l := NewMultiTierLimiter(cfg)
	l.SetNamespaceLimits(&fixedLimits{perMinute: map[string]int{"anchat": 600}})
	ctx := context.Background()
	_, _ = l.AllowRequest(ctx, RateLimitRequest{Namespace: "anchat", Wallet: "w1", IP: "203.0.113.7"})
	if d, _ := l.AllowRequest(ctx, RateLimitRequest{Namespace: "anchat", Wallet: "w1", IP: "203.0.113.7"}); d.Scope != "per_wallet" {
		t.Errorf("one wallet past its own burst was refused as %q", d.Scope)
	}
}

// A ceiling that cannot be read is not skipped: the call is refused with a
// retryable error, and the engine does not run it.
func TestMultiTier_unreadableCeilingRefuses(t *testing.T) {
	l := limiterWith(&fixedLimits{err: errors.New("registry down")})
	if _, err := l.AllowRequest(context.Background(), RateLimitRequest{Namespace: "anchat", Wallet: "w1", IP: "203.0.113.7"}); !errors.Is(err, ErrInvokeLimitsUnavailable) {
		t.Fatalf("limiter: %v", err)
	}

	engine, err := NewEngine(nil, NewMockRegistry(), NewMockHostServices(), zap.NewNop(), WithRateLimiter(l))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = engine.runtime.Close(context.Background()) })
	_, err = engine.Execute(context.Background(), &Function{ID: "f1", Name: "rpc", Namespace: "anchat"},
		nil, &InvocationContext{Namespace: "anchat", FunctionName: "rpc", CallerWallet: "w1", CallerIP: "203.0.113.7"})
	if !errors.Is(err, ErrInvokeLimitsUnavailable) {
		t.Errorf("engine: %v", err)
	}
}
