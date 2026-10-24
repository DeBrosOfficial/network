package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The cluster's own events are the rows with no namespace. These run the real
// statement against a migrated registry, because `namespace = NULL` matches
// nothing and a fake would answer it with whatever rows it was given.
func TestOperatorAuditHandler_readsTheClusterRowsAndNoNamespacesRows(t *testing.T) {
	f := newFlow(t)
	for _, row := range []struct {
		namespace any
		action    string
	}{
		{nil, "operator.action"},
		{nil, "namespace.delete"},
		{flowNamespace, "key.issue"},
	} {
		if _, err := f.db.Exec(`INSERT INTO audit_events(namespace, actor, action, result) VALUES (?, '0xop', ?, 'success')`,
			row.namespace, row.action); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	// The caller's credential names a namespace; the operator route reads the
	// cluster's rows whatever it names.
	rec := httptest.NewRecorder()
	f.h.OperatorAuditHandler(rec, auditRequest(flowNamespace, "?namespace="+flowNamespace))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Scope  string       `json:"scope"`
		Count  int          `json:"count"`
		Events []AuditEntry `json:"events"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Scope != "cluster" || body.Count != 2 {
		t.Fatalf("scope %q, %d events; want cluster, 2: %+v", body.Scope, body.Count, body.Events)
	}
	for _, e := range body.Events {
		if e.Action == "key.issue" {
			t.Error("a namespace's event was served as the cluster's")
		}
	}

	// And the namespace route still serves only that namespace's.
	rec = httptest.NewRecorder()
	f.h.AuditHandler(rec, auditRequest(flowNamespace, ""))
	var ns struct {
		Count  int          `json:"count"`
		Events []AuditEntry `json:"events"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&ns); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if ns.Count != 1 || ns.Events[0].Action != "key.issue" {
		t.Errorf("namespace trail = %+v, want only key.issue", ns.Events)
	}
}

// The filters are the namespace route's: an unknown action is refused, and the
// page is bounded.
func TestOperatorAuditHandler_takesTheSameFilters(t *testing.T) {
	db := &auditQueryDB{}
	h := auditHandlers(t, db)

	rec := httptest.NewRecorder()
	h.OperatorAuditHandler(rec, auditRequest("", "?action=nonsense"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("unknown action: status %d, want 400", rec.Code)
	}

	rec = httptest.NewRecorder()
	h.OperatorAuditHandler(rec, auditRequest("", "?limit=100000"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	if got := db.lastArgs[len(db.lastArgs)-1]; got != auditPageMax {
		t.Errorf("limit bound = %v, want %d", got, auditPageMax)
	}
	for _, arg := range db.lastArgs[:len(db.lastArgs)-1] {
		t.Errorf("the cluster read bound %v; it binds nothing but the limit", arg)
	}
}
