//go:build e2e_fleet

package authclusteradmin

import (
	"net/http"
	"testing"
	"time"

	"github.com/Orama-Network/orama/e2e/harness"
	"github.com/Orama-Network/orama/e2e/harness/eventually"
	"github.com/Orama-Network/orama/e2e/harness/gw"
	"github.com/Orama-Network/orama/e2e/harness/ns"
)

const (
	pathOperatorAudit = "/v1/operator/audit"
	// auditOperatorRemove is the action an operator's removal of a namespace
	// is recorded under, at cluster level (core/pkg/gateway/auth/audit.go).
	auditOperatorRemove = "namespace.operator_remove"
	// auditVisibleBudget bounds the wait for the event to reach the node the
	// read lands on.
	auditVisibleBudget = 30 * time.Second
	auditPollEvery     = 2 * time.Second
)

// TestClusterAudit_operatorReadsWhatNoNamespaceCan: a namespace's removal is
// recorded under no namespace, so neither its owner's trail nor anyone's
// /v1/audit shows it. An operator reads it with `orama audit --cluster`; the
// owner is refused the operator route (NOT_AN_OPERATOR)
// (docs/whitepaper/technical-reference/appendices/i-api-surface.md "/v1/operator/audit",
// docs/whitepaper/technical-reference/appendices/d-cli-reference.md "orama audit").
func TestClusterAudit_operatorReadsWhatNoNamespaceCan(t *testing.T) {
	f := harness.Fleet(t)
	n := ns.New(t, f, ns.Options{})
	owner := n.Owner.Token()

	resp := n.Owner.Client.MustSend(t, gw.Req{Method: http.MethodGet, Path: pathOperatorAudit, Bearer: owner})
	if resp.Status != http.StatusForbidden || resp.ErrorCode() != "NOT_AN_OPERATOR" {
		t.Fatalf("the owner read the cluster's audit trail: want 403 NOT_AN_OPERATOR, got %d %s", resp.Status, resp.Body)
	}

	cli := harness.CLI(t)
	cli.MustOK(t, "cluster", "namespace", "remove", n.Name, "--reason", "e2e: cluster audit", "--force")
	n.MarkRemoved(t)

	err := eventually.Poll(t.Context(), auditPollEvery, auditVisibleBudget, "the removal in the cluster's audit trail", func() (bool, error) {
		out := cli.MustOK(t, "audit", "--cluster", "--action", auditOperatorRemove, "--limit", "200").Stdout
		return containsFold(out, n.Name), nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
