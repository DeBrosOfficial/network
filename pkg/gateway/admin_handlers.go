package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/DeBrosOfficial/network/pkg/logging"
	"github.com/DeBrosOfficial/network/pkg/rqlite"
	"go.uber.org/zap"
)

// CreateDatabaseRequest is the request body for database creation
type CreateDatabaseRequest struct {
	Database          string `json:"database"`
	ReplicationFactor int    `json:"replication_factor,omitempty"` // defaults to 3
}

// CreateDatabaseResponse is the response for database creation
type CreateDatabaseResponse struct {
	Status   string `json:"status"`
	Database string `json:"database"`
	Message  string `json:"message,omitempty"`
	Error    string `json:"error,omitempty"`
}

// databaseCreateHandler handles database creation requests via pubsub
func (g *Gateway) databaseCreateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CreateDatabaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		g.respondJSON(w, http.StatusBadRequest, CreateDatabaseResponse{
			Status: "error",
			Error:  "Invalid request body",
		})
		return
	}

	if req.Database == "" {
		g.respondJSON(w, http.StatusBadRequest, CreateDatabaseResponse{
			Status: "error",
			Error:  "database field is required",
		})
		return
	}

	// Default replication factor
	if req.ReplicationFactor == 0 {
		req.ReplicationFactor = 3
	}

	// Check if database already exists in metadata
	if existing := g.dbMetaCache.Get(req.Database); existing != nil {
		g.respondJSON(w, http.StatusConflict, CreateDatabaseResponse{
			Status:   "exists",
			Database: req.Database,
			Message:  "Database already exists",
		})
		return
	}

	g.logger.ComponentInfo(logging.ComponentGeneral, "Creating database via gateway",
		zap.String("database", req.Database),
		zap.Int("replication_factor", req.ReplicationFactor))

	// Publish DATABASE_CREATE_REQUEST via pubsub
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// We need to get the node ID to act as requester
	// For now, use a placeholder - the actual node will coordinate
	createReq := rqlite.DatabaseCreateRequest{
		DatabaseName:      req.Database,
		RequesterNodeID:   "gateway", // Gateway is requesting on behalf of client
		ReplicationFactor: req.ReplicationFactor,
	}

	msgData, err := rqlite.MarshalMetadataMessage(rqlite.MsgDatabaseCreateRequest, "gateway", createReq)
	if err != nil {
		g.logger.ComponentError(logging.ComponentGeneral, "Failed to marshal create request",
			zap.Error(err))
		g.respondJSON(w, http.StatusInternalServerError, CreateDatabaseResponse{
			Status: "error",
			Error:  fmt.Sprintf("Failed to create database: %v", err),
		})
		return
	}

	// Publish to metadata topic
	metadataTopic := "/debros/metadata/v1"
	if err := g.client.PubSub().Publish(ctx, metadataTopic, msgData); err != nil {
		g.logger.ComponentError(logging.ComponentGeneral, "Failed to publish create request",
			zap.Error(err))
		g.respondJSON(w, http.StatusInternalServerError, CreateDatabaseResponse{
			Status: "error",
			Error:  fmt.Sprintf("Failed to publish create request: %v", err),
		})
		return
	}

	// Wait briefly for metadata sync (3 seconds)
	waitCtx, waitCancel := context.WithTimeout(ctx, 3*time.Second)
	defer waitCancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-waitCtx.Done():
			// Timeout - database creation is async, return accepted status
			g.respondJSON(w, http.StatusAccepted, CreateDatabaseResponse{
				Status:   "accepted",
				Database: req.Database,
				Message:  "Database creation initiated, it may take a few seconds to become available",
			})
			return
		case <-ticker.C:
			// Check if metadata arrived
			if metadata := g.dbMetaCache.Get(req.Database); metadata != nil {
				g.respondJSON(w, http.StatusOK, CreateDatabaseResponse{
					Status:   "created",
					Database: req.Database,
					Message:  "Database created successfully",
				})
				return
			}
		}
	}
}
