package serverless

import (
	"context"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// allQueryParam makes GET /v1/functions list every version row.
const allQueryParam = "all"

// ListFunctions handles GET /v1/functions[?all=true]: the current version of
// each function, or with all=true every version row the namespace holds,
// whatever its status (disabled, superseded, deleted), until it is purged.
func (h *ServerlessHandlers) ListFunctions(w http.ResponseWriter, r *http.Request) {
	namespace, ok := managedNamespace(w, r)
	if !ok {
		return
	}
	all, err := boolQueryParam(r, allQueryParam)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	list := h.registry.List
	if all {
		list = h.registry.ListAll
	}
	functions, err := list(ctx, namespace)
	if err != nil {
		h.logger.Error("Failed to list functions",
			zap.Error(err))
		writeStoreError(w, "Failed to list functions", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"functions": functions,
		"count":     len(functions),
	})
}
