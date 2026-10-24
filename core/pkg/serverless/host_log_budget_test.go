package serverless

import (
	"context"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// bug-424: a guest looping on a failing host call could write a log line per
// call until its timeout. One invocation's host calls write at most
// MaxHostLogLinesPerInvocation lines, then one summary line.

func TestHostLogger_budgetThenSummary(t *testing.T) {
	core, logs := observer.New(zapcore.InfoLevel)
	base := zap.New(core)
	ctx, budget := WithHostLogBudget(context.Background())

	const calls = 1000
	for i := 0; i < calls; i++ {
		HostLogger(ctx, base).Error("host function cache_get failed")
	}
	if got := logs.Len(); got != MaxHostLogLinesPerInvocation {
		t.Fatalf("%d failing calls wrote %d lines, want %d", calls, got, MaxHostLogLinesPerInvocation)
	}
	budget.Summarize(base, zap.String("function", "loop"))
	all := logs.All()
	if len(all) != MaxHostLogLinesPerInvocation+1 {
		t.Fatalf("Summarize wrote %d lines, want one", len(all)-MaxHostLogLinesPerInvocation)
	}
	summary := all[len(all)-1].ContextMap()
	if summary["suppressed"] != int64(calls-MaxHostLogLinesPerInvocation) || summary["function"] != "loop" {
		t.Fatalf("summary = %v, want suppressed=%d and the function named", summary, calls-MaxHostLogLinesPerInvocation)
	}
}

func TestHostLogger_noSummaryWithinBudget(t *testing.T) {
	core, logs := observer.New(zapcore.InfoLevel)
	base := zap.New(core)
	ctx, budget := WithHostLogBudget(context.Background())
	HostLogger(ctx, base).Warn("one failure")
	budget.Summarize(base)
	if logs.Len() != 1 {
		t.Fatalf("an invocation within its budget wrote %d lines, want just its own", logs.Len())
	}
}

// Each invocation has its own budget: one that spent it leaves the next whole.
func TestHostLogger_budgetIsPerInvocation(t *testing.T) {
	core, logs := observer.New(zapcore.InfoLevel)
	base := zap.New(core)
	spent, _ := WithHostLogBudget(context.Background())
	for i := 0; i < 2*MaxHostLogLinesPerInvocation; i++ {
		HostLogger(spent, base).Error("x")
	}
	fresh, _ := WithHostLogBudget(context.Background())
	HostLogger(fresh, base).Error("next invocation")
	if last := logs.All()[logs.Len()-1]; last.Message != "next invocation" {
		t.Fatalf("a new invocation's line was dropped by another's budget")
	}
}

// A line the logger would not write anyway does not spend the budget.
func TestHostLogger_disabledLevelsSpendNothing(t *testing.T) {
	core, logs := observer.New(zapcore.WarnLevel)
	base := zap.New(core)
	ctx, budget := WithHostLogBudget(context.Background())
	for i := 0; i < 10*MaxHostLogLinesPerInvocation; i++ {
		HostLogger(ctx, base).Info("guest log_info echo below the level")
	}
	HostLogger(ctx, base).Error("a real failure")
	if logs.Len() != 1 || budget.Suppressed() != 0 {
		t.Fatalf("lines below the level spent the budget: %d written, %d suppressed", logs.Len(), budget.Suppressed())
	}
}

// Outside an invocation (no budget on ctx) the logger is unchanged.
func TestHostLogger_withoutBudgetIsUnlimited(t *testing.T) {
	core, logs := observer.New(zapcore.InfoLevel)
	base := zap.New(core)
	for i := 0; i < 2*MaxHostLogLinesPerInvocation; i++ {
		HostLogger(context.Background(), base).Error("x")
	}
	if logs.Len() != 2*MaxHostLogLinesPerInvocation {
		t.Fatalf("wrote %d lines without a budget, want all", logs.Len())
	}
	if HostLogger(context.Background(), nil) != nil {
		t.Fatal("a nil logger must stay nil")
	}
}

// The engine's host functions log through the budget: a guest looping on a
// failing http_fetch writes the budget and no more.
func TestEngineHostFunctions_failureLogsAreBudgeted(t *testing.T) {
	e, _ := engineWithObserver(t)
	core, logs := observer.New(zapcore.DebugLevel)
	e.logger = zap.New(core)
	ctx, budget := WithHostLogBudget(context.Background())
	mod, err := e.runtime.Instantiate(ctx, guestMemory)
	if err != nil {
		t.Fatal(err)
	}
	mod.Memory().Write(0, []byte("GET"))
	mod.Memory().Write(16, []byte(engineFetchURL))

	for i := 0; i < 500; i++ {
		e.hHTTPFetch(ctx, mod, 0, 3, 16, uint32(len(engineFetchURL)), 0, 0, 0, 0)
	}
	if logs.Len() != MaxHostLogLinesPerInvocation {
		t.Fatalf("500 failing http_fetch calls wrote %d lines, want %d", logs.Len(), MaxHostLogLinesPerInvocation)
	}
	if budget.Suppressed() != 500-MaxHostLogLinesPerInvocation {
		t.Fatalf("suppressed = %d, want %d", budget.Suppressed(), 500-MaxHostLogLinesPerInvocation)
	}
}
