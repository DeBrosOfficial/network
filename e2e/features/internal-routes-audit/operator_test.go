//go:build e2e_fleet

package internalroutesaudit

import (
	"net/http"
	"testing"

	"github.com/Orama-Network/orama/e2e/features/internal/tenancy"
	"github.com/Orama-Network/orama/e2e/harness"
	"github.com/Orama-Network/orama/e2e/harness/gw"
	"github.com/Orama-Network/orama/e2e/harness/ns"
)

// pathOperatorRegister records a node in the operator's inventory
// (docs/whitepaper/technical-reference/appendices/i-api-surface.md "Node and operator").
const pathOperatorRegister = "/v1/operator/node/register"

// refusal is a credential and the answer it is owed.
type refusal struct {
	what   string
	cred   tenancy.Cred
	status int
	code   string
}

// operatorRefusals are the callers who are not operators of the cluster:
// nobody (401 AUTH_MISSING); the namespace's owner, its admin key, a runtime
// member and a lobby session, none of them on the operator list (403
// NOT_AN_OPERATOR, the scope gate in operator_gate.go): no permission reaches
// an operator route.
func operatorRefusals(t *testing.T, n *ns.Namespace) []refusal {
	t.Helper()
	lobby := gw.NewUser(t, harness.Fleet(t), gw.LobbyNamespace)
	return []refusal{
		{"no credential", tenancy.Cred{}, http.StatusUnauthorized, tenancy.CodeMissing},
		{"namespace owner", tenancy.Owner(n), http.StatusForbidden, tenancy.CodeNotOperator},
		{"admin API key", tenancy.Cred{APIKey: tenancy.APIKey(t, n, scopeAdmin)}, http.StatusForbidden, tenancy.CodeNotOperator},
		{"runtime member", tenancy.Cred{Bearer: tenancy.Member(t, n, tenancy.RoleRuntime).Token()}, http.StatusForbidden, tenancy.CodeNotOperator},
		{"lobby session", tenancy.Cred{Bearer: lobby.Token()}, http.StatusForbidden, tenancy.CodeNotOperator},
	}
}

// TestOperatorNodeRegister_nonOperatorsRefused: recording a node in the
// inventory needs a wallet session on the operator list; a
// plain namespace admin — which any fresh wallet is, of its own namespace —
// is refused before the body is read, with or without spoofed headers. The
// body names no node, so even a missing gate would answer 400 and write
// nothing.
func TestOperatorNodeRegister_nonOperatorsRefused(t *testing.T) {
	t.Parallel()
	f := harness.Fleet(t)
	n := tenancy.Namespace(t, f, ns.Options{})
	c := harness.GW(t)
	r := route{path: pathOperatorRegister, method: http.MethodPost, body: `{}`}
	for _, ref := range operatorRefusals(t, n) {
		for what, extra := range map[string]http.Header{"plain": nil, "spoofed headers": spoofed(t, n.Name)} {
			resp := send(t, c, r, ref.cred, extra)
			if resp.Status != ref.status || resp.ErrorCode() != ref.code {
				t.Errorf("%s (%s) on %s: HTTP %d %q, want %d %s: %.300s",
					ref.what, what, pathOperatorRegister, resp.Status, resp.ErrorCode(), ref.status, ref.code, resp.Body)
			}
		}
	}
	requireConverged(t)
}
