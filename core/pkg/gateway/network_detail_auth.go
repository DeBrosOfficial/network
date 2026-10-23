package gateway

import (
	"net/http"

	nodeauth "github.com/Orama-Network/orama/core/pkg/auth"
)

// authorizeNetworkDetail admits the callers the network status routes serve:
// another node over the mesh, stamped with the coordination MAC — its IPFS
// Cluster peer discovery (pkg/ipfs) — or an operator. It writes the refusal;
// false means the handler returns.
//
// The route policy (networkDetailPolicy) sends a stamped request here without a
// credential and every other request through the operator gate first, which
// has already checked the operator list; it is checked again here.
func (g *Gateway) authorizeNetworkDetail(w http.ResponseWriter, r *http.Request) bool {
	if nodeauth.HasCoordinationStamp(r) {
		if g.verifyCoordination(r) {
			return true
		}
		http.Error(w, "not found", http.StatusNotFound)
		return false
	}
	if g.operatorHandler == nil {
		writeError(w, http.StatusServiceUnavailable, "this gateway cannot check the operator list")
		return false
	}
	_, ok := g.operatorHandler.Authorize(w, r)
	return ok
}
