package serverless

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/zap"
)

// capturingLimiter refuses every request and keeps the last one it was asked
// about, so a test sees exactly what the engine put in front of the limiter.
type capturingLimiter struct{ last RateLimitRequest }

func (c *capturingLimiter) Allow(context.Context, string) (bool, error) { return false, nil }

func (c *capturingLimiter) AllowRequest(_ context.Context, req RateLimitRequest) (Decision, error) {
	c.last = req
	return Decision{Allowed: false, Scope: "per_key"}, nil
}

// executeAgainst runs fn through an engine whose limiter is lim and returns
// the error, which the refusing limiter makes a RateLimitedError.
func executeAgainst(t *testing.T, lim *capturingLimiter, fn *Function, invCtx *InvocationContext) error {
	t.Helper()
	engine, err := NewEngine(nil, NewMockRegistry(), NewMockHostServices(), zap.NewNop(), WithRateLimiter(lim))
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	t.Cleanup(func() { _ = engine.runtime.Close(context.Background()) })
	_, err = engine.Execute(context.Background(), fn, nil, invCtx)
	return err
}

// The engine hands the limiter the caller's credential fingerprint, so a
// caller with no wallet is bucketed by its own key (bug-934).
func TestEngine_rateLimitRequestCarriesTheCallersKey(t *testing.T) {
	lim := &capturingLimiter{}
	err := executeAgainst(t, lim, &Function{ID: "f1", Name: "rpc", Namespace: "anchat"}, &InvocationContext{
		Namespace: "anchat", FunctionName: "rpc", CallerWallet: "anchat", CallerKey: "key_0123456789ab", CallerIP: "203.0.113.7",
	})
	var rle *RateLimitedError
	if !errors.As(err, &rle) {
		t.Fatalf("Execute returned %v, want the limiter's refusal", err)
	}
	if lim.last.Key != "key_0123456789ab" || lim.last.IP != "203.0.113.7" || lim.last.Wallet != "anchat" {
		t.Errorf("the limiter was asked about %+v", lim.last)
	}
}

// bug-740: the engine never filled RateLimitRequest.Override, so a function's
// own limit could not run. It is the deployed function's RateLimit now.
func TestEngine_rateLimitRequestCarriesTheFunctionsOwnLimit(t *testing.T) {
	lim := &capturingLimiter{}
	own := &FunctionRateLimit{PerIP: RateLimitRule{Requests: 2, WindowSeconds: 3600}}
	_ = executeAgainst(t, lim, &Function{ID: "f1", Name: "user-create", Namespace: "anchat-test", RateLimit: own},
		&InvocationContext{Namespace: "anchat-test", FunctionName: "user-create", CallerIP: "198.51.100.9"})
	if lim.last.Override != own {
		t.Errorf("the limiter was asked with override %+v, want the function's %+v", lim.last.Override, own)
	}
}
