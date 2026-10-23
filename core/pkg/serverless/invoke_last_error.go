package serverless

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
)

// Codes function_invoke_last_error reports, so a guest can tell a refusal no
// retry will fix from a failure worth retrying (bugboard #263). The names are
// the batched database envelope's where the meaning is the same
// (rqlite.BatchCode*), so a guest switches on one vocabulary.
const (
	// InvokeCodeAuthForbidden: the caller may not invoke the target. A
	// misconfiguration; retrying never helps.
	InvokeCodeAuthForbidden = "AUTH_FORBIDDEN"
	// InvokeCodeNotFound: no such function, or not its current version.
	InvokeCodeNotFound = "NOT_FOUND"
	// InvokeCodeRateLimited: refused by a rate limit before the target ran.
	// Retrying later is safe.
	InvokeCodeRateLimited = "RATE_LIMITED"
	// InvokeCodeUnavailable: the target could not be loaded or the gateway
	// could not accept the call right now. Retrying is safe: it did not run.
	InvokeCodeUnavailable = "UNAVAILABLE"
	// InvokeCodeDeadlineExceeded: the target ran and was stopped at its
	// deadline. What it did before that may have happened.
	InvokeCodeDeadlineExceeded = "DEADLINE_EXCEEDED"
	// InvokeCodeInvalidArgument: the call itself was malformed (its name or
	// payload could not be read from guest memory, or the payload is too
	// large). Retrying the same call never helps.
	InvokeCodeInvalidArgument = "INVALID_ARGUMENT"
	// InvokeCodeInternal: anything else, the target's own failure included;
	// read the error.
	InvokeCodeInternal = "INTERNAL"
)

// maxInvokeErrorBytes bounds the error text handed back to the guest.
const maxInvokeErrorBytes = 1024

// InvokeError is what function_invoke_last_error returns, as JSON.
type InvokeError struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

// ClassifyInvokeError maps a nested invocation's error onto an InvokeCode*.
// The order matters: a timeout is checked before resource exhaustion because
// the target ran.
func ClassifyInvokeError(err error) string {
	var invalid *ValidationError
	switch {
	case IsUnauthorized(err):
		return InvokeCodeAuthForbidden
	case IsNotFound(err):
		return InvokeCodeNotFound
	case IsTimeout(err):
		return InvokeCodeDeadlineExceeded
	case errors.Is(err, ErrPayloadTooLarge), errors.Is(err, ErrInvalidInput), errors.As(err, &invalid):
		return InvokeCodeInvalidArgument
	case errors.Is(err, ErrRateLimited):
		return InvokeCodeRateLimited
	case IsServiceUnavailable(err), errors.Is(err, ErrWASMFetchTimeout),
		errors.Is(err, ErrNamespaceNotServed), errors.Is(err, ErrQueueFull),
		errors.Is(err, ErrAsyncInvokeSaturated):
		return InvokeCodeUnavailable
	}
	return InvokeCodeInternal
}

// invokeErrorSlot holds the outcome of the invocation's last function_invoke
// or function_invoke_async call. It rides the invocation's context, so it is
// new for every invocation and every persistent-socket callback: a reactor or
// persistent instance that is reused never hands one invocation's error to the
// next, which would make a guest blame a call that succeeded.
type invokeErrorSlot struct {
	mu   sync.Mutex
	last *InvokeError
}

type invokeErrorSlotKey struct{}

// WithInvokeErrorSlot returns a derived ctx carrying an empty slot.
func WithInvokeErrorSlot(ctx context.Context) context.Context {
	return context.WithValue(ctx, invokeErrorSlotKey{}, &invokeErrorSlot{})
}

// RecordInvokeOutcome stores the outcome of a nested invocation: nil clears
// the slot, an error replaces it with the error classified. A ctx with no slot
// (outside an invocation) records nothing.
func RecordInvokeOutcome(ctx context.Context, err error) {
	if err == nil {
		recordInvokeError(ctx, nil)
		return
	}
	recordInvokeError(ctx, &InvokeError{Error: boundedErrorText(err.Error()), Code: ClassifyInvokeError(err)})
}

// RecordInvokeArgumentError stores a call the host could not even make
// because its arguments could not be read.
func RecordInvokeArgumentError(ctx context.Context, msg string) {
	recordInvokeError(ctx, &InvokeError{Error: msg, Code: InvokeCodeInvalidArgument})
}

func recordInvokeError(ctx context.Context, e *InvokeError) {
	if ctx == nil {
		return
	}
	s, ok := ctx.Value(invokeErrorSlotKey{}).(*invokeErrorSlot)
	if !ok {
		return
	}
	s.mu.Lock()
	s.last = e
	s.mu.Unlock()
}

// LastInvokeErrorJSON returns the slot's error as JSON, or nil when the last
// nested call succeeded, none was made, or ctx carries no slot.
func LastInvokeErrorJSON(ctx context.Context) []byte {
	if ctx == nil {
		return nil
	}
	s, ok := ctx.Value(invokeErrorSlotKey{}).(*invokeErrorSlot)
	if !ok {
		return nil
	}
	s.mu.Lock()
	last := s.last
	s.mu.Unlock()
	if last == nil {
		return nil
	}
	out, err := json.Marshal(last)
	if err != nil {
		return nil
	}
	return out
}

func boundedErrorText(s string) string {
	if len(s) <= maxInvokeErrorBytes {
		return s
	}
	return strings.ToValidUTF8(s[:maxInvokeErrorBytes], "") + "…"
}
