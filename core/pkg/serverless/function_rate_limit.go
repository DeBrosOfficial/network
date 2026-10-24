package serverless

import (
	"fmt"
	"time"
)

// A function's own invoke limits: `rate_limit` in function.yaml (bug-740,
// feat-938). They tighten the gateway-wide tiers for one function and never
// loosen them: a call has to pass these and then the per-caller and
// per-namespace buckets every invoke is charged to.
//
//	rate_limit:
//	  per_caller: {requests: 5, window: 1m} # each caller of this function
//	  per_ip:     {requests: 2, window: 1h} # each client address, whoever signed in
//
// A rule is a token bucket that holds `requests` and refills them evenly over
// `window`: the first `requests` calls pass at once, and after that one more
// every window/requests. Two signups an hour per address is therefore two at
// once, then one every thirty minutes.

// Bounds of a rule. The window is whole seconds from one second to a day: a
// longer window outlives the bucket it is counted in (buckets are per gateway
// process, held in a bounded LRU, and emptied by a restart), so it would
// promise more than the gateway keeps.
const (
	MaxRateLimitRequests = 100_000
	MinRateLimitWindow   = time.Second
	MaxRateLimitWindow   = 24 * time.Hour
)

// Field names a refused rule is reported under, as function.yaml spells them.
const (
	fieldRateLimitPerCaller = "rate_limit.per_caller"
	fieldRateLimitPerIP     = "rate_limit.per_ip"
)

// FunctionRateLimit is a function's own invoke limits. A rule with zero
// requests is absent.
type FunctionRateLimit struct {
	// PerCaller is charged to each caller: a wallet; a caller with a
	// credential and no wallet by (key, address); an anonymous caller by
	// address — the same identity the gateway-wide tier buckets it by.
	PerCaller RateLimitRule `json:"per_caller"`
	// PerIP is charged to each client address, whoever is signed in, so a
	// fresh wallet per call does not escape it (account creation, feat-938).
	PerIP RateLimitRule `json:"per_ip"`
}

// RateLimitRule allows Requests calls per WindowSeconds.
type RateLimitRule struct {
	Requests      int `json:"requests"`
	WindowSeconds int `json:"window_seconds"`
}

// Set reports whether the rule limits anything.
func (r RateLimitRule) Set() bool { return r.Requests > 0 }

// ratePerSecond is the bucket's refill rate.
func (r RateLimitRule) ratePerSecond() float64 {
	return float64(r.Requests) / float64(r.WindowSeconds)
}

// burst is the bucket's capacity: a full window's requests.
func (r RateLimitRule) burst() float64 { return float64(r.Requests) }

func (r RateLimitRule) validate(field string) error {
	if r.Requests == 0 && r.WindowSeconds == 0 {
		return nil
	}
	if r.Requests < 1 || r.Requests > MaxRateLimitRequests {
		return &ValidationError{Field: field + ".requests",
			Message: fmt.Sprintf("must be from 1 to %d, got %d", MaxRateLimitRequests, r.Requests)}
	}
	window := time.Duration(r.WindowSeconds) * time.Second
	if window < MinRateLimitWindow || window > MaxRateLimitWindow {
		return &ValidationError{Field: field + ".window",
			Message: fmt.Sprintf("must be from %s to %s, got %ds", MinRateLimitWindow, MaxRateLimitWindow, r.WindowSeconds)}
	}
	return nil
}

// Validate refuses a rule out of bounds. A nil limit is no limit.
func (l *FunctionRateLimit) Validate() error {
	if l == nil {
		return nil
	}
	if err := l.PerCaller.validate(fieldRateLimitPerCaller); err != nil {
		return err
	}
	return l.PerIP.validate(fieldRateLimitPerIP)
}

// rateLimitFromColumns is the limit a function row holds, nil when it holds
// none, so a function deployed without one reads back exactly as it was sent.
func rateLimitFromColumns(callerRequests, callerWindow, ipRequests, ipWindow int) *FunctionRateLimit {
	l := &FunctionRateLimit{
		PerCaller: RateLimitRule{Requests: callerRequests, WindowSeconds: callerWindow},
		PerIP:     RateLimitRule{Requests: ipRequests, WindowSeconds: ipWindow},
	}
	if !l.PerCaller.Set() && !l.PerIP.Set() {
		return nil
	}
	return l
}

// columns are the four values the registry stores the limit in.
func (l *FunctionRateLimit) columns() (callerRequests, callerWindow, ipRequests, ipWindow int) {
	if l == nil {
		return 0, 0, 0, 0
	}
	return l.PerCaller.Requests, l.PerCaller.WindowSeconds, l.PerIP.Requests, l.PerIP.WindowSeconds
}
