package gateway

import (
	"encoding/json"
	"net/http"
	"time"

	"git.debros.io/DeBros/network/pkg/client"
	"git.debros.io/DeBros/network/pkg/logging"
	"go.uber.org/zap"
)

// healthResponse is the JSON structure used by healthHandler
type healthResponse struct {
	Status    string    `json:"status"`
	StartedAt time.Time `json:"started_at"`
	Uptime    string    `json:"uptime"`
}

func (g *Gateway) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	server := healthResponse{
		Status:    "ok",
		StartedAt: g.startedAt,
		Uptime:    time.Since(g.startedAt).String(),
	}

	var clientHealth *client.HealthStatus
	if g.client != nil {
		if h, err := g.client.Health(); err == nil {
			clientHealth = h
		} else {
			g.logger.ComponentWarn(logging.ComponentClient, "failed to fetch client health", zap.Error(err))
		}
	}

	resp := struct {
		Status string               `json:"status"`
		Server healthResponse       `json:"server"`
		Client *client.HealthStatus `json:"client"`
	}{
		Status: "ok",
		Server: server,
		Client: clientHealth,
	}

	_ = json.NewEncoder(w).Encode(resp)
}

// statusHandler aggregates server uptime and network status
func (g *Gateway) statusHandler(w http.ResponseWriter, r *http.Request) {
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
	writeJSON(w, http.StatusOK, map[string]any{
		"server": map[string]any{
			"started_at": g.startedAt,
			"uptime":     time.Since(g.startedAt).String(),
		},
		"network": status,
	})
}
