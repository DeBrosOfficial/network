package serverless

import (
	"context"
	"sync/atomic"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// MaxHostLogLinesPerInvocation is how many gateway log lines one invocation's
// host calls may write: host-function failures and the gateway's copy of the
// function's own log_info / log_error lines. A failing host call can cost the
// guest almost nothing (a refused ttl, no cache configured) and there is no
// fuel metering, so a guest looping on one until its timeout (30-300 s) could
// otherwise write millions of lines (bugboard #424). Past the budget the lines
// are counted, not written, and one summary line names how many were dropped.
// The function's own log lines still reach its invocation record through the
// LogBuffer, which has its own cap.
const MaxHostLogLinesPerInvocation = 32

// HostLogBudget counts the gateway log lines one invocation's host calls
// write. It rides the invocation's context, like the publish counter, so
// concurrent invocations each have their own.
type HostLogBudget struct{ lines atomic.Int64 }

type hostLogBudgetKey struct{}

// WithHostLogBudget returns a derived ctx carrying a fresh budget, and the
// budget, whose Summarize the caller runs when the invocation ends.
func WithHostLogBudget(ctx context.Context) (context.Context, *HostLogBudget) {
	b := &HostLogBudget{}
	return context.WithValue(ctx, hostLogBudgetKey{}, b), b
}

// HostLogger returns l limited by the invocation budget ctx carries, or l
// itself when ctx carries none (a call outside an invocation, such as a
// reactor module's _initialize). Every host-function log line goes through it.
func HostLogger(ctx context.Context, l *zap.Logger) *zap.Logger {
	if ctx == nil || l == nil {
		return l
	}
	b, ok := ctx.Value(hostLogBudgetKey{}).(*HostLogBudget)
	if !ok {
		return l
	}
	return l.WithOptions(zap.WrapCore(func(c zapcore.Core) zapcore.Core {
		return budgetCore{Core: c, budget: b}
	}))
}

// Suppressed is how many lines past the budget were not written.
func (b *HostLogBudget) Suppressed() int64 {
	return max(b.lines.Load()-MaxHostLogLinesPerInvocation, 0)
}

// Summarize writes one line saying how many lines the budget dropped, when it
// dropped any. fields identify the invocation.
func (b *HostLogBudget) Summarize(l *zap.Logger, fields ...zap.Field) {
	n := b.Suppressed()
	if n == 0 {
		return
	}
	l.Warn("host log lines suppressed for this invocation",
		append(fields, zap.Int64("suppressed", n), zap.Int("budget", MaxHostLogLinesPerInvocation))...)
}

// budgetCore admits a line to the wrapped core only while the budget lasts.
// Lines the wrapped core would not write anyway (below its level) do not
// spend it.
type budgetCore struct {
	zapcore.Core
	budget *HostLogBudget
}

func (c budgetCore) With(fields []zapcore.Field) zapcore.Core {
	return budgetCore{Core: c.Core.With(fields), budget: c.budget}
}

func (c budgetCore) Check(ent zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if !c.Core.Enabled(ent.Level) {
		return ce
	}
	if c.budget.lines.Add(1) > MaxHostLogLinesPerInvocation {
		return ce
	}
	return c.Core.Check(ent, ce)
}
