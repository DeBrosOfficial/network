package serverless

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestMultiTier_within_limit_allows(t *testing.T) {
	l := NewMultiTierLimiter(DefaultLimiterConfig())
	for i := 0; i < 10; i++ {
		d, _ := l.AllowRequest(context.Background(), RateLimitRequest{
			Namespace: "ns", Function: "fn", Wallet: "w1",
		})
		if !d.Allowed {
			t.Fatalf("request %d unexpectedly denied", i)
		}
	}
}

func TestMultiTier_per_wallet_burst_exhausted(t *testing.T) {
	cfg := DefaultLimiterConfig()
	cfg.PerWalletBurst = 5
	cfg.PerWalletPerMinute = 600 // refill 10/sec, slow enough that burst matters
	l := NewMultiTierLimiter(cfg)

	// Burn the burst.
	for i := 0; i < 5; i++ {
		d, _ := l.AllowRequest(context.Background(), RateLimitRequest{
			Namespace: "ns", Wallet: "w1",
		})
		if !d.Allowed {
			t.Fatalf("burst[%d] should be allowed", i)
		}
	}
	// Next one rejected.
	d, _ := l.AllowRequest(context.Background(), RateLimitRequest{
		Namespace: "ns", Wallet: "w1",
	})
	if d.Allowed {
		t.Fatal("expected rejection after burst")
	}
	if d.Scope != "per_wallet" {
		t.Errorf("expected scope=per_wallet, got %q", d.Scope)
	}
	if d.RetryAfter <= 0 {
		t.Errorf("expected positive RetryAfter, got %v", d.RetryAfter)
	}
}

func TestMultiTier_per_wallet_isolation(t *testing.T) {
	cfg := DefaultLimiterConfig()
	cfg.PerWalletBurst = 3
	cfg.PerWalletPerMinute = 60 // 1/sec — slow refill
	l := NewMultiTierLimiter(cfg)

	// Wallet A burns its burst.
	for i := 0; i < 3; i++ {
		d, _ := l.AllowRequest(context.Background(), RateLimitRequest{
			Namespace: "ns", Wallet: "A",
		})
		if !d.Allowed {
			t.Fatalf("A[%d] should be allowed", i)
		}
	}
	dA, _ := l.AllowRequest(context.Background(), RateLimitRequest{
		Namespace: "ns", Wallet: "A",
	})
	if dA.Allowed {
		t.Fatal("expected A to be rate-limited")
	}

	// Wallet B unaffected.
	dB, _ := l.AllowRequest(context.Background(), RateLimitRequest{
		Namespace: "ns", Wallet: "B",
	})
	if !dB.Allowed {
		t.Fatal("expected B to be allowed (isolated from A)")
	}
}

func TestMultiTier_per_namespace_ceiling(t *testing.T) {
	cfg := DefaultLimiterConfig()
	cfg.PerNamespaceBurst = 3
	cfg.PerNamespacePerMinute = 60
	cfg.PerWalletBurst = 1000 // way above ns burst
	cfg.PerWalletPerMinute = 60_000
	l := NewMultiTierLimiter(cfg)

	// Different wallets, but they share the namespace ceiling.
	for i := 0; i < 3; i++ {
		d, _ := l.AllowRequest(context.Background(), RateLimitRequest{
			Namespace: "ns",
			Wallet:    "w" + string(rune('1'+i)),
		})
		if !d.Allowed {
			t.Fatalf("ns burst[%d] should be allowed", i)
		}
	}
	d, _ := l.AllowRequest(context.Background(), RateLimitRequest{
		Namespace: "ns", Wallet: "w99",
	})
	if d.Allowed {
		t.Fatal("expected per-namespace ceiling rejection")
	}
	if d.Scope != "per_namespace" {
		t.Errorf("expected scope=per_namespace, got %q", d.Scope)
	}
}

// bug-740: a function that declares per_caller is refused for a caller past
// it, while the gateway-wide per-wallet bucket (burst 60) would still allow.
func TestMultiTier_per_function_caller_tighter(t *testing.T) {
	l := NewMultiTierLimiter(DefaultLimiterConfig())
	override := &FunctionRateLimit{PerCaller: RateLimitRule{Requests: 5, WindowSeconds: 60}}
	req := RateLimitRequest{Namespace: "ns", Function: "expensive", Wallet: "w1", IP: "1.1.1.1", Override: override}
	for i := 0; i < 5; i++ {
		if d, _ := l.AllowRequest(context.Background(), req); !d.Allowed {
			t.Fatalf("call %d of 5 refused", i+1)
		}
	}
	d, _ := l.AllowRequest(context.Background(), req)
	if d.Allowed || d.Scope != "per_function_caller" {
		t.Fatalf("the 6th call got %+v, want a per_function_caller refusal", d)
	}
	// A 5-a-minute bucket refills one every 12 s.
	if d.RetryAfter <= 0 || d.RetryAfter > 12*time.Second {
		t.Errorf("retry after %v, want (0, 12s]", d.RetryAfter)
	}
	other := req
	other.Wallet = "w2"
	if d, _ := l.AllowRequest(context.Background(), other); !d.Allowed {
		t.Error("another wallet was refused by the first one's bucket")
	}
	unlimited := req
	unlimited.Function, unlimited.Override = "cheap", nil
	if d, _ := l.AllowRequest(context.Background(), unlimited); !d.Allowed {
		t.Error("a function with no limit of its own was refused by another function's")
	}
}

// feat-938: per_ip is charged to the address whoever signed in, so a fresh
// wallet per signup does not escape it; two an hour is two at once, then one
// every thirty minutes.
func TestMultiTier_per_function_ip_spans_wallets(t *testing.T) {
	l := NewMultiTierLimiter(DefaultLimiterConfig())
	override := &FunctionRateLimit{PerIP: RateLimitRule{Requests: 2, WindowSeconds: 3600}}
	signup := func(wallet, ip string) Decision {
		d, _ := l.AllowRequest(context.Background(), RateLimitRequest{
			Namespace: "anchat-test", Function: "user-create", Wallet: wallet, IP: ip, Override: override,
		})
		return d
	}
	if !signup("0xfresh1", "198.51.100.9").Allowed || !signup("0xfresh2", "198.51.100.9").Allowed {
		t.Fatal("the first two signups from an address were refused")
	}
	d := signup("0xfresh3", "198.51.100.9")
	if d.Allowed || d.Scope != "per_function_ip" {
		t.Fatalf("a third signup from the address with a fresh wallet got %+v, want per_function_ip", d)
	}
	if d.RetryAfter <= 29*time.Minute || d.RetryAfter > 30*time.Minute {
		t.Errorf("retry after %v, want about 30 minutes", d.RetryAfter)
	}
	if !signup("0xfresh4", "203.0.113.1").Allowed {
		t.Error("another address was refused")
	}
	// A key caller and an anonymous caller from the address are refused too.
	for name, req := range map[string]RateLimitRequest{
		"key caller": {Namespace: "anchat-test", Function: "user-create", Wallet: "anchat-test", Key: "key_a", IP: "198.51.100.9", Override: override},
		"anonymous":  {Namespace: "anchat-test", Function: "user-create", IP: "198.51.100.9", Override: override},
	} {
		if d, _ := l.AllowRequest(context.Background(), req); d.Allowed {
			t.Errorf("%s from the spent address was allowed", name)
		}
	}
}

// per_caller buckets a key caller by (key, address), like the per-caller tier:
// the users of one app's shared key do not spend each other's budget.
func TestMultiTier_per_function_caller_separates_key_callers(t *testing.T) {
	l := NewMultiTierLimiter(DefaultLimiterConfig())
	override := &FunctionRateLimit{PerCaller: RateLimitRule{Requests: 1, WindowSeconds: 3600}}
	call := func(key, ip string) Decision {
		d, _ := l.AllowRequest(context.Background(), RateLimitRequest{
			Namespace: "ns", Function: "rpc", Wallet: "ns", Key: key, IP: ip, Override: override,
		})
		return d
	}
	if !call("key_a", "1.1.1.1").Allowed {
		t.Fatal("first call refused")
	}
	if d := call("key_a", "1.1.1.1"); d.Allowed || d.Scope != "per_function_caller" {
		t.Fatalf("second call from the same key and address got %+v", d)
	}
	if !call("key_a", "2.2.2.2").Allowed || !call("key_b", "1.1.1.1").Allowed {
		t.Error("another user of the key, or another key, was refused")
	}
}

func TestMultiTier_anonymous_falls_back_to_ip(t *testing.T) {
	cfg := DefaultLimiterConfig()
	cfg.PerIPBurst = 2
	cfg.PerIPPerMinute = 60
	l := NewMultiTierLimiter(cfg)

	for i := 0; i < 2; i++ {
		d, _ := l.AllowRequest(context.Background(), RateLimitRequest{
			Namespace: "ns", IP: "1.2.3.4",
		})
		if !d.Allowed {
			t.Fatalf("IP burst[%d] should allow", i)
		}
	}
	d, _ := l.AllowRequest(context.Background(), RateLimitRequest{
		Namespace: "ns", IP: "1.2.3.4",
	})
	if d.Allowed {
		t.Fatal("expected per-IP rejection for anonymous caller")
	}
	if d.Scope != "per_ip" {
		t.Errorf("expected scope=per_ip, got %q", d.Scope)
	}

	// Different IP unaffected.
	d2, _ := l.AllowRequest(context.Background(), RateLimitRequest{
		Namespace: "ns", IP: "5.6.7.8",
	})
	if !d2.Allowed {
		t.Fatal("expected different IP to be allowed")
	}
}

func TestMultiTier_lru_eviction_when_cap_reached(t *testing.T) {
	cfg := DefaultLimiterConfig()
	cfg.PerWalletBurst = 1
	cfg.PerWalletPerMinute = 60
	cfg.MaxBucketsPerScope = 32 // 2 per shard with 16 shards
	l := NewMultiTierLimiter(cfg)

	// Saturate one wallet's bucket so its retry-after > 0.
	l.AllowRequest(context.Background(), RateLimitRequest{Namespace: "ns", Wallet: "victim"})
	d, _ := l.AllowRequest(context.Background(), RateLimitRequest{Namespace: "ns", Wallet: "victim"})
	if d.Allowed {
		t.Fatal("victim should be rate-limited initially")
	}

	// Push lots of new wallets through to evict the victim.
	// We need many to ensure same-shard collisions.
	for i := 0; i < 1000; i++ {
		l.AllowRequest(context.Background(), RateLimitRequest{
			Namespace: "ns", Wallet: "filler-" + string(rune(i%256)) + "-" + string(rune(i/256)),
		})
	}

	// After eviction, victim's bucket is recreated full → first call allowed again.
	// (We can't deterministically assert eviction without inspecting internals;
	// the test confirms LRU growth doesn't blow up — no panic, no deadlock.)
	_, err := l.AllowRequest(context.Background(), RateLimitRequest{
		Namespace: "ns", Wallet: "victim",
	})
	if err != nil {
		t.Errorf("unexpected error after LRU churn: %v", err)
	}
}

func TestMultiTier_concurrent_no_race(t *testing.T) {
	// Run with -race
	l := NewMultiTierLimiter(DefaultLimiterConfig())
	var wg sync.WaitGroup
	for g := 0; g < 16; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				l.AllowRequest(context.Background(), RateLimitRequest{
					Namespace: "ns",
					Function:  "fn",
					Wallet:    "w" + string(rune(id)),
					IP:        "1.2.3.4",
				})
			}
		}(g)
	}
	wg.Wait()
}

func TestMultiTier_satisfies_legacy_interface(t *testing.T) {
	var _ RateLimiter = (*MultiTierLimiter)(nil)
	var _ TieredRateLimiter = (*MultiTierLimiter)(nil)
}

func TestRateLimitedError_message(t *testing.T) {
	e := &RateLimitedError{Scope: "per_wallet", RetryAfter: 2 * time.Second}
	if e.Error() == "" {
		t.Error("expected non-empty error message")
	}
}

// Sanity-check the legacy TokenBucketLimiter still works for any code on the
// old single-bucket path.
func TestTokenBucketLimiter_legacy_works(t *testing.T) {
	l := NewTokenBucketLimiter(60)
	allowed, _ := l.Allow(context.Background(), "global")
	if !allowed {
		t.Error("first call should be allowed")
	}
}

// TestMultiTier_default_per_wallet_limit_matches_docs pins the figures
// website/src/docs/developer/functions.mdx ("Invoke rate limits") states: a wallet gets a 60-invoke
// burst, the 61st is refused as per_wallet, and the refill is 10 a second.
func TestMultiTier_default_per_wallet_limit_matches_docs(t *testing.T) {
	l := NewMultiTierLimiter(DefaultLimiterConfig())
	req := RateLimitRequest{Namespace: "ns", Function: "fn", Wallet: "w1"}
	for i := 0; i < 60; i++ {
		if d, _ := l.AllowRequest(context.Background(), req); !d.Allowed {
			t.Fatalf("invoke %d of the documented 60-invoke burst refused (scope=%s)", i+1, d.Scope)
		}
	}
	d, _ := l.AllowRequest(context.Background(), req)
	if d.Allowed || d.Scope != "per_wallet" {
		t.Fatalf("invoke 61 = %+v, want refused with scope=per_wallet", d)
	}
	if d.RetryAfter <= 0 || d.RetryAfter > 150*time.Millisecond {
		t.Errorf("RetryAfter = %s, want about 100ms (10 a second)", d.RetryAfter)
	}
}

// bug-934: callers with a credential and no wallet were bucketed by the
// namespace, so one heavy key starved every other caller of the namespace, and
// every user of an app's shared key starved each other. They are bucketed by
// (key, IP) now, at the per-wallet limits.
func TestMultiTier_key_callers_bucketed_by_key_and_ip(t *testing.T) {
	cfg := DefaultLimiterConfig()
	cfg.PerWalletBurst = 3
	cfg.PerWalletPerMinute = 60 // 1/sec — slow refill
	l := NewMultiTierLimiter(cfg)
	allow := func(req RateLimitRequest) Decision {
		t.Helper()
		d, err := l.AllowRequest(context.Background(), req)
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	// Every key caller of the namespace carries the namespace as its wallet,
	// as getWalletFromRequest answers it.
	heavy := RateLimitRequest{Namespace: "ns", Wallet: "ns", Key: "key_a", IP: "1.1.1.1"}
	for i := 0; i < 3; i++ {
		if !allow(heavy).Allowed {
			t.Fatalf("burst[%d] should be allowed", i)
		}
	}
	d := allow(heavy)
	if d.Allowed || d.Scope != "per_key" || d.RetryAfter <= 0 {
		t.Fatalf("after its burst the key caller got %+v, want a per_key refusal with a retry-after", d)
	}
	for name, other := range map[string]RateLimitRequest{
		"another key, same address":      {Namespace: "ns", Wallet: "ns", Key: "key_b", IP: "1.1.1.1"},
		"same key, another address":      {Namespace: "ns", Wallet: "ns", Key: "key_a", IP: "2.2.2.2"},
		"a wallet of the same namespace": {Namespace: "ns", Wallet: "0xwallet", IP: "1.1.1.1"},
		"same key in another namespace":  {Namespace: "other", Wallet: "other", Key: "key_a", IP: "1.1.1.1"},
	} {
		if !allow(other).Allowed {
			t.Errorf("%s was refused because one key caller spent its own bucket", name)
		}
	}
}
