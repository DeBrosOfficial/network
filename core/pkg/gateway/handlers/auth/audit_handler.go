package auth

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	authsvc "github.com/Orama-Network/orama/core/pkg/gateway/auth"
	"go.uber.org/zap"
)

// A record nobody can read is not a record. The events are written to a
// Raft-replicated table that, until this endpoint, only a raw-database query
// could reach — which needs the admin grant on a route that serves the whole
// database, so "who minted this key" was not a question a namespace owner
// could ask without being handed something far larger.
//
// GET /v1/audit returns this namespace's own events, most recent first, and
// GET /v1/operator/audit the cluster's own, to an operator.

const (
	auditPageDefault = 50
	auditPageMax     = 200
)

// AuditEntry is one row as a caller sees it.
type AuditEntry struct {
	Action    string `json:"action"`
	Actor     string `json:"actor,omitempty"`
	Resource  string `json:"resource,omitempty"`
	Result    string `json:"result"`
	IP        string `json:"ip,omitempty"`
	UserAgent string `json:"user_agent,omitempty"`
	Metadata  string `json:"metadata,omitempty"`
	CreatedAt string `json:"created_at"`
}

// AuditHandler serves GET /v1/audit.
//
// The namespace is taken from the caller's own credential, never from the
// query string: reading another namespace's audit trail would be a neat way to
// learn who its owners are and when they sign in.
func (h *Handlers) AuditHandler(w http.ResponseWriter, r *http.Request) {
	if !h.auditReady(w, r) {
		return
	}
	namespace := namespaceFromContext(r)
	if namespace == "" {
		writeError(w, http.StatusUnauthorized, "the namespace could not be resolved from this credential")
		return
	}
	h.serveAudit(w, r, auditScope{namespace: namespace})
}

// OperatorAuditHandler serves GET /v1/operator/audit: the cluster's own
// events, the rows written with no namespace. Minting an invite, a node
// joining or being claimed, and a namespace's deletion are recorded there, so
// that "who let that node in" and "who deleted that namespace" have an answer,
// and a namespace created again under the same name does not inherit the
// previous tenant's trail.
//
// The route is operator-only (route policy); no namespace role, grant or key
// reaches it.
func (h *Handlers) OperatorAuditHandler(w http.ResponseWriter, r *http.Request) {
	if !h.auditReady(w, r) {
		return
	}
	h.serveAudit(w, r, auditScope{cluster: true})
}

// auditScope is whose events a page is drawn from: one namespace's, or the
// cluster's own (namespace IS NULL). Exactly one is set.
type auditScope struct {
	namespace string
	cluster   bool
}

// auditReady refuses what neither audit route can answer, before any filter
// is read.
func (h *Handlers) auditReady(w http.ResponseWriter, r *http.Request) bool {
	if h.authService == nil {
		writeError(w, http.StatusServiceUnavailable, "auth service not initialized")
		return false
	}
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return false
	}
	return true
}

// serveAudit reads the filters both audit routes take and answers one page of
// scope's events, most recent first.
func (h *Handlers) serveAudit(w http.ResponseWriter, r *http.Request, scope auditScope) {
	limit := auditPageDefault
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			writeError(w, http.StatusBadRequest, "limit must be a positive whole number")
			return
		}
		limit = parsed
	}
	if limit > auditPageMax {
		limit = auditPageMax
	}

	// An optional filter, checked against the actions this gateway records so
	// a typo comes back as a refusal rather than an empty page.
	action := strings.TrimSpace(r.URL.Query().Get("action"))
	if action != "" && !authsvc.IsAuditAction(action) {
		writeError(w, http.StatusBadRequest, "unknown action: "+action)
		return
	}

	// Only this actor's events. The value is matched exactly, which is what
	// makes it useful on a wallet.
	principal := strings.TrimSpace(r.URL.Query().Get("principal"))

	// Everything after a point in time. The value the API itself returns in
	// created_at is accepted verbatim, so a follower can echo back the last
	// row it saw without reformatting it.
	since := ""
	if raw := strings.TrimSpace(r.URL.Query().Get("since")); raw != "" {
		parsed, err := parseAuditSince(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		since = parsed
	}

	entries, err := h.readAuditEvents(r.Context(), scope, auditFilter{
		Action:    action,
		Principal: principal,
		Since:     since,
		Limit:     limit,
	})
	if err != nil {
		h.logger.Error("the audit trail could not be read", zap.String("namespace", scope.namespace),
			zap.Bool("cluster", scope.cluster), zap.Error(err))
		writeError(w, http.StatusServiceUnavailable, "the audit trail could not be read right now; retry shortly")
		return
	}

	body := map[string]any{
		"events": entries,
		"count":  len(entries),
	}
	if scope.cluster {
		body["scope"] = "cluster"
	} else {
		body["namespace"] = scope.namespace
	}
	writeJSON(w, http.StatusOK, body)
}

// namespaceFromContext reads the namespace the caller's credential resolved to.
func namespaceFromContext(r *http.Request) string {
	if v := r.Context().Value(CtxKeyNamespaceOverride); v != nil {
		if s, ok := v.(string); ok {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

// auditFilter is what a caller asked for. Every field is optional except the
// limit, which the handler has already bounded.
type auditFilter struct {
	Action    string
	Principal string
	Since     string
	Limit     int
}

// auditTimeLayout is how SQLite's CURRENT_TIMESTAMP renders, and so how
// created_at compares: UTC, no zone, second resolution.
const auditTimeLayout = "2006-01-02 15:04:05"

// parseAuditSince normalises a caller's timestamp to the stored form.
//
// Comparing timestamps as strings is only correct when both sides are written
// the same way, so an RFC3339 value is converted to UTC first rather than
// compared as it arrived — "2026-01-01T00:00:00+02:00" sorts after every row
// written that day if it is not.
func parseAuditSince(raw string) (string, error) {
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t.UTC().Format(auditTimeLayout), nil
	}
	if t, err := time.Parse(auditTimeLayout, raw); err == nil {
		return t.Format(auditTimeLayout), nil
	}
	return "", fmt.Errorf("since must be RFC3339 (2026-01-02T15:04:05Z) or %q, got %q", auditTimeLayout, raw)
}

// readAuditEvents returns scope's events, most recent first. Both scopes are
// served by the (namespace, created_at) index.
func (h *Handlers) readAuditEvents(ctx context.Context, scope auditScope, filter auditFilter) ([]AuditEntry, error) {
	query := `SELECT action, actor, resource, result, ip, user_agent, metadata, created_at
	          FROM audit_events WHERE `
	var args []interface{}
	if scope.cluster {
		query += "namespace IS NULL"
	} else {
		query += "namespace = ?"
		args = append(args, scope.namespace)
	}
	if filter.Action != "" {
		query += " AND action = ?"
		args = append(args, filter.Action)
	}
	if filter.Principal != "" {
		query += " AND actor = ?"
		args = append(args, filter.Principal)
	}
	if filter.Since != "" {
		query += " AND created_at > ?"
		args = append(args, filter.Since)
	}
	query += " ORDER BY created_at DESC, id DESC LIMIT ?"
	args = append(args, filter.Limit)

	res, err := h.authService.Audit().Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	entries := make([]AuditEntry, 0)
	if res == nil {
		return entries, nil
	}
	for _, raw := range res.Rows {
		row := raw
		if len(row) < 8 {
			continue
		}
		entries = append(entries, AuditEntry{
			Action:    stringCell(row[0]),
			Actor:     stringCell(row[1]),
			Resource:  stringCell(row[2]),
			Result:    stringCell(row[3]),
			IP:        stringCell(row[4]),
			UserAgent: stringCell(row[5]),
			Metadata:  stringCell(row[6]),
			CreatedAt: stringCell(row[7]),
		})
	}
	return entries, nil
}

func stringCell(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
