package hostfunctions

import (
	"errors"
	"testing"
	"time"

	"github.com/Orama-Network/orama/core/pkg/olric/olrictest"
	"github.com/Orama-Network/orama/core/pkg/serverless"
	olriclib "github.com/olric-data/olric"
)

// bug-741: a time-bucketed counter (rl:<caller>:<minute>) has to expire, and
// cache_incr / cache_incr_by cannot give it an expiry. cache_incr_ttl sets one
// when it creates the counter and leaves an existing counter's alone.

func TestCacheIncrByTTL_createsTheCounterWithItsExpiry(t *testing.T) {
	h := newEmbeddedCache(t)
	ctx := nsCtx()

	if got, err := h.CacheIncrByTTL(ctx, "rl:w1:100", 1, 60); err != nil || got != 1 {
		t.Fatalf("CacheIncrByTTL on a missing key = %d, %v; want 1", got, err)
	}
	first := storedTTL(t, h, "rl:w1:100")
	if first == 0 {
		t.Fatal("a counter cache_incr_ttl created has no expiry")
	}
	if remaining := time.Until(time.UnixMilli(first)); remaining <= 55*time.Second || remaining > 61*time.Second {
		t.Fatalf("expiry %v from now, want about 60s", remaining)
	}

	if got, err := h.CacheIncrByTTL(ctx, "rl:w1:100", 4, 3600); err != nil || got != 5 {
		t.Fatalf("second CacheIncrByTTL = %d, %v; want 5", got, err)
	}
	if drift := storedTTL(t, h, "rl:w1:100") - first; drift < -expiryDriftMs || drift > expiryDriftMs {
		t.Fatalf("an increment of an existing counter moved its expiry by %dms", drift)
	}
}

func TestCacheIncrByTTL_counterExpiresAndStartsOver(t *testing.T) {
	h := newEmbeddedCache(t)
	ctx := nsCtx()

	if got, err := h.CacheIncrByTTL(ctx, "bucket", 2, 1); err != nil || got != 2 {
		t.Fatalf("CacheIncrByTTL = %d, %v; want 2", got, err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := h.CacheGet(ctx, "bucket"); errors.Is(err, olriclib.ErrKeyNotFound) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("a counter created with a 1s ttl was still readable after 5s")
		}
		time.Sleep(50 * time.Millisecond)
	}
	if got, err := h.CacheIncrByTTL(ctx, "bucket", 2, 60); err != nil || got != 2 {
		t.Fatalf("CacheIncrByTTL after expiry = %d, %v; want a new counter at 2", got, err)
	}
	if storedTTL(t, h, "bucket") == 0 {
		t.Fatal("the recreated counter has no expiry")
	}
}

// A counter that existed without an expiry keeps none: cache_incr_ttl only
// sets the expiry of a counter it creates.
func TestCacheIncrByTTL_existingCounterKeepsItsExpiry(t *testing.T) {
	h := newEmbeddedCache(t)
	ctx := nsCtx()

	if _, err := h.CacheIncrBy(ctx, "plain", 3); err != nil {
		t.Fatalf("CacheIncrBy: %v", err)
	}
	if got, err := h.CacheIncrByTTL(ctx, "plain", 1, 60); err != nil || got != 4 {
		t.Fatalf("CacheIncrByTTL on an existing counter = %d, %v; want 4", got, err)
	}
	if ttl := storedTTL(t, h, "plain"); ttl != 0 {
		t.Fatalf("an existing counter with no expiry got TTL %d", ttl)
	}
}

func TestCacheIncrByTTL_zeroTTLIsNoExpiry(t *testing.T) {
	h := newEmbeddedCache(t)
	if got, err := h.CacheIncrByTTL(nsCtx(), "z", 1, 0); err != nil || got != 1 {
		t.Fatalf("CacheIncrByTTL ttl 0 = %d, %v; want 1", got, err)
	}
	if ttl := storedTTL(t, h, "z"); ttl != 0 {
		t.Fatalf("ttl 0 stored TTL %d, want none", ttl)
	}
}

func TestCacheIncrByTTL_refusesAnOutOfRangeTTL(t *testing.T) {
	h := newEmbeddedCache(t)
	for _, ttl := range []int64{-1, maxCacheTTLSeconds + 1} {
		if _, err := h.CacheIncrByTTL(nsCtx(), "k", 1, ttl); !errors.Is(err, serverless.ErrInvalidCacheTTL) {
			t.Errorf("ttl %d: err = %v, want ErrInvalidCacheTTL", ttl, err)
		}
	}
	if _, err := h.CacheGet(nsCtx(), "k"); !errors.Is(err, serverless.ErrCacheMiss) {
		t.Errorf("a refused call created the counter (CacheGet err = %v)", err)
	}
}

// A gateway's cluster client never registered olric's error codes, so the NX
// Put that finds the counter comes back as a plain "key found" error. That is
// the counter existing, not a failure.
func TestCacheIncrByTTL_throughAClusterClient(t *testing.T) {
	members := olrictest.StartCluster(t, 1)
	h := &HostFunctions{cacheClient: fixedOlric(clusterClient(t, members[0]))}
	ctx := nsCtx()

	for want := int64(1); want <= 3; want++ {
		if got, err := h.CacheIncrByTTL(ctx, "cc", 1, 60); err != nil || got != want {
			t.Fatalf("increment %d through a cluster client = %d, %v", want, got, err)
		}
	}
	if storedTTL(t, h, "cc") == 0 {
		t.Fatal("the counter has no expiry")
	}
}
