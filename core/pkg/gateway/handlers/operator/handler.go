// Package operator provides HTTP handlers for node operator management.
//
// Operators authenticate via wallet JWT (same auth flow as namespaces).
// Each operator's nodes are tracked by their wallet address in the
// dns_nodes and wireguard_peers tables.
package operator

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/Orama-Network/orama/core/pkg/gateway/auth"
	"github.com/Orama-Network/orama/core/pkg/gateway/ctxkeys"
	"github.com/Orama-Network/orama/core/pkg/rqlite"
	"go.uber.org/zap"
)

// Handler provides HTTP handlers for operator node management.
type Handler struct {
	logger       *zap.Logger
	rqliteClient rqlite.Client

	// audit records what an operator did. An operator action is the highest
	// authority anything on this cluster has, and none of it was recorded.
	audit *auth.AuditLog
}

// SetAuditLog wires the record. Set by the gateway after construction; nil
// leaves the actions unrecorded, which is the test case.
func (h *Handler) SetAuditLog(a *auth.AuditLog) { h.audit = a }

// NewHandler creates an operator handler.
func NewHandler(logger *zap.Logger, rqliteClient rqlite.Client) *Handler {
	return &Handler{
		logger:       logger,
		rqliteClient: rqliteClient,
	}
}

// walletFromRequest is the operator's wallet; see WalletFromRequest.
func (h *Handler) walletFromRequest(r *http.Request) string {
	return WalletFromRequest(r)
}

// WalletFromRequest is the wallet a request may act as an operator for: the
// subject of a wallet session, or "" for any other credential.
//
// An API key, or a token exchanged for one, used to resolve to the wallet that
// owns the key's namespace. With the default creation policy every namespace
// is owned by an operator, so any namespace admin could mint a key and be an
// operator through it: mint a cluster invite, rotate the cluster's secrets,
// add operators, export the registry. A key is a namespace's credential, and
// whoever holds one holds only what the namespace grants it.
//
// Exported for the gateway, which asks the same question in the scope gate and
// before letting a caller read the cluster registry.
func WalletFromRequest(r *http.Request) string {
	claims, ok := r.Context().Value(ctxkeys.JWT).(*auth.JWTClaims)
	if !ok || claims == nil {
		return ""
	}
	sub := strings.TrimSpace(claims.Sub)
	if !auth.IsWalletSubject(sub) {
		return ""
	}
	return sub
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
