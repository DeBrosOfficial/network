//go:build e2e_fleet

package serverless

import (
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Orama-Network/orama/e2e/harness/gw"
)

// Invoke rate limits a function declares for itself (function.yaml rate_limit,
// bug-740 / feat-938), and the bucket a caller with no wallet is charged to
// (bug-934). Each gateway keeps its own buckets and a namespace runs on up to
// every node of the fleet, so a rule of n requests is refused at the latest on
// call n*gateways+1 — that is the bound each loop below sends.

// gatewayBound is how many gateways a namespace's calls may spread over.
func gatewayBound(fx *fixture) int { return len(fx.f.State.Nodes) }

// untilRefused sends call until it is answered 429, at most attempts times,
// and returns that answer, failing the test if every call passed.
func untilRefused(t *testing.T, attempts int, call func(i int) *gw.Response) *gw.Response {
	t.Helper()
	for i := 0; i < attempts; i++ {
		r := call(i)
		switch r.Status {
		case http.StatusOK:
		case http.StatusTooManyRequests:
			return r
		default:
			t.Fatalf("call %d: want 200 or 429, got %d %.300s", i+1, r.Status, r.Body)
		}
	}
	t.Fatalf("%d calls all passed a limit that should have refused one", attempts)
	return nil
}

// checkRefusal asserts a function-limit refusal: RATE_LIMITED, retryable, the
// scope in the message and a Retry-After no longer than one refill.
func checkRefusal(t *testing.T, r *gw.Response, scope string, maxWait time.Duration) {
	t.Helper()
	if rpcCode(r) != codeRateLimited || !rpcRetryable(r) {
		t.Errorf("refusal: want %s retryable, got %.300s", codeRateLimited, r.Body)
	}
	if !strings.Contains(string(r.Body), "scope="+scope) {
		t.Errorf("refusal does not name scope=%s: %.300s", scope, r.Body)
	}
	secs, err := strconv.ParseFloat(r.Header.Get("Retry-After"), 64)
	if err != nil || secs <= 0 || time.Duration(secs*float64(time.Second)) > maxWait {
		t.Errorf("Retry-After %q: want a positive wait of at most %s", r.Header.Get("Retry-After"), maxWait)
	}
}

// TestInvoke_functionRateLimitPerIP: per_ip is charged to the client address
// whoever signed in, so alternating wallets does not escape it — the signup
// case, two an hour per address (website/src/docs/developer/functions.mdx#invoke-rate-limits).
func TestInvoke_functionRateLimitPerIP(t *testing.T) {
	t.Parallel()
	fx := setup(t)
	const requests = 2
	deploy(t, fx, fnSpec{name: "e2e-signup", yaml: "rate_limit:\n  per_ip:\n    requests: 2\n    window: 1h\n"})
	wallets := []string{fx.admin, fx.runtime}
	for i := 0; i < requests; i++ {
		if r := invoke(t, fx.c, "e2e-signup", wallets[i%len(wallets)], map[string]any{"op": "echo"}); r.Status != http.StatusOK {
			t.Fatalf("signup %d of %d from the address: %d %.300s", i+1, requests, r.Status, r.Body)
		}
	}
	r := untilRefused(t, requests*gatewayBound(fx)+1, func(i int) *gw.Response {
		return invoke(t, fx.c, "e2e-signup", wallets[i%len(wallets)], map[string]any{"op": "echo"})
	})
	// Two an hour refill one every thirty minutes.
	checkRefusal(t, r, "per_function_ip", 30*time.Minute)
}

// TestInvoke_functionRateLimitPerCallerByKey: per_caller buckets a caller with
// no wallet by its own key, not by the namespace every key of it belongs to.
// One key spending its budget leaves another key and a wallet theirs (bug-934).
func TestInvoke_functionRateLimitPerCallerByKey(t *testing.T) {
	t.Parallel()
	fx := setup(t)
	deploy(t, fx, fnSpec{name: "e2e-keyed", yaml: "rate_limit:\n  per_caller:\n    requests: 1\n    window: 1h\n"})
	heavy, other := mintKey(t, fx, "invoke-only"), mintKey(t, fx, "invoke-only")
	r := untilRefused(t, gatewayBound(fx)+1, func(int) *gw.Response { return invokeKey(t, fx, "e2e-keyed", heavy) })
	checkRefusal(t, r, "per_function_caller", time.Hour)
	if r := invokeKey(t, fx, "e2e-keyed", other); r.Status != http.StatusOK {
		t.Errorf("a second key was refused after the first spent its budget: %d %.300s", r.Status, r.Body)
	}
	if r := invoke(t, fx.c, "e2e-keyed", fx.admin, map[string]any{"op": "echo"}); r.Status != http.StatusOK {
		t.Errorf("a wallet was refused after a key spent its budget: %d %.300s", r.Status, r.Body)
	}
}

// TestDeploy_refusesARateLimitOutOfRange: the gateway checks the rule's ranges
// at deploy and names the field.
func TestDeploy_refusesARateLimitOutOfRange(t *testing.T) {
	t.Parallel()
	fx := setup(t)
	dir := writeFixture(t, fnSpec{name: "e2e-badlimit", yaml: "rate_limit:\n  per_ip:\n    requests: 2\n    window: 48h\n"})
	res, err := fx.n.CLI.For(t).Run(t.Context(), "function", "deploy", dir)
	if err != nil {
		t.Fatal(err)
	}
	if res.Exit == 0 {
		t.Cleanup(func() { deleteFn(t, fx.n.CLI, "e2e-badlimit") })
	}
	if res.Exit == 0 || !strings.Contains(res.Stdout+res.Stderr, "rate_limit.per_ip.window") {
		t.Errorf("a 48h window deployed (exit %d): %s %s", res.Exit, res.Stdout, res.Stderr)
	}
}
