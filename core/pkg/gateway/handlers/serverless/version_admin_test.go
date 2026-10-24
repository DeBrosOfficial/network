package serverless

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Orama-Network/orama/core/pkg/gateway/auth"
	"github.com/Orama-Network/orama/core/pkg/gateway/ctxkeys"
	"github.com/Orama-Network/orama/core/pkg/serverless"
)

// withGrant is r carrying the namespace grant of role.
func withGrant(r *http.Request, role auth.Role) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), ctxkeys.Grant, &auth.Grant{Role: role}))
}

// bug-940: a purge is for the namespace's owner or an admin; a developer, who
// may delete, may not purge, and the registry is never reached.
func TestDeleteFunction_purgeTakesOwnerOrAdmin(t *testing.T) {
	for _, tc := range []struct {
		role auth.Role
		want int
	}{
		{auth.RoleOwner, http.StatusOK},
		{auth.RoleAdmin, http.StatusOK},
		{auth.RoleDeveloper, http.StatusForbidden},
		{auth.RoleRuntime, http.StatusForbidden},
	} {
		t.Run(string(tc.role), func(t *testing.T) {
			reg := newMockRegistry()
			h := newTestHandlers(reg)
			req := withGrant(asCredentialOf(httptest.NewRequest(http.MethodDelete, "/v1/functions/fn?purge=true", nil), "ns"), tc.role)
			rec := httptest.NewRecorder()
			h.DeleteFunction(rec, req, "fn", 0)
			if rec.Code != tc.want {
				t.Fatalf("purge as %s = %d; want %d (%s)", tc.role, rec.Code, tc.want, rec.Body.String())
			}
			purged := len(reg.purged) == 1 && reg.purged[0] == "fn@0"
			if purged != (tc.want == http.StatusOK) {
				t.Errorf("purge as %s reached the registry: %v", tc.role, reg.purged)
			}
		})
	}
}

// A developer still deletes: only the purge is restricted.
func TestDeleteFunction_withoutPurgeIsNotRestricted(t *testing.T) {
	reg := newMockRegistry()
	h := newTestHandlers(reg)
	req := withGrant(asCredentialOf(httptest.NewRequest(http.MethodDelete, "/v1/functions/fn", nil), "ns"), auth.RoleDeveloper)
	rec := httptest.NewRecorder()
	h.DeleteFunction(rec, req, "fn", 0)
	if rec.Code != http.StatusOK || len(reg.purged) != 0 {
		t.Fatalf("developer delete = %d, purged %v; want 200 and no purge", rec.Code, reg.purged)
	}
}

func TestDeleteFunction_purgeOfOneVersionAndBadValues(t *testing.T) {
	reg := newMockRegistry()
	h := newTestHandlers(reg)
	req := withGrant(asCredentialOf(httptest.NewRequest(http.MethodDelete, "/v1/functions/fn@3?purge=true", nil), "ns"), auth.RoleOwner)
	rec := httptest.NewRecorder()
	h.DeleteFunction(rec, req, "fn", 3)
	if rec.Code != http.StatusOK || len(reg.purged) != 1 || reg.purged[0] != "fn@3" {
		t.Fatalf("purge fn@3 = %d, purged %v", rec.Code, reg.purged)
	}

	req = withGrant(asCredentialOf(httptest.NewRequest(http.MethodDelete, "/v1/functions/fn?purge=maybe", nil), "ns"), auth.RoleOwner)
	rec = httptest.NewRecorder()
	h.DeleteFunction(rec, req, "fn", 0)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("purge=maybe = %d; want 400", rec.Code)
	}

	reg.deleteErr = serverless.ErrFunctionNotFound
	req = withGrant(asCredentialOf(httptest.NewRequest(http.MethodDelete, "/v1/functions/gone?purge=true", nil), "ns"), auth.RoleOwner)
	rec = httptest.NewRecorder()
	h.DeleteFunction(rec, req, "gone", 0)
	if rec.Code != http.StatusNotFound {
		t.Errorf("purge of an unknown function = %d; want 404", rec.Code)
	}
}

func TestRollbackFunction(t *testing.T) {
	reg := newMockRegistry()
	h := newTestHandlers(reg)
	req := asCredentialOf(httptest.NewRequest(http.MethodPost, "/v1/functions/fn@2/rollback", nil), "ns")
	rec := httptest.NewRecorder()
	h.handleFunctionByName(rec, req)
	if rec.Code != http.StatusOK || len(reg.rolledBack) != 1 || reg.rolledBack[0] != "fn@2" {
		t.Fatalf("rollback fn@2 = %d (%s), rolled back %v", rec.Code, rec.Body.String(), reg.rolledBack)
	}

	for _, tc := range []struct {
		name, method, path string
		want               int
	}{
		{"no version", http.MethodPost, "/v1/functions/fn/rollback", http.StatusBadRequest},
		{"not a post", http.MethodGet, "/v1/functions/fn@2/rollback", http.StatusMethodNotAllowed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.handleFunctionByName(rec, asCredentialOf(httptest.NewRequest(tc.method, tc.path, nil), "ns"))
			if rec.Code != tc.want {
				t.Errorf("%s %s = %d; want %d", tc.method, tc.path, rec.Code, tc.want)
			}
		})
	}

	reg.getErr = serverless.ErrVersionNotFound
	rec = httptest.NewRecorder()
	h.handleFunctionByName(rec, asCredentialOf(httptest.NewRequest(http.MethodPost, "/v1/functions/fn@9/rollback", nil), "ns"))
	if rec.Code != http.StatusNotFound {
		t.Errorf("rollback to a version that cannot run = %d; want 404", rec.Code)
	}
}

func TestListFunctions_allListsEveryVersion(t *testing.T) {
	reg := newMockRegistry()
	h := newTestHandlers(reg)
	rec := httptest.NewRecorder()
	h.ListFunctions(rec, asCredentialOf(httptest.NewRequest(http.MethodGet, "/v1/functions?all=true", nil), "ns"))
	if rec.Code != http.StatusOK || !reg.listedAll {
		t.Fatalf("list ?all=true = %d, ListAll called %v", rec.Code, reg.listedAll)
	}
	reg.listedAll = false
	rec = httptest.NewRecorder()
	h.ListFunctions(rec, asCredentialOf(httptest.NewRequest(http.MethodGet, "/v1/functions", nil), "ns"))
	if rec.Code != http.StatusOK || reg.listedAll {
		t.Errorf("plain list = %d, ListAll called %v; want the current versions only", rec.Code, reg.listedAll)
	}
	rec = httptest.NewRecorder()
	h.ListFunctions(rec, asCredentialOf(httptest.NewRequest(http.MethodGet, "/v1/functions?all=yes-please", nil), "ns"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("all=yes-please = %d; want 400", rec.Code)
	}
}
