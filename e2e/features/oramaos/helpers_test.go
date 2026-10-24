//go:build e2e_fleet

package oramaos

import (
	"net/http"
	"testing"

	"github.com/Orama-Network/orama/e2e/features/internal/infra"
	"github.com/Orama-Network/orama/e2e/features/internal/tenancy"
	"github.com/Orama-Network/orama/e2e/harness"
	"github.com/Orama-Network/orama/e2e/harness/fleet"
	"github.com/Orama-Network/orama/e2e/harness/gw"
	"github.com/Orama-Network/orama/e2e/harness/ns"
)

// scopeAdmin is the widest key a namespace can mint (core/pkg/gateway/auth/scopes.go).
const scopeAdmin = "admin"

// expectNotAnOperator records an error unless resp is the operator list's
// refusal, 403 NOT_AN_OPERATOR.
func expectNotAnOperator(t *testing.T, resp *gw.Response, what string) {
	t.Helper()
	if resp.Status != http.StatusForbidden || resp.ErrorCode() != tenancy.CodeNotOperator {
		t.Errorf("%s: HTTP %d %q, want 403 %s: %.300s", what, resp.Status, resp.ErrorCode(), tenancy.CodeNotOperator, resp.Body)
	}
}

// operatorNamespace is a fresh namespace whose owner is made an operator of
// the cluster for t (`orama maint operator add`, removed at t's cleanup).
func operatorNamespace(t *testing.T, f *fleet.Fleet) *ns.Namespace {
	t.Helper()
	n := tenancy.Namespace(t, f, ns.Options{})
	addr := n.Owner.Wallet.Address()
	cli := harness.CLI(t)
	infra.ExpectExit(t, infra.Run(t, cli, "maint", "operator", "add", addr), infra.ExitOK)
	t.Cleanup(func() {
		ctx, cancel := fleet.CleanupContext(t)
		defer cancel()
		if res, err := cli.Run(ctx, "maint", "operator", "remove", addr); err != nil || res.Exit != 0 {
			t.Errorf("cleanup: removing operator %s failed: %v %s", addr, err, res.Stderr)
		}
	})
	return n
}
