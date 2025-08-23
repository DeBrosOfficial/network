package gateway

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"git.debros.io/DeBros/network/pkg/client"
	"git.debros.io/DeBros/network/pkg/storage"
)

// Database HTTP handlers
func (g *Gateway) dbQueryHandler(w http.ResponseWriter, r *http.Request) {
    if g.client == nil { writeError(w, http.StatusServiceUnavailable, "client not initialized"); return }
    if r.Method != http.MethodPost { writeError(w, http.StatusMethodNotAllowed, "method not allowed"); return }
    var body struct{ SQL string `json:"sql"`; Args []any `json:"args"` }
    if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.SQL == "" { writeError(w, http.StatusBadRequest, "invalid body: {sql, args?}"); return }
    ctx := client.WithInternalAuth(r.Context())
    res, err := g.client.Database().Query(ctx, body.SQL, body.Args...)
    if err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }
    writeJSON(w, http.StatusOK, res)
}

func (g *Gateway) dbTransactionHandler(w http.ResponseWriter, r *http.Request) {
    if g.client == nil { writeError(w, http.StatusServiceUnavailable, "client not initialized"); return }
    if r.Method != http.MethodPost { writeError(w, http.StatusMethodNotAllowed, "method not allowed"); return }
    var body struct{ Statements []string `json:"statements"` }
    if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body.Statements) == 0 { writeError(w, http.StatusBadRequest, "invalid body: {statements:[...]}"); return }
    ctx := client.WithInternalAuth(r.Context())
    if err := g.client.Database().Transaction(ctx, body.Statements); err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }
    writeJSON(w, http.StatusOK, map[string]any{"status":"ok"})
}

func (g *Gateway) dbSchemaHandler(w http.ResponseWriter, r *http.Request) {
    if g.client == nil { writeError(w, http.StatusServiceUnavailable, "client not initialized"); return }
    if r.Method != http.MethodGet { writeError(w, http.StatusMethodNotAllowed, "method not allowed"); return }
    ctx := client.WithInternalAuth(r.Context())
    schema, err := g.client.Database().GetSchema(ctx)
    if err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }
    writeJSON(w, http.StatusOK, schema)
}

func (g *Gateway) dbCreateTableHandler(w http.ResponseWriter, r *http.Request) {
    if g.client == nil { writeError(w, http.StatusServiceUnavailable, "client not initialized"); return }
    if r.Method != http.MethodPost { writeError(w, http.StatusMethodNotAllowed, "method not allowed"); return }
    var body struct{ Schema string `json:"schema"` }
    if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Schema == "" { writeError(w, http.StatusBadRequest, "invalid body: {schema}"); return }
    ctx := client.WithInternalAuth(r.Context())
    if err := g.client.Database().CreateTable(ctx, body.Schema); err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }
    writeJSON(w, http.StatusCreated, map[string]any{"status":"ok"})
}

func (g *Gateway) dbDropTableHandler(w http.ResponseWriter, r *http.Request) {
    if g.client == nil { writeError(w, http.StatusServiceUnavailable, "client not initialized"); return }
    if r.Method != http.MethodPost { writeError(w, http.StatusMethodNotAllowed, "method not allowed"); return }
    var body struct{ Table string `json:"table"` }
    if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Table == "" { writeError(w, http.StatusBadRequest, "invalid body: {table}"); return }
    ctx := client.WithInternalAuth(r.Context())
    if err := g.client.Database().DropTable(ctx, body.Table); err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }
    writeJSON(w, http.StatusOK, map[string]any{"status":"ok"})
}

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

    // Use internal auth for downstream client calls; gateway has already authenticated the request
    ctx := client.WithInternalAuth(r.Context())

	switch r.Method {
	case http.MethodGet:
        val, err := g.client.Storage().Get(ctx, key)
		if err != nil {
            // Some storage backends may return base64-encoded text; try best-effort decode for transparency
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

func (g *Gateway) storageGetHandler(w http.ResponseWriter, r *http.Request) {
	if g.client == nil {
		writeError(w, http.StatusServiceUnavailable, "client not initialized")
		return
	}
	key := r.URL.Query().Get("key")
	if key == "" {
		writeError(w, http.StatusBadRequest, "missing 'key'")
		return
	}
	if !g.validateNamespaceParam(r) {
		writeError(w, http.StatusForbidden, "namespace mismatch")
		return
	}
    val, err := g.client.Storage().Get(client.WithInternalAuth(r.Context()), key)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(val)
}

func (g *Gateway) storagePutHandler(w http.ResponseWriter, r *http.Request) {
	if g.client == nil {
		writeError(w, http.StatusServiceUnavailable, "client not initialized")
		return
	}
	key := r.URL.Query().Get("key")
	if key == "" {
		writeError(w, http.StatusBadRequest, "missing 'key'")
		return
	}
	if !g.validateNamespaceParam(r) {
		writeError(w, http.StatusForbidden, "namespace mismatch")
		return
	}
	defer r.Body.Close()
	b, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read body")
		return
	}
    if err := g.client.Storage().Put(client.WithInternalAuth(r.Context()), key, b); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"status": "ok", "key": key, "size": len(b)})
}

func (g *Gateway) storageDeleteHandler(w http.ResponseWriter, r *http.Request) {
	if g.client == nil {
		writeError(w, http.StatusServiceUnavailable, "client not initialized")
		return
	}
	if !g.validateNamespaceParam(r) {
		writeError(w, http.StatusForbidden, "namespace mismatch")
		return
	}
	key := r.URL.Query().Get("key")
	if key == "" {
		var body struct {
			Key string `json:"key"`
			Namespace string `json:"namespace"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
			key = body.Key
		}
	}
	if key == "" {
		writeError(w, http.StatusBadRequest, "missing 'key'")
		return
	}
    if err := g.client.Storage().Delete(client.WithInternalAuth(r.Context()), key); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "key": key})
}

func (g *Gateway) storageListHandler(w http.ResponseWriter, r *http.Request) {
	if g.client == nil {
		writeError(w, http.StatusServiceUnavailable, "client not initialized")
		return
	}
	if !g.validateNamespaceParam(r) {
		writeError(w, http.StatusForbidden, "namespace mismatch")
		return
	}
	prefix := r.URL.Query().Get("prefix")
	limitStr := r.URL.Query().Get("limit")
	limit := 100
	if limitStr != "" {
		if n, err := strconv.Atoi(limitStr); err == nil && n > 0 {
			limit = n
		}
	}
    keys, err := g.client.Storage().List(client.WithInternalAuth(r.Context()), prefix, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"keys": keys})
}

func (g *Gateway) storageExistsHandler(w http.ResponseWriter, r *http.Request) {
	if g.client == nil {
		writeError(w, http.StatusServiceUnavailable, "client not initialized")
		return
	}
	if !g.validateNamespaceParam(r) {
		writeError(w, http.StatusForbidden, "namespace mismatch")
		return
	}
	key := r.URL.Query().Get("key")
	if key == "" {
		writeError(w, http.StatusBadRequest, "missing 'key'")
		return
	}
    exists, err := g.client.Storage().Exists(client.WithInternalAuth(r.Context()), key)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"exists": exists})
}

func (g *Gateway) networkConnectHandler(w http.ResponseWriter, r *http.Request) {
	if g.client == nil {
		writeError(w, http.StatusServiceUnavailable, "client not initialized")
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		Multiaddr string `json:"multiaddr"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Multiaddr == "" {
		writeError(w, http.StatusBadRequest, "invalid body: expected {multiaddr}")
		return
	}
	if err := g.client.Network().ConnectToPeer(r.Context(), body.Multiaddr); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (g *Gateway) networkDisconnectHandler(w http.ResponseWriter, r *http.Request) {
	if g.client == nil {
		writeError(w, http.StatusServiceUnavailable, "client not initialized")
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		PeerID string `json:"peer_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.PeerID == "" {
		writeError(w, http.StatusBadRequest, "invalid body: expected {peer_id}")
		return
	}
	if err := g.client.Network().DisconnectFromPeer(r.Context(), body.PeerID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (g *Gateway) validateNamespaceParam(r *http.Request) bool {
	qns := r.URL.Query().Get("namespace")
	if qns == "" {
		return true
	}
	if v := r.Context().Value(storage.CtxKeyNamespaceOverride); v != nil {
		if s, ok := v.(string); ok && s != "" {
			return s == qns
		}
	}
	// If no namespace in context, disallow explicit namespace param
	return false
}
