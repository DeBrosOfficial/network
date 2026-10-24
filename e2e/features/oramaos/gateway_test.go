//go:build e2e_fleet

package oramaos

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/Orama-Network/orama/e2e/features/internal/tenancy"
	"github.com/Orama-Network/orama/e2e/harness"
	"github.com/Orama-Network/orama/e2e/harness/gw"
	"github.com/Orama-Network/orama/e2e/harness/ns"
)

const pathEnroll = "/v1/node/enroll"

// enrollBody is `orama maint node enroll`'s request (core/pkg/gateway/handlers/enroll
// EnrollRequest), with a token no invite ever had.
func enrollBody(t *testing.T, nodeIP string) []byte {
	t.Helper()
	raw, err := json.Marshal(map[string]string{"code": strings.Repeat("ab", codeHex/2), "token": "e2e-not-an-invite", "node_ip": nodeIP})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// TestOramaOSEnroll_nodeIPMustBePublicIPv4: the gateway stores node_ip as
// every other node's WireGuard Endpoint and pushes the configuration to it,
// so anything but a public IPv4 is refused (400) before the invite is looked
// at; a public address with a token no invite has is 401 and pushes nothing
// (website/src/docs/operator/orama-os.mdx "Step 3"; core/pkg/gateway/handlers/enroll
// handler.go, nodeip.go).
func TestOramaOSEnroll_nodeIPMustBePublicIPv4(t *testing.T) {
	t.Parallel()
	c := harness.GW(t)
	for _, ip := range []string{"127.0.0.1", "10.0.0.9", "192.168.1.1", "172.16.0.1", "100.64.0.1", "169.254.169.254", "0.0.0.0", "224.0.0.1", "::1", "2001:db8::1", "1.2.3.4\nEndpoint = 6.6.6.6", "not-an-ip"} {
		r := c.MustSend(t, gw.Req{Method: http.MethodPost, Path: pathEnroll, Header: http.Header{"Content-Type": {"application/json"}}, Body: enrollBody(t, ip)})
		if r.Status != http.StatusBadRequest || !strings.Contains(string(r.Body), "node_ip") {
			t.Errorf("node_ip %q: HTTP %d %.200s, want 400 naming node_ip", ip, r.Status, r.Body)
		}
	}
	r := c.MustSend(t, gw.Req{Method: http.MethodPost, Path: pathEnroll, Header: http.Header{"Content-Type": {"application/json"}}, Body: enrollBody(t, "203.0.113.7")})
	if r.Status != http.StatusUnauthorized {
		t.Errorf("a public node_ip with no invite: HTTP %d %.200s, want 401", r.Status, r.Body)
	}
	for name, body := range map[string][]byte{"empty": []byte(`{}`), "not json": []byte("{"), "no token": []byte(`{"code":"x","node_ip":"203.0.113.7"}`)} {
		if r := c.MustSend(t, gw.Req{Method: http.MethodPost, Path: pathEnroll, Body: body}); r.Status != http.StatusBadRequest {
			t.Errorf("%s: HTTP %d, want 400", name, r.Status)
		}
	}
}

// nodeRouteTarget is an overlay address no node holds: were a refusal
// missing, a leave or a command would still reach nothing of the fleet.
const nodeRouteTarget = "10.0.0.250"

// nodeRouteRequests are status, logs, command and leave aimed at
// nodeRouteTarget.
func nodeRouteRequests() []gw.Req {
	return []gw.Req{
		{Path: "/v1/node/status", Query: map[string][]string{"wg_ip": {nodeRouteTarget}}},
		{Path: "/v1/node/logs", Query: map[string][]string{"wg_ip": {nodeRouteTarget}, "service": {"gateway"}}},
		{Method: http.MethodPost, Path: "/v1/node/command", Query: map[string][]string{"wg_ip": {nodeRouteTarget}}, Body: []byte(`{"action":"restart","service":"rqlite"}`)},
		{Method: http.MethodPost, Path: "/v1/node/leave", Body: []byte(`{"wg_ip":"` + nodeRouteTarget + `"}`)},
	}
}

// sendNodeRoute sends req with who's credential.
func sendNodeRoute(t *testing.T, c *gw.Client, req gw.Req, who tenancy.Cred) *gw.Response {
	t.Helper()
	req.Bearer, req.APIKey = who.Bearer, who.APIKey
	if req.Body != nil {
		req.Header = http.Header{"Content-Type": {"application/json"}}
	}
	return c.MustSend(t, req)
}

// TestOramaOSNodeRoutes_refuseNonOperators: status, logs, command and leave
// reach a node's agent and can stop it or drop it from the mesh; with no
// credential, as a signed-in wallet that is not an operator, as a namespace's
// owner or with its admin key, each is refused before anything is proxied
// (website/src/docs/operator/orama-os.mdx "Node Management";
// core/pkg/gateway/route_policy.go operatorRoute). The owner and the admin key
// hold `*:*:*` in their namespace, which reached these routes until bug-55;
// they are refused by the operator list, 403 NOT_AN_OPERATOR.
func TestOramaOSNodeRoutes_refuseNonOperators(t *testing.T) {
	t.Parallel()
	f := harness.Fleet(t)
	c := harness.GW(t)
	wallet := gw.NewUser(t, f, gw.LobbyNamespace)
	n := tenancy.Namespace(t, f, ns.Options{})
	tenants := map[string]tenancy.Cred{
		"a namespace owner": tenancy.Owner(n),
		"an admin API key":  {APIKey: tenancy.APIKey(t, n, scopeAdmin)},
	}
	for _, req := range nodeRouteRequests() {
		for who, cred := range map[string]tenancy.Cred{"nobody": {}, "a wallet": {Bearer: wallet.Token()}} {
			tenancy.ExpectDenied(t, sendNodeRoute(t, c, req, cred), who+" at "+req.Path)
		}
		for who, cred := range tenants {
			expectNotAnOperator(t, sendNodeRoute(t, c, req, cred), who+" at "+req.Path)
		}
	}
}

// TestOramaOSNodeRoutes_operatorListIsTheOnlyAuthority: a namespace whose
// owner is on the operator list still hands out no operator credential: its
// admin key is refused NOT_AN_OPERATOR on every node route, while the owner's
// own session passes the gate (a status naming no node is the handler's 400,
// so nothing is proxied, and a leave of an address no agent answers is 502
// NODE_STOP_UNCONFIRMED, so nothing is removed).
func TestOramaOSNodeRoutes_operatorListIsTheOnlyAuthority(t *testing.T) {
	t.Parallel()
	f := harness.Fleet(t)
	c := harness.GW(t)
	op := operatorNamespace(t, f)
	key := tenancy.Cred{APIKey: tenancy.APIKey(t, op, scopeAdmin)}
	for _, req := range nodeRouteRequests() {
		expectNotAnOperator(t, sendNodeRoute(t, c, req, key), "an operator namespace's admin key at "+req.Path)
	}
	r := sendNodeRoute(t, c, gw.Req{Path: "/v1/node/status"}, tenancy.Owner(op))
	if r.Status != http.StatusBadRequest {
		t.Errorf("an operator's session at /v1/node/status naming no node: HTTP %d %.200s, want the handler's 400", r.Status, r.Body)
	}
	// bug-720: a leave whose node does not confirm the stop (no agent answers
	// at nodeRouteTarget) removes nothing and says so.
	leave := nodeRouteRequests()[3]
	r = sendNodeRoute(t, c, leave, tenancy.Owner(op))
	if r.Status != http.StatusBadGateway || r.ErrorCode() != codeNodeStopUnconfirmed {
		t.Errorf("an operator's leave of a node with no agent: HTTP %d %q %.300s, want 502 %s", r.Status, r.ErrorCode(), r.Body, codeNodeStopUnconfirmed)
	}
}

// codeNodeStopUnconfirmed is the code POST /v1/node/leave answers when the
// node's agent did not confirm that it stopped (core/pkg/gateway/handlers/enroll
// ErrCodeNodeStopUnconfirmed).
const codeNodeStopUnconfirmed = "NODE_STOP_UNCONFIRMED"

// TestOramaOS_luksShamirUnlockAndABUpdates records what a single VM cannot
// reach, so the report shows it uncovered rather than silently absent: the
// Shamir unlock needs K = max(2, floor(N/3)) peer vault-guardians on the
// overlay, and enrollment of a node with no peers fails ("no peers available
// for key distribution", website/src/docs/operator/orama-os.mdx "Genesis Node"); an A/B
// update can be pointed at a test server (ORAMA_UPDATE_URL baked into the
// image, os/agent/internal/update ResolveUpdateURL), but the agent installs
// only a release signed by the embedded RootWallet SignerAddress, which the
// harness does not hold.
func TestOramaOS_luksShamirUnlockAndABUpdates(t *testing.T) {
	harness.Fleet(t)
	harness.SkipNotApplicable(t, "needs an enrolled OramaOS cluster of K+ nodes (Shamir unlock) and a release signed by the OramaOS SignerAddress key (A/B signature and 3-boot rollback); the harness has neither")
}
