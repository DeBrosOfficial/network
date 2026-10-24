package serverless

// ratelimit.go provides a multi-tier token-bucket rate limiter for serverless
// function invocations. Four tiers, applied in order; first rejection wins:
//
//  1. The function's own limits — only when it declares them (rate_limit in
//     function.yaml, FunctionRateLimit): per (function, caller) and per
//     (function, client address)
//  2. Per-caller — gateway-wide default per-user limit
//  3. Per client network for authenticated callers — only in a namespace that
//     turned its per-IP limits on (NamespaceLimits, bugboard #749)
//  4. Per-namespace total — protects against single-namespace exhaustion
//
// Tier 2 knows three kinds of caller. A wallet has its own bucket. A caller
// with no wallet that presented a credential (an API key, a key's or a
// workload's token) is bucketed by (key fingerprint, IP): an app ships one key
// to all its users, so the key alone would put them all in one bucket, and the
// namespace — what such a caller used to be bucketed by — did exactly that for
// every key of the namespace (bug-934). An anonymous caller is bucketed by IP.
//
// Tier 3 is for a namespace whose sign-in is open: a wallet costs nothing
// there, so one host holding many identities multiplies tier 2 by their
// number. It buckets an authenticated caller (a wallet or a key) by its client
// network, an IPv6 address folded to its /64 (clientkey.BucketKey), and comes
// after tier 2 so one identity meets its own limit first. An anonymous caller
// is already bucketed by address in tier 2.
//
// Per-bucket state is held in a sharded LRU to bound memory: configurable
// MaxBucketsPerScope (default 100k per tier) — buckets evicted are
// effectively "limit reset for that key" which is acceptable.
//
// See plan: core/plans/platform/09_PER_WALLET_RATE_LIMIT.md.

import (
	"container/list"
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"sync"
	"time"

	"github.com/Orama-Network/orama/core/pkg/gateway/clientkey"
)

// NamespaceLimits answers the per-namespace settings the limiter applies.
// AuthenticatedInvokesPerIP is a namespace's ceiling on authenticated invokes
// per client network, in requests per minute, or 0 when it has none. It is
// asked on every authenticated invoke, so it answers from a cache.
type NamespaceLimits interface {
	AuthenticatedInvokesPerIP(ctx context.Context, namespace string) (int, error)
}

// ErrInvokeLimitsUnavailable is an invoke the limiter could not judge: the
// namespace's limits could not be read. The invoke is refused rather than run
// past a ceiling the namespace may have set; retryable.
var ErrInvokeLimitsUnavailable = errors.New("the namespace's invoke limits cannot be read right now")

// perIPAuthenticatedBurstDivisor sets the burst of the per-network tier at a
// tenth of its per-minute rate, the ratio of the per-wallet and per-namespace
// defaults.
const perIPAuthenticatedBurstDivisor = 10

// RateLimitRequest holds the inputs for a single Allow check.
type RateLimitRequest struct {
	Namespace string
	Function  string
	Wallet    string             // empty = anonymous; per-wallet tier falls back to per-IP
	Key       string             // credential fingerprint of a caller with no wallet; see InvokeRequest.CallerKey
	IP        string             // remote IP: the anonymous bucket, and half of a key caller's
	Override  *FunctionRateLimit // the function's own limits, nil when it declares none
}

// LimiterConfig holds gateway-wide rate-limiter defaults. Zero values mean
// "use the built-in default" — see DefaultLimiterConfig.
type LimiterConfig struct {
	// Per-(namespace, wallet) defaults
	PerWalletPerMinute int
	PerWalletBurst     int

	// Per-(namespace) ceiling
	PerNamespacePerMinute int
	PerNamespaceBurst     int

	// Per-IP bucket for anonymous callers
	PerIPPerMinute int
	PerIPBurst     int

	// LRU cap for tracked buckets per tier
	MaxBucketsPerScope int
}

// DefaultLimiterConfig returns a sensible default config. Tuned for typical
// per-user app load with headroom for short bursts.
func DefaultLimiterConfig() LimiterConfig {
	return LimiterConfig{
		PerWalletPerMinute:    600,    // 10/sec sustained
		PerWalletBurst:        60,     // 60-token burst window
		PerNamespacePerMinute: 60_000, // 1k/sec sustained per namespace
		PerNamespaceBurst:     6000,
		PerIPPerMinute:        120, // 2/sec for anonymous
		PerIPBurst:            30,
		MaxBucketsPerScope:    100_000,
	}
}

// Decision is the result of an Allow check.
type Decision struct {
	Allowed    bool
	RetryAfter time.Duration
	Scope      string // "per_function_caller" | "per_function_ip" | "per_wallet" | "per_key" | "per_ip" | "per_ip_authenticated" | "per_namespace"
}

// RateLimitedError is returned by the engine when a request is rejected.
// Carries the retry-after for the gateway HTTP layer to serve as Retry-After.
type RateLimitedError struct {
	Scope      string
	RetryAfter time.Duration
}

func (e *RateLimitedError) Error() string {
	return fmt.Sprintf("rate limit exceeded (scope=%s, retry_after=%s)", e.Scope, e.RetryAfter)
}

// Is makes a RateLimitedError an ErrRateLimited, so everything that asks
// IsResourceExhausted — the invoker's retry decision, the WebSocket error
// frame — reads a tiered refusal as the rate limit it is.
func (e *RateLimitedError) Is(target error) bool { return target == ErrRateLimited }

// MultiTierLimiter implements RateLimiter as layered token-bucket scopes with
// sharded LRU bucket tracking.
type MultiTierLimiter struct {
	cfg LimiterConfig

	// nsLimits answers the namespaces' own settings; nil until
	// SetNamespaceLimits, and then no namespace has any.
	nsLimits NamespaceLimits

	// Tier buckets — each scope owns its own LRU.
	fnCallerBuckets  *lruBuckets // (ns, fn, caller) — only for functions that declare per_caller
	fnIPBuckets      *lruBuckets // (ns, fn, ip) — only for functions that declare per_ip
	walletBuckets    *lruBuckets // (ns, wallet)
	keyBuckets       *lruBuckets // (ns, key, ip) — callers with a credential and no wallet
	namespaceBuckets *lruBuckets // (ns)
	ipBuckets        *lruBuckets // (ns, ip) — for anonymous callers
	authIPBuckets    *lruBuckets // (ns, client network) — authenticated callers, where the namespace set a ceiling
}

// NewMultiTierLimiter constructs a limiter with the given config. Pass
// DefaultLimiterConfig() for sensible defaults; override fields as needed.
func NewMultiTierLimiter(cfg LimiterConfig) *MultiTierLimiter {
	if cfg.PerWalletPerMinute <= 0 {
		cfg.PerWalletPerMinute = 600
	}
	if cfg.PerWalletBurst <= 0 {
		cfg.PerWalletBurst = 60
	}
	if cfg.PerNamespacePerMinute <= 0 {
		cfg.PerNamespacePerMinute = 60_000
	}
	if cfg.PerNamespaceBurst <= 0 {
		cfg.PerNamespaceBurst = 6000
	}
	if cfg.PerIPPerMinute <= 0 {
		cfg.PerIPPerMinute = 120
	}
	if cfg.PerIPBurst <= 0 {
		cfg.PerIPBurst = 30
	}
	if cfg.MaxBucketsPerScope <= 0 {
		cfg.MaxBucketsPerScope = 100_000
	}
	return &MultiTierLimiter{
		cfg:              cfg,
		fnCallerBuckets:  newLRUBuckets(cfg.MaxBucketsPerScope),
		fnIPBuckets:      newLRUBuckets(cfg.MaxBucketsPerScope),
		walletBuckets:    newLRUBuckets(cfg.MaxBucketsPerScope),
		keyBuckets:       newLRUBuckets(cfg.MaxBucketsPerScope),
		namespaceBuckets: newLRUBuckets(cfg.MaxBucketsPerScope),
		ipBuckets:        newLRUBuckets(cfg.MaxBucketsPerScope),
		authIPBuckets:    newLRUBuckets(cfg.MaxBucketsPerScope),
	}
}

// SetNamespaceLimits gives the limiter the namespaces' own settings. Called
// once at startup, before the first invoke.
func (l *MultiTierLimiter) SetNamespaceLimits(src NamespaceLimits) {
	l.nsLimits = src
}

// AllowRequest returns the layered decision. The first tier to reject wins;
// on rejection, RetryAfter is the wait until that tier could accept again.
//
// This is the rich path; the engine prefers it via type assertion. The
// legacy `Allow(ctx, key)` method below remains for back-compat with the
// simple RateLimiter interface.
func (l *MultiTierLimiter) AllowRequest(ctx context.Context, req RateLimitRequest) (Decision, error) {
	// Tier 1: the function's own limits — only when it declared them.
	if o := req.Override; o != nil {
		fn := req.Namespace + "/" + req.Function + "/"
		if o.PerIP.Set() && req.IP != "" {
			if d := l.fnIPBuckets.tryConsume(fn+req.IP, o.PerIP.ratePerSecond(), o.PerIP.burst()); !d.Allowed {
				d.Scope = "per_function_ip"
				return d, nil
			}
		}
		if caller := req.caller(); o.PerCaller.Set() && caller != "" {
			if d := l.fnCallerBuckets.tryConsume(fn+caller, o.PerCaller.ratePerSecond(), o.PerCaller.burst()); !d.Allowed {
				d.Scope = "per_function_caller"
				return d, nil
			}
		}
	}

	// Tier 2: per-(ns, key, ip) for a credential with no wallet, per-(ns,
	// wallet), or per-(ns, ip) for anonymous. A key caller has the per-wallet
	// limits: it is one user of an app, like a wallet.
	if req.Key != "" {
		key := req.Namespace + "/" + req.caller()
		if d := l.keyBuckets.tryConsume(key,
			float64(l.cfg.PerWalletPerMinute)/60.0,
			float64(l.cfg.PerWalletBurst)); !d.Allowed {
			d.Scope = "per_key"
			return d, nil
		}
	} else if req.Wallet != "" {
		key := req.Namespace + "/" + req.Wallet
		if d := l.walletBuckets.tryConsume(key,
			float64(l.cfg.PerWalletPerMinute)/60.0,
			float64(l.cfg.PerWalletBurst)); !d.Allowed {
			d.Scope = "per_wallet"
			return d, nil
		}
	} else if req.IP != "" {
		key := req.Namespace + "/" + req.IP
		if d := l.ipBuckets.tryConsume(key,
			float64(l.cfg.PerIPPerMinute)/60.0,
			float64(l.cfg.PerIPBurst)); !d.Allowed {
			d.Scope = "per_ip"
			return d, nil
		}
	}

	// Tier 3: per client network for an authenticated caller, where the
	// namespace set a ceiling.
	if d, err := l.allowAuthenticatedIP(ctx, req); err != nil || !d.Allowed {
		return d, err
	}

	// Tier 4: per-namespace total (always applies).
	if req.Namespace != "" {
		if d := l.namespaceBuckets.tryConsume(req.Namespace,
			float64(l.cfg.PerNamespacePerMinute)/60.0,
			float64(l.cfg.PerNamespaceBurst)); !d.Allowed {
			d.Scope = "per_namespace"
			return d, nil
		}
	}

	return Decision{Allowed: true}, nil
}

// allowAuthenticatedIP charges an authenticated caller to its client network
// when its namespace has a ceiling on that.
func (l *MultiTierLimiter) allowAuthenticatedIP(ctx context.Context, req RateLimitRequest) (Decision, error) {
	if l.nsLimits == nil || req.IP == "" || req.Namespace == "" || (req.Key == "" && req.Wallet == "") {
		return Decision{Allowed: true}, nil
	}
	perMinute, err := l.nsLimits.AuthenticatedInvokesPerIP(ctx, req.Namespace)
	if err != nil {
		return Decision{}, fmt.Errorf("%w: %v", ErrInvokeLimitsUnavailable, err)
	}
	if perMinute <= 0 {
		return Decision{Allowed: true}, nil
	}
	burst := max(1, perMinute/perIPAuthenticatedBurstDivisor)
	d := l.authIPBuckets.tryConsume(req.Namespace+"/"+clientkey.BucketKey(req.IP), float64(perMinute)/60.0, float64(burst))
	if !d.Allowed {
		d.Scope = "per_ip_authenticated"
	}
	return d, nil
}

// caller is the identity a request is charged to, the same one the per-caller
// tier buckets it by: a credential with no wallet by (key, address), a wallet
// by itself, an anonymous caller by address. Each kind has its own prefix so
// no value of one can collide with another.
func (req RateLimitRequest) caller() string {
	switch {
	case req.Key != "":
		return "k:" + req.Key + "/" + req.IP
	case req.Wallet != "":
		return "w:" + req.Wallet
	case req.IP != "":
		return "ip:" + req.IP
	default:
		return ""
	}
}

// Allow satisfies the legacy serverless.RateLimiter interface. Treats `key`
// as the wallet/namespace combo from older call sites. Prefer AllowRequest
// in new code.
func (l *MultiTierLimiter) Allow(ctx context.Context, key string) (bool, error) {
	d, err := l.AllowRequest(ctx, RateLimitRequest{Namespace: "_global", Wallet: key})
	return d.Allowed, err
}

// ----------------------------------------------------------------------------
// Backward compatibility: keep the old TokenBucketLimiter type as a
// single-bucket impl that satisfies the old simple interface. New code uses
// MultiTierLimiter exclusively.
// ----------------------------------------------------------------------------

// TokenBucketLimiter is a single global bucket — kept for backwards compat.
// Prefer MultiTierLimiter for any new use.
type TokenBucketLimiter struct {
	mu       sync.Mutex
	tokens   float64
	max      float64
	refill   float64
	lastTime time.Time
}

// NewTokenBucketLimiter creates a single-bucket limiter with the given per-minute limit.
func NewTokenBucketLimiter(perMinute int) *TokenBucketLimiter {
	perSecond := float64(perMinute) / 60.0
	return &TokenBucketLimiter{
		tokens:   float64(perMinute),
		max:      float64(perMinute),
		refill:   perSecond,
		lastTime: time.Now(),
	}
}

// Allow checks if a request should be allowed.
// The key argument is ignored — this is a single global bucket.
func (t *TokenBucketLimiter) Allow(_ context.Context, _ string) (bool, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(t.lastTime).Seconds()
	t.lastTime = now

	t.tokens += elapsed * t.refill
	if t.tokens > t.max {
		t.tokens = t.max
	}
	if t.tokens < 1.0 {
		return false, nil
	}
	t.tokens--
	return true, nil
}

// ----------------------------------------------------------------------------
// lruBuckets: sharded map of token buckets with LRU eviction
// ----------------------------------------------------------------------------

const lruShards = 16

type lruBuckets struct {
	shards [lruShards]*bucketShard
}

type bucketShard struct {
	mu      sync.Mutex
	buckets map[string]*tokenBucket
	order   *list.List // each Element.Value is a string (the key); front = most recent
	keyToEl map[string]*list.Element
	cap     int
}

func newLRUBuckets(capacity int) *lruBuckets {
	per := capacity / lruShards
	if per < 1 {
		per = 1
	}
	lb := &lruBuckets{}
	for i := range lb.shards {
		lb.shards[i] = &bucketShard{
			buckets: make(map[string]*tokenBucket, per),
			order:   list.New(),
			keyToEl: make(map[string]*list.Element, per),
			cap:     per,
		}
	}
	return lb
}

// tryConsume looks up or creates the bucket for `key`, attempts to consume
// one token, and returns the decision. Updates LRU on touch and evicts the
// least-recently-used bucket when the shard is at capacity.
func (l *lruBuckets) tryConsume(key string, ratePerSec, burst float64) Decision {
	s := l.shards[shardIdx(key)]
	s.mu.Lock()
	defer s.mu.Unlock()

	b, ok := s.buckets[key]
	if !ok {
		// Capacity check + LRU eviction.
		if len(s.buckets) >= s.cap {
			oldest := s.order.Back()
			if oldest != nil {
				oldKey := oldest.Value.(string)
				delete(s.buckets, oldKey)
				delete(s.keyToEl, oldKey)
				s.order.Remove(oldest)
			}
		}
		b = &tokenBucket{
			tokens:     burst, // start full
			max:        burst,
			refill:     ratePerSec,
			lastRefill: time.Now(),
		}
		s.buckets[key] = b
		s.keyToEl[key] = s.order.PushFront(key)
	} else {
		// Touch LRU.
		s.order.MoveToFront(s.keyToEl[key])
		// Update rate/burst in case the function's override changed since last call.
		b.refill = ratePerSec
		b.max = burst
	}

	return b.tryConsume()
}

// tokenBucket is a leaky bucket; not safe for concurrent use without external lock.
type tokenBucket struct {
	tokens     float64
	max        float64
	refill     float64 // tokens per second
	lastRefill time.Time
}

func (b *tokenBucket) tryConsume() Decision {
	now := time.Now()
	elapsed := now.Sub(b.lastRefill).Seconds()
	b.lastRefill = now
	b.tokens += elapsed * b.refill
	if b.tokens > b.max {
		b.tokens = b.max
	}
	if b.tokens < 1.0 {
		// Compute how long until we have one token.
		needed := 1.0 - b.tokens
		seconds := needed / b.refill
		return Decision{
			Allowed:    false,
			RetryAfter: time.Duration(seconds * float64(time.Second)),
		}
	}
	b.tokens--
	return Decision{Allowed: true}
}

// shardIdx hashes key to one of lruShards.
func shardIdx(key string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	return h.Sum32() % lruShards
}
