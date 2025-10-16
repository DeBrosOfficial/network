package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/DeBrosOfficial/network/pkg/logging"
	"github.com/DeBrosOfficial/network/pkg/rqlite"
	"go.uber.org/zap"
)

// DatabaseMetadataCache manages per-database metadata for routing
type DatabaseMetadataCache struct {
	cache  map[string]*rqlite.DatabaseMetadata
	mu     sync.RWMutex
	logger *logging.ColoredLogger
}

// NewDatabaseMetadataCache creates a new metadata cache
func NewDatabaseMetadataCache(logger *logging.ColoredLogger) *DatabaseMetadataCache {
	return &DatabaseMetadataCache{
		cache:  make(map[string]*rqlite.DatabaseMetadata),
		logger: logger,
	}
}

// Update updates metadata for a database (vector clock aware)
func (dmc *DatabaseMetadataCache) Update(metadata *rqlite.DatabaseMetadata) {
	if metadata == nil {
		return
	}

	dmc.mu.Lock()
	defer dmc.mu.Unlock()

	existing, exists := dmc.cache[metadata.DatabaseName]
	if !exists || metadata.Version > existing.Version {
		dmc.cache[metadata.DatabaseName] = metadata
		dmc.logger.ComponentDebug(logging.ComponentGeneral, "Updated database metadata",
			zap.String("database", metadata.DatabaseName),
			zap.Uint64("version", metadata.Version))
	}
}

// Get retrieves metadata for a database
func (dmc *DatabaseMetadataCache) Get(dbName string) *rqlite.DatabaseMetadata {
	dmc.mu.RLock()
	defer dmc.mu.RUnlock()
	return dmc.cache[dbName]
}

// ResolveEndpoints returns RQLite HTTP endpoints for a database (leader first)
func (dmc *DatabaseMetadataCache) ResolveEndpoints(dbName string) []string {
	dmc.mu.RLock()
	defer dmc.mu.RUnlock()

	metadata, exists := dmc.cache[dbName]
	if !exists {
		return nil
	}

	endpoints := make([]string, 0, len(metadata.NodeIDs))

	// Add leader first
	if metadata.LeaderNodeID != "" {
		if ports, ok := metadata.PortMappings[metadata.LeaderNodeID]; ok {
			endpoint := fmt.Sprintf("http://127.0.0.1:%d", ports.HTTPPort)
			endpoints = append(endpoints, endpoint)
		}
	}

	// Add followers
	for _, nodeID := range metadata.NodeIDs {
		if nodeID == metadata.LeaderNodeID {
			continue // Already added
		}
		if ports, ok := metadata.PortMappings[nodeID]; ok {
			endpoint := fmt.Sprintf("http://127.0.0.1:%d", ports.HTTPPort)
			endpoints = append(endpoints, endpoint)
		}
	}

	return endpoints
}

// StartMetadataSubscriber subscribes to the metadata topic and updates the cache
func (g *Gateway) StartMetadataSubscriber(ctx context.Context) error {
	metadataTopic := "/debros/metadata/v1"

	g.logger.ComponentInfo(logging.ComponentGeneral, "Subscribing to metadata topic",
		zap.String("topic", metadataTopic))

	handler := func(topic string, data []byte) error {
		// Parse metadata message
		var msg rqlite.MetadataMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			g.logger.ComponentDebug(logging.ComponentGeneral, "Failed to parse metadata message",
				zap.Error(err))
			return nil // Don't fail on parse errors
		}

		// Only process METADATA_SYNC messages
		if msg.Type != rqlite.MsgMetadataSync {
			return nil
		}

		// Extract database metadata
		var syncMsg rqlite.MetadataSync
		if err := msg.UnmarshalPayload(&syncMsg); err != nil {
			g.logger.ComponentDebug(logging.ComponentGeneral, "Failed to unmarshal metadata sync",
				zap.Error(err))
			return nil
		}

		if syncMsg.Metadata != nil {
			g.dbMetaCache.Update(syncMsg.Metadata)
		}

		return nil
	}

	return g.client.PubSub().Subscribe(ctx, metadataTopic, handler)
}
