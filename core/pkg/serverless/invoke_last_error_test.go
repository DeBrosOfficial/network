package serverless

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/tetratelabs/wazero/api"
	"go.uber.org/zap"
)

// feat-263: function_invoke returns 0 for every failure; a guest reads which
// one from function_invoke_last_error.

func TestClassifyInvokeError(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want string
	}{
		{fmt.Errorf("invoke: %w", ErrUnauthorized), InvokeCodeAuthForbidden},
		{ErrFunctionNotFound, InvokeCodeNotFound},
		{ErrVersionSuperseded, InvokeCodeNotFound},
		{&ExecutionError{FunctionName: "f", Cause: ErrTimeout}, InvokeCodeDeadlineExceeded},
		{&RateLimitedError{Scope: "per_function"}, InvokeCodeRateLimited},
		{ErrRateLimited, InvokeCodeRateLimited},
		{ErrPayloadTooLarge, InvokeCodeInvalidArgument},
		{&ValidationError{Field: "function_name", Message: "cannot be empty"}, InvokeCodeInvalidArgument},
		{ErrWASMFetchTimeout, InvokeCodeUnavailable},
		{ErrNamespaceNotServed, InvokeCodeUnavailable},
		{fmt.Errorf("%w (max 64)", ErrAsyncInvokeSaturated), InvokeCodeUnavailable},
		{ErrDatabaseUnavailable, InvokeCodeUnavailable},
		{&ExecutionError{FunctionName: "f", Cause: errors.New("wasm trap")}, InvokeCodeInternal},
	} {
		if got := ClassifyInvokeError(tc.err); got != tc.want {
			t.Errorf("ClassifyInvokeError(%v) = %s, want %s", tc.err, got, tc.want)
		}
	}
}

func lastInvokeError(t *testing.T, ctx context.Context) *InvokeError {
	t.Helper()
	raw := LastInvokeErrorJSON(ctx)
	if raw == nil {
		return nil
	}
	var e InvokeError
	if err := json.Unmarshal(raw, &e); err != nil {
		t.Fatalf("last error is not JSON: %s", raw)
	}
	return &e
}

func TestInvokeErrorSlot_recordsAndClears(t *testing.T) {
	ctx := WithInvokeErrorSlot(context.Background())
	if lastInvokeError(t, ctx) != nil {
		t.Fatal("a fresh slot reports an error")
	}
	RecordInvokeOutcome(ctx, ErrUnauthorized)
	if e := lastInvokeError(t, ctx); e == nil || e.Code != InvokeCodeAuthForbidden || e.Error == "" {
		t.Fatalf("after a refused call: %+v", e)
	}
	RecordInvokeOutcome(ctx, nil)
	if e := lastInvokeError(t, ctx); e != nil {
		t.Fatalf("a successful call left the earlier error: %+v", e)
	}
}

// A new invocation, or a reused instance's next callback, gets a new slot and
// never sees the previous one's error.
func TestInvokeErrorSlot_isPerInvocation(t *testing.T) {
	first := WithInvokeErrorSlot(context.Background())
	RecordInvokeOutcome(first, ErrUnauthorized)
	next := WithInvokeErrorSlot(context.Background())
	if e := lastInvokeError(t, next); e != nil {
		t.Fatalf("a new invocation inherited %+v", e)
	}
	// A nested invocation attaches its own slot on its derived ctx; what it
	// records stays out of its caller's.
	nested := WithInvokeErrorSlot(next)
	RecordInvokeOutcome(nested, ErrFunctionNotFound)
	if e := lastInvokeError(t, next); e != nil {
		t.Fatalf("a nested invocation's error reached its caller: %+v", e)
	}
}

func TestInvokeErrorSlot_noSlotRecordsNothing(t *testing.T) {
	RecordInvokeOutcome(context.Background(), ErrUnauthorized)
	if LastInvokeErrorJSON(context.Background()) != nil {
		t.Fatal("a ctx with no slot reported an error")
	}
}

func TestInvokeErrorSlot_longErrorIsBounded(t *testing.T) {
	ctx := WithInvokeErrorSlot(context.Background())
	RecordInvokeOutcome(ctx, errors.New(strings.Repeat("x", 10*maxInvokeErrorBytes)))
	if e := lastInvokeError(t, ctx); len(e.Error) > maxInvokeErrorBytes+4 {
		t.Fatalf("error text of %d bytes handed to the guest", len(e.Error))
	}
}

// invokeServices answers function_invoke with a chosen error.
type invokeServices struct {
	*MockHostServices
	err error
	out []byte
}

func (s *invokeServices) FunctionInvoke(context.Context, string, []byte) ([]byte, error) {
	return s.out, s.err
}

func (s *invokeServices) FunctionInvokeAsync(context.Context, string, []byte) error {
	return s.err
}

// guestWithMalloc is a one-page memory and a malloc that always answers
// address 1024, enough for the host to hand a result back.
var guestWithMalloc = []byte{
	0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00, // magic, version
	0x01, 0x06, 0x01, 0x60, 0x01, 0x7f, 0x01, 0x7f, // type: (i32) -> i32
	0x03, 0x02, 0x01, 0x00, // function 0 has type 0
	0x05, 0x03, 0x01, 0x00, 0x01, // one memory of one page
	0x07, 0x13, 0x02, // two exports
	0x06, 'm', 'e', 'm', 'o', 'r', 'y', 0x02, 0x00,
	0x06, 'm', 'a', 'l', 'l', 'o', 'c', 0x00, 0x00,
	0x0a, 0x07, 0x01, 0x05, 0x00, 0x41, 0x80, 0x08, 0x0b, // malloc: i32.const 1024
}

func readPacked(t *testing.T, mod api.Module, packed uint64) string {
	t.Helper()
	if packed == 0 {
		return ""
	}
	b, ok := mod.Memory().Read(uint32(packed>>32), uint32(packed))
	if !ok {
		t.Fatal("packed result is outside guest memory")
	}
	return string(b)
}

func TestEngine_functionInvokeLastError(t *testing.T) {
	svc := &invokeServices{MockHostServices: NewMockHostServices()}
	e, err := NewEngine(nil, NewMockRegistry(), svc, zap.NewNop())
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	t.Cleanup(func() { _ = e.runtime.Close(context.Background()) })
	ctx := WithInvokeErrorSlot(context.Background())
	mod, err := e.runtime.Instantiate(ctx, guestWithMalloc)
	if err != nil {
		t.Fatal(err)
	}
	mod.Memory().Write(0, []byte("settle"))

	svc.err = &ExecutionError{FunctionName: "settle", Cause: fmt.Errorf("invoke: %w", ErrUnauthorized)}
	if got := e.hFunctionInvoke(ctx, mod, 0, 6, 0, 0); got != 0 {
		t.Fatalf("a refused invoke returned %d", got)
	}
	var last InvokeError
	if err := json.Unmarshal([]byte(readPacked(t, mod, e.hFunctionInvokeLastError(ctx, mod))), &last); err != nil || last.Code != InvokeCodeAuthForbidden {
		t.Fatalf("function_invoke_last_error after a refusal = %+v, %v; want AUTH_FORBIDDEN", last, err)
	}

	svc.err = nil
	e.hFunctionInvoke(ctx, mod, 0, 6, 0, 0)
	if got := e.hFunctionInvokeLastError(ctx, mod); got != 0 {
		t.Fatalf("after a successful invoke the last error is %q, want none", readPacked(t, mod, got))
	}

	// An argument outside guest memory is the guest's own mistake.
	e.hFunctionInvoke(ctx, mod, 1<<20, 6, 0, 0)
	if err := json.Unmarshal([]byte(readPacked(t, mod, e.hFunctionInvokeLastError(ctx, mod))), &last); err != nil || last.Code != InvokeCodeInvalidArgument {
		t.Fatalf("after an unreadable name = %+v, %v; want INVALID_ARGUMENT", last, err)
	}

	// function_invoke_async records whether the call was accepted.
	svc.err = fmt.Errorf("%w (max 64)", ErrAsyncInvokeSaturated)
	if got := e.hFunctionInvokeAsync(ctx, mod, 0, 6, 0, 0); got != 0 {
		t.Fatalf("a refused async invoke returned %d", got)
	}
	if err := json.Unmarshal([]byte(readPacked(t, mod, e.hFunctionInvokeLastError(ctx, mod))), &last); err != nil || last.Code != InvokeCodeUnavailable {
		t.Fatalf("after a saturated async invoke = %+v, %v; want UNAVAILABLE", last, err)
	}

	// Another invocation's context starts empty.
	if got := e.hFunctionInvokeLastError(WithInvokeErrorSlot(context.Background()), mod); got != 0 {
		t.Fatalf("a new invocation reads %q", readPacked(t, mod, got))
	}
}

func TestEngine_HostModule_FunctionInvokeLastErrorExported(t *testing.T) {
	e, err := NewEngine(nil, NewMockRegistry(), NewMockHostServices(), zap.NewNop())
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer func() { _ = e.runtime.Close(context.Background()) }()
	for _, name := range []string{"env", "host", "orama"} {
		def, ok := e.runtime.Module(name).ExportedFunctionDefinitions()["function_invoke_last_error"]
		if !ok {
			t.Fatalf("%s.function_invoke_last_error is not exported", name)
		}
		if len(def.ParamTypes()) != 0 || len(def.ResultTypes()) != 1 || def.ResultTypes()[0] != api.ValueTypeI64 {
			t.Errorf("%s.function_invoke_last_error = %v -> %v, want () -> i64", name, def.ParamTypes(), def.ResultTypes())
		}
	}
}
