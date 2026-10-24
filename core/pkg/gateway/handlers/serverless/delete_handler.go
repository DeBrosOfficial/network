package serverless

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/Orama-Network/orama/core/pkg/gateway/auth"
	"github.com/Orama-Network/orama/core/pkg/serverless"
	"go.uber.org/zap"
)

// purgeQueryParam turns DELETE /v1/functions/{name} into a purge.
const purgeQueryParam = "purge"

// DeleteFunction handles DELETE /v1/functions/{name}[@version][?purge=true].
//
// A delete marks the version rows deleted: they stop resolving, and stay
// listed by `GET /v1/functions?all=true`. A purge removes them for good with
// everything keyed to them (Registry.Purge). A purge destroys history nobody
// can recover, so it takes the namespace's owner or an admin, where a delete
// takes anyone who manages functions.
func (h *ServerlessHandlers) DeleteFunction(w http.ResponseWriter, r *http.Request, name string, version int) {
	namespace, ok := managedNamespace(w, r)
	if !ok {
		return
	}
	purge, err := boolQueryParam(r, purgeQueryParam)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if purge && !h.getCallerIsAdminFromRequest(r) {
		writeError(w, http.StatusForbidden,
			"purging a function removes it and its history for good; only the namespace's owner or an admin may")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	remove, action, message := h.registry.Delete, auth.AuditFunctionDeleted, "Function deleted successfully"
	if purge {
		remove, action, message = h.registry.Purge, auth.AuditFunctionPurged, "Function purged"
	}
	if err := remove(ctx, namespace, name, version); err != nil {
		if serverless.IsNotFound(err) {
			writeError(w, http.StatusNotFound, "Function not found")
		} else {
			writeStoreError(w, "Failed to delete function", err)
		}
		return
	}
	if purge {
		h.refreshDispatcher(ctx)
	}

	h.recordAudit(r, namespace, action, name)

	writeJSON(w, http.StatusOK, map[string]string{
		"message": message,
	})
}

// RollbackFunction handles POST /v1/functions/{name}@{version}/rollback: the
// version becomes the function's current one, the only one that is invoked.
func (h *ServerlessHandlers) RollbackFunction(w http.ResponseWriter, r *http.Request, name string, version int) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	namespace, ok := managedNamespace(w, r)
	if !ok {
		return
	}
	if version <= 0 {
		writeError(w, http.StatusBadRequest,
			"name the version to roll back to: POST /v1/functions/<name>@<version>/rollback")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if err := h.registry.Rollback(ctx, namespace, name, version); err != nil {
		if serverless.IsNotFound(err) {
			writeError(w, http.StatusNotFound, fmt.Sprintf(
				"function %s has no version %d to roll back to: it does not exist, was pruned or was deleted", name, version))
		} else {
			writeStoreError(w, "Failed to roll back function", err)
		}
		return
	}
	// The dispatcher subscribes only to topics whose trigger belongs to an
	// active version, and a rollback of a disabled function makes its
	// triggers live again.
	h.refreshDispatcher(ctx)

	h.recordAudit(r, namespace, auth.AuditFunctionRolledBack, name+"@"+strconv.Itoa(version))

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":   "ok",
		"function": name,
		"version":  version,
	})
}

// refreshDispatcher re-reads the pubsub trigger rows after a purge or a
// rollback changed them, so the dispatcher drops a subscription that no
// trigger needs any more (bugboard #282). Its periodic loop does the same, so
// a failure here is logged, not returned.
func (h *ServerlessHandlers) refreshDispatcher(ctx context.Context) {
	if h.dispatcher == nil {
		return
	}
	if err := h.dispatcher.Refresh(ctx); err != nil {
		h.logger.Warn("PubSubDispatcher Refresh after a function change failed (periodic loop will retry)",
			zap.Error(err))
	}
}

// boolQueryParam reads an optional boolean query parameter: absent is false,
// and a value strconv.ParseBool cannot read is an error naming it.
func boolQueryParam(r *http.Request, name string) (bool, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return false, nil
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("query parameter %s must be true or false, not %q", name, raw)
	}
	return v, nil
}
