package gateway

import (
	"io"
	"net/http"
)

func (g *Gateway) storageHandler(w http.ResponseWriter, r *http.Request) {
	if g.client == nil {
		writeError(w, http.StatusServiceUnavailable, "client not initialized")
		return
	}

	key := r.URL.Query().Get("key")
	if key == "" {
		writeError(w, http.StatusBadRequest, "missing 'key' query parameter")
		return
	}

	ctx := r.Context()

	switch r.Method {
	case http.MethodGet:
		val, err := g.client.Storage().Get(ctx, key)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(val)
		return

	case http.MethodPut:
		defer r.Body.Close()
		b, err := io.ReadAll(r.Body)
		if err != nil {
			writeError(w, http.StatusBadRequest, "failed to read body")
			return
		}
		if err := g.client.Storage().Put(ctx, key, b); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{
			"status": "ok",
			"key":    key,
			"size":   len(b),
		})
		return

	case http.MethodOptions:
		w.WriteHeader(http.StatusNoContent)
		return
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
}

func (g *Gateway) networkStatusHandler(w http.ResponseWriter, r *http.Request) {
	if g.client == nil {
		writeError(w, http.StatusServiceUnavailable, "client not initialized")
		return
	}
	ctx := r.Context()
	status, err := g.client.Network().GetStatus(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (g *Gateway) networkPeersHandler(w http.ResponseWriter, r *http.Request) {
	if g.client == nil {
		writeError(w, http.StatusServiceUnavailable, "client not initialized")
		return
	}
	ctx := r.Context()
	peers, err := g.client.Network().GetPeers(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, peers)
}
