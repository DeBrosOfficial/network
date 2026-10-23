package persistent

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/Orama-Network/orama/core/pkg/serverless"
	"go.uber.org/zap"
)

// TestUpdateInvocationContext_swapVisibleToWithInvCtx verifies the
// post-swap invCtx is what withInvCtx reads. Regression guard for
// bugboard #321 (mid-session JWT refresh on persistent WS).
func TestUpdateInvocationContext_swapVisibleToWithInvCtx(t *testing.T) {
	original := &serverless.InvocationContext{CallerJWTSubject: "user-A", WSClientID: "c1"}
	updated := &serverless.InvocationContext{CallerJWTSubject: "user-A-refreshed", WSClientID: "c1"}

	i := &Instance{invCtx: original}

	// Pre-swap: withInvCtx returns ctx carrying original.
	ctx := withInvCtxOnly(i)
	got := serverless.InvocationContextFromCtx(ctx)
	if got.CallerJWTSubject != "user-A" {
		t.Errorf("pre-swap: CallerJWTSubject = %q; want user-A", got.CallerJWTSubject)
	}

	// Swap.
	if err := i.UpdateInvocationContext(updated); err != nil {
		t.Fatalf("UpdateInvocationContext: %v", err)
	}

	// Post-swap: withInvCtx returns ctx carrying updated.
	ctx = withInvCtxOnly(i)
	got = serverless.InvocationContextFromCtx(ctx)
	if got.CallerJWTSubject != "user-A-refreshed" {
		t.Errorf("post-swap: CallerJWTSubject = %q; want user-A-refreshed", got.CallerJWTSubject)
	}
}

// TestUpdateInvocationContext_nilRejected ensures the nil-guard fires
// — silently accepting nil would re-open the cross-tenant identity
// leak the persistent invCtx exists to prevent.
func TestUpdateInvocationContext_nilRejected(t *testing.T) {
	original := &serverless.InvocationContext{CallerJWTSubject: "user-A"}
	i := &Instance{invCtx: original}

	err := i.UpdateInvocationContext(nil)
	if err == nil {
		t.Fatal("expected error for nil invCtx; got nil")
	}

	// Original must be untouched after the failed swap.
	ctx := withInvCtxOnly(i)
	got := serverless.InvocationContextFromCtx(ctx)
	if got.CallerJWTSubject != "user-A" {
		t.Errorf("after rejected nil swap: CallerJWTSubject = %q; want user-A (unchanged)",
			got.CallerJWTSubject)
	}
}

// TestUpdateInvocationContext_concurrentSwapsAndReads stresses the
// RWMutex contract: many concurrent withInvCtx readers + a writer
// swapping the pointer must never panic, deadlock, or produce a nil
// dereference. The race detector catches torn reads/writes.
func TestUpdateInvocationContext_concurrentSwapsAndReads(t *testing.T) {
	a := &serverless.InvocationContext{CallerJWTSubject: "a"}
	b := &serverless.InvocationContext{CallerJWTSubject: "b"}
	i := &Instance{invCtx: a}

	const (
		readers   = 16
		writes    = 100
		readsPerW = 50
	)
	var wg sync.WaitGroup

	// Reader pool — each loops reading via withInvCtx.
	var readsObserved int64
	for r := 0; r < readers; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for n := 0; n < writes*readsPerW; n++ {
				ctx := withInvCtxOnly(i)
				if got := serverless.InvocationContextFromCtx(ctx); got == nil {
					t.Errorf("withInvCtx returned ctx with nil invCtx during concurrent swap")
					return
				}
				atomic.AddInt64(&readsObserved, 1)
			}
		}()
	}

	// Writer: alternates between a and b.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for n := 0; n < writes; n++ {
			cur := a
			if n%2 == 1 {
				cur = b
			}
			if err := i.UpdateInvocationContext(cur); err != nil {
				t.Errorf("UpdateInvocationContext concurrent write: %v", err)
				return
			}
		}
	}()

	wg.Wait()

	if atomic.LoadInt64(&readsObserved) == 0 {
		t.Error("no successful reads observed during concurrent test")
	}
}

// TestUpdateInvocationContext_swapDoesNotAffectInFlightCtx — the ctx
// already returned by an earlier withInvCtx call MUST keep carrying
// the OLD invCtx pointer, even after a later swap. Otherwise an
// in-flight WASM-host call would see its identity change mid-call.
// Bugboard #321 design correctness check.
func TestUpdateInvocationContext_swapDoesNotAffectInFlightCtx(t *testing.T) {
	original := &serverless.InvocationContext{CallerJWTSubject: "before"}
	updated := &serverless.InvocationContext{CallerJWTSubject: "after"}
	i := &Instance{invCtx: original}

	// Snapshot a ctx using the original invCtx.
	inflightCtx := withInvCtxOnly(i)

	// Swap.
	if err := i.UpdateInvocationContext(updated); err != nil {
		t.Fatalf("UpdateInvocationContext: %v", err)
	}

	// The previously-captured ctx still carries "before".
	got := serverless.InvocationContextFromCtx(inflightCtx)
	if got.CallerJWTSubject != "before" {
		t.Errorf("in-flight ctx changed under swap: got %q; want 'before' (an in-flight invocation must complete under its original identity)",
			got.CallerJWTSubject)
	}

	// New withInvCtx calls see "after".
	freshCtx := withInvCtxOnly(i)
	got = serverless.InvocationContextFromCtx(freshCtx)
	if got.CallerJWTSubject != "after" {
		t.Errorf("post-swap fresh ctx = %q; want 'after'", got.CallerJWTSubject)
	}
}

// withInvCtxOnly is withInvCtx for tests that read only the context.
func withInvCtxOnly(i *Instance) context.Context {
	ctx, _ := i.withInvCtx(context.Background())
	return ctx
}

// feat-263: every export call of a reused persistent instance gets its own
// function_invoke_last_error slot, so one frame never reads the error of a
// nested invoke another frame made.
func TestWithInvCtx_freshInvokeErrorSlotPerCall(t *testing.T) {
	i := &Instance{invCtx: &serverless.InvocationContext{Namespace: "ns"}, logger: zap.NewNop()}
	first := withInvCtxOnly(i)
	serverless.RecordInvokeOutcome(first, serverless.ErrUnauthorized)
	if serverless.LastInvokeErrorJSON(first) == nil {
		t.Fatal("the first call's slot did not record the error, so the test checks nothing")
	}
	if got := serverless.LastInvokeErrorJSON(withInvCtxOnly(i)); got != nil {
		t.Fatalf("the next call on the same instance reads %s", got)
	}
}
