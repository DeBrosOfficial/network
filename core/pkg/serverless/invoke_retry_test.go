package serverless

import (
	"testing"
	"time"
)

// TestTimeoutIsNotResourceExhaustion: a function that ran out of time ran, so
// it is not classed with the refusals (bug-936). The gateway maps the two to
// different codes on that distinction.
func TestTimeoutIsNotResourceExhaustion(t *testing.T) {
	timedOut := &ExecutionError{FunctionName: "fn", Cause: ErrTimeout}
	if IsResourceExhausted(timedOut) {
		t.Error("a timed-out invocation is classed as resource exhaustion")
	}
	if !IsTimeout(timedOut) {
		t.Error("a wrapped ErrTimeout is not recognised as a timeout")
	}
	if IsTimeout(&RateLimitedError{Scope: "per_wallet"}) {
		t.Error("a rate-limit refusal is recognised as a timeout")
	}
}

// TestRateLimitedErrorIsErrRateLimited: the tiered refusal is a rate limit to
// every classifier, not only to the HTTP handler that unwraps it by type.
func TestRateLimitedErrorIsErrRateLimited(t *testing.T) {
	if !IsResourceExhausted(&RateLimitedError{Scope: "per_ip", RetryAfter: time.Second}) {
		t.Error("RateLimitedError is not resource exhaustion")
	}
}

// TestInvokerDoesNotRetryTimeoutOrRateLimit: the invoker re-runs a failed
// invocation up to the function's retry count, but never one that ran out of
// time (its effects may have happened) nor one the limiter refused (a retry is
// refused the same way and spends the caller's budget).
func TestInvokerDoesNotRetryTimeoutOrRateLimit(t *testing.T) {
	inv := &Invoker{}
	cases := map[string]error{
		"timeout":    &ExecutionError{FunctionName: "fn", Cause: ErrTimeout},
		"rate limit": &RateLimitedError{Scope: "per_wallet", RetryAfter: time.Second},
	}
	for name, err := range cases {
		if inv.isRetryable(err) {
			t.Errorf("%s: the invoker would re-run it", name)
		}
	}
}
