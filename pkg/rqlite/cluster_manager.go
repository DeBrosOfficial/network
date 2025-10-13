package rqlite

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/DeBrosOfficial/network/pkg/config"
	"github.com/DeBrosOfficial/network/pkg/pubsub"
	"go.uber.org/zap"
)

// ClusterManager manages multiple RQLite database clusters on a single node
type ClusterManager struct {
	nodeID          string
	config          *config.DatabaseConfig
	discoveryConfig *config.DiscoveryConfig
	dataDir         string
	logger          *zap.Logger

	metadataStore       *MetadataStore
	activeClusters      map[string]*RQLiteInstance // dbName -> instance
	portManager         *PortManager
	pubsubAdapter       *pubsub.ClientAdapter
	coordinatorRegistry *CoordinatorRegistry

	mu     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
}

// NewClusterManager creates a new cluster manager
func NewClusterManager(
	nodeID string,
	cfg *config.DatabaseConfig,
	discoveryCfg *config.DiscoveryConfig,
	dataDir string,
	pubsubAdapter *pubsub.ClientAdapter,
	logger *zap.Logger,
) *ClusterManager {
	ctx, cancel := context.WithCancel(context.Background())

	// Initialize port manager
	portManager := NewPortManager(
		PortRange{Start: cfg.PortRangeHTTPStart, End: cfg.PortRangeHTTPEnd},
		PortRange{Start: cfg.PortRangeRaftStart, End: cfg.PortRangeRaftEnd},
	)

	return &ClusterManager{
		nodeID:              nodeID,
		config:              cfg,
		discoveryConfig:     discoveryCfg,
		dataDir:             dataDir,
		logger:              logger,
		metadataStore:       NewMetadataStore(),
		activeClusters:      make(map[string]*RQLiteInstance),
		portManager:         portManager,
		pubsubAdapter:       pubsubAdapter,
		coordinatorRegistry: NewCoordinatorRegistry(),
		ctx:                 ctx,
		cancel:              cancel,
	}
}

// Start starts the cluster manager
func (cm *ClusterManager) Start() error {
	cm.logger.Info("Starting cluster manager",
		zap.String("node_id", cm.nodeID),
		zap.Int("max_databases", cm.config.MaxDatabases))

	// Subscribe to metadata topic
	metadataTopic := "/debros/metadata/v1"
	if err := cm.pubsubAdapter.Subscribe(cm.ctx, metadataTopic, cm.handleMetadataMessage); err != nil {
		return fmt.Errorf("failed to subscribe to metadata topic: %w", err)
	}

	// Announce node capacity
	go cm.announceCapacityPeriodically()

	// Start health monitoring
	go cm.monitorHealth()

	// Start idle detection for hibernation
	if cm.config.HibernationTimeout > 0 {
		go cm.monitorIdleDatabases()
	}

	// Perform startup reconciliation
	go cm.reconcileOrphanedData()

	cm.logger.Info("Cluster manager started successfully")
	return nil
}

// Stop stops the cluster manager
func (cm *ClusterManager) Stop() error {
	cm.logger.Info("Stopping cluster manager")

	cm.cancel()

	// Stop all active clusters
	cm.mu.Lock()
	defer cm.mu.Unlock()

	for dbName, instance := range cm.activeClusters {
		cm.logger.Info("Stopping database instance",
			zap.String("database", dbName))
		if err := instance.Stop(); err != nil {
			cm.logger.Warn("Error stopping database instance",
				zap.String("database", dbName),
				zap.Error(err))
		}
	}

	cm.logger.Info("Cluster manager stopped")
	return nil
}

// handleMetadataMessage processes incoming metadata messages
func (cm *ClusterManager) handleMetadataMessage(topic string, data []byte) error {
	msg, err := UnmarshalMetadataMessage(data)
	if err != nil {
		// Silently ignore non-metadata messages (other pubsub traffic)
		cm.logger.Debug("Ignoring non-metadata message on metadata topic", zap.Error(err))
		return nil
	}

	// Skip messages from self
	if msg.NodeID == cm.nodeID {
		return nil
	}

	cm.logger.Debug("Received metadata message",
		zap.String("type", string(msg.Type)),
		zap.String("from", msg.NodeID))

	switch msg.Type {
	case MsgDatabaseCreateRequest:
		return cm.handleCreateRequest(msg)
	case MsgDatabaseCreateResponse:
		return cm.handleCreateResponse(msg)
	case MsgDatabaseCreateConfirm:
		return cm.handleCreateConfirm(msg)
	case MsgDatabaseStatusUpdate:
		return cm.handleStatusUpdate(msg)
	case MsgNodeCapacityAnnouncement:
		return cm.handleCapacityAnnouncement(msg)
	case MsgNodeHealthPing:
		return cm.handleHealthPing(msg)
	case MsgDatabaseIdleNotification:
		return cm.handleIdleNotification(msg)
	case MsgDatabaseShutdownCoordinated:
		return cm.handleShutdownCoordinated(msg)
	case MsgDatabaseWakeupRequest:
		return cm.handleWakeupRequest(msg)
	case MsgNodeReplacementNeeded:
		return cm.handleNodeReplacementNeeded(msg)
	case MsgNodeReplacementOffer:
		return cm.handleNodeReplacementOffer(msg)
	case MsgNodeReplacementConfirm:
		return cm.handleNodeReplacementConfirm(msg)
	case MsgMetadataSync:
		return cm.handleMetadataSync(msg)
	case MsgMetadataChecksumReq:
		return cm.handleChecksumRequest(msg)
	case MsgMetadataChecksumRes:
		return cm.handleChecksumResponse(msg)
	default:
		cm.logger.Debug("Unhandled message type", zap.String("type", string(msg.Type)))
	}

	return nil
}

// CreateDatabase creates a new database cluster
func (cm *ClusterManager) CreateDatabase(dbName string, replicationFactor int) error {
	cm.logger.Info("Initiating database creation",
		zap.String("database", dbName),
		zap.Int("replication_factor", replicationFactor))

	// Check if database already exists
	if existing := cm.metadataStore.GetDatabase(dbName); existing != nil {
		return fmt.Errorf("database %s already exists", dbName)
	}

	// Create coordinator for this database creation
	coordinator := NewCreateCoordinator(dbName, replicationFactor, cm.nodeID, cm.logger)
	cm.coordinatorRegistry.Register(coordinator)
	defer cm.coordinatorRegistry.Remove(dbName)

	// Broadcast create request
	req := DatabaseCreateRequest{
		DatabaseName:      dbName,
		RequesterNodeID:   cm.nodeID,
		ReplicationFactor: replicationFactor,
	}

	msgData, err := MarshalMetadataMessage(MsgDatabaseCreateRequest, cm.nodeID, req)
	if err != nil {
		return fmt.Errorf("failed to marshal create request: %w", err)
	}

	topic := "/debros/metadata/v1"
	if err := cm.pubsubAdapter.Publish(cm.ctx, topic, msgData); err != nil {
		return fmt.Errorf("failed to publish create request: %w", err)
	}

	cm.logger.Info("Database create request broadcasted, waiting for responses",
		zap.String("database", dbName))

	// Wait for responses (2 seconds timeout)
	waitCtx, cancel := context.WithTimeout(cm.ctx, 2*time.Second)
	defer cancel()

	if err := coordinator.WaitForResponses(waitCtx, 2*time.Second); err != nil {
		cm.logger.Warn("Timeout waiting for responses", zap.String("database", dbName), zap.Error(err))
	}

	// Select nodes
	responses := coordinator.GetResponses()
	if len(responses) < replicationFactor {
		return fmt.Errorf("insufficient nodes responded: got %d, need %d", len(responses), replicationFactor)
	}

	selectedResponses := coordinator.SelectNodes()
	cm.logger.Info("Selected nodes for database",
		zap.String("database", dbName),
		zap.Int("count", len(selectedResponses)))

	// Determine if this node is the coordinator (lowest ID among responders)
	allNodeIDs := make([]string, len(selectedResponses))
	for i, resp := range selectedResponses {
		allNodeIDs[i] = resp.NodeID
	}
	coordinatorID := SelectCoordinator(allNodeIDs)
	isCoordinator := coordinatorID == cm.nodeID

	if isCoordinator {
		cm.logger.Info("This node is coordinator, broadcasting confirmation",
			zap.String("database", dbName))

		// Build node assignments
		assignments := make([]NodeAssignment, len(selectedResponses))
		for i, resp := range selectedResponses {
			role := "follower"
			if i == 0 {
				role = "leader"
			}
			assignments[i] = NodeAssignment{
				NodeID:   resp.NodeID,
				HTTPPort: resp.AvailablePorts.HTTPPort,
				RaftPort: resp.AvailablePorts.RaftPort,
				Role:     role,
			}
		}

		// Broadcast confirmation
		confirm := DatabaseCreateConfirm{
			DatabaseName:      dbName,
			SelectedNodes:     assignments,
			CoordinatorNodeID: cm.nodeID,
		}

		msgData, err := MarshalMetadataMessage(MsgDatabaseCreateConfirm, cm.nodeID, confirm)
		if err != nil {
			return fmt.Errorf("failed to marshal create confirm: %w", err)
		}

		if err := cm.pubsubAdapter.Publish(cm.ctx, topic, msgData); err != nil {
			return fmt.Errorf("failed to publish create confirm: %w", err)
		}

		cm.logger.Info("Database creation confirmation broadcasted",
			zap.String("database", dbName))
	}

	return nil
}

// GetDatabase returns the RQLite instance for a database
func (cm *ClusterManager) GetDatabase(dbName string) *RQLiteInstance {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.activeClusters[dbName]
}

// ListDatabases returns all active database names
func (cm *ClusterManager) ListDatabases() []string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	names := make([]string, 0, len(cm.activeClusters))
	for name := range cm.activeClusters {
		names = append(names, name)
	}
	return names
}

// GetMetadataStore returns the metadata store
func (cm *ClusterManager) GetMetadataStore() *MetadataStore {
	return cm.metadataStore
}

// announceCapacityPeriodically announces node capacity every 30 seconds
func (cm *ClusterManager) announceCapacityPeriodically() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Announce immediately
	cm.announceCapacity()

	for {
		select {
		case <-cm.ctx.Done():
			return
		case <-ticker.C:
			cm.announceCapacity()
		}
	}
}

// announceCapacity announces this node's capacity
func (cm *ClusterManager) announceCapacity() {
	cm.mu.RLock()
	currentDatabases := len(cm.activeClusters)
	cm.mu.RUnlock()

	announcement := NodeCapacityAnnouncement{
		NodeID:           cm.nodeID,
		MaxDatabases:     cm.config.MaxDatabases,
		CurrentDatabases: currentDatabases,
		PortRangeHTTP:    PortRange{Start: cm.config.PortRangeHTTPStart, End: cm.config.PortRangeHTTPEnd},
		PortRangeRaft:    PortRange{Start: cm.config.PortRangeRaftStart, End: cm.config.PortRangeRaftEnd},
	}

	msgData, err := MarshalMetadataMessage(MsgNodeCapacityAnnouncement, cm.nodeID, announcement)
	if err != nil {
		cm.logger.Warn("Failed to marshal capacity announcement", zap.Error(err))
		return
	}

	topic := "/debros/metadata/v1"
	if err := cm.pubsubAdapter.Publish(cm.ctx, topic, msgData); err != nil {
		cm.logger.Warn("Failed to publish capacity announcement", zap.Error(err))
		return
	}

	// Update local metadata store
	capacity := &NodeCapacity{
		NodeID:           cm.nodeID,
		MaxDatabases:     cm.config.MaxDatabases,
		CurrentDatabases: currentDatabases,
		PortRangeHTTP:    announcement.PortRangeHTTP,
		PortRangeRaft:    announcement.PortRangeRaft,
		LastHealthCheck:  time.Now(),
		IsHealthy:        true,
	}
	cm.metadataStore.SetNode(capacity)
}

// monitorHealth monitors the health of active databases
func (cm *ClusterManager) monitorHealth() {
	// Use a default interval if the configured one is invalid
	interval := cm.discoveryConfig.HealthCheckInterval
	if interval <= 0 {
		interval = 10 * time.Second
		cm.logger.Warn("Invalid health check interval, using default",
			zap.Duration("configured", cm.discoveryConfig.HealthCheckInterval),
			zap.Duration("default", interval))
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-cm.ctx.Done():
			return
		case <-ticker.C:
			cm.checkDatabaseHealth()
		}
	}
}

// checkDatabaseHealth checks if all active databases are healthy
func (cm *ClusterManager) checkDatabaseHealth() {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	for dbName, instance := range cm.activeClusters {
		if !instance.IsRunning() {
			cm.logger.Warn("Database instance is not running",
				zap.String("database", dbName))
			// TODO: Implement recovery logic
		}
	}
}

// monitorIdleDatabases monitors for idle databases to hibernate
func (cm *ClusterManager) monitorIdleDatabases() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-cm.ctx.Done():
			return
		case <-ticker.C:
			cm.detectIdleDatabases()
		}
	}
}

// detectIdleDatabases detects idle databases and broadcasts idle notifications
func (cm *ClusterManager) detectIdleDatabases() {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	for dbName, instance := range cm.activeClusters {
		if instance.IsIdle(cm.config.HibernationTimeout) && instance.Status == StatusActive {
			cm.logger.Debug("Database is idle",
				zap.String("database", dbName),
				zap.Duration("idle_time", time.Since(instance.LastQuery)))

			// Broadcast idle notification
			notification := DatabaseIdleNotification{
				DatabaseName: dbName,
				NodeID:       cm.nodeID,
				LastActivity: instance.LastQuery,
			}

			msgData, err := MarshalMetadataMessage(MsgDatabaseIdleNotification, cm.nodeID, notification)
			if err != nil {
				cm.logger.Warn("Failed to marshal idle notification", zap.Error(err))
				continue
			}

			topic := "/debros/metadata/v1"
			if err := cm.pubsubAdapter.Publish(cm.ctx, topic, msgData); err != nil {
				cm.logger.Warn("Failed to publish idle notification", zap.Error(err))
			}
		}
	}
}

// reconcileOrphanedData checks for orphaned database directories
func (cm *ClusterManager) reconcileOrphanedData() {
	// Wait a bit for metadata to sync
	time.Sleep(10 * time.Second)

	cm.logger.Info("Starting orphaned data reconciliation")

	// Read data directory
	entries, err := os.ReadDir(cm.dataDir)
	if err != nil {
		cm.logger.Error("Failed to read data directory for reconciliation", zap.Error(err))
		return
	}

	orphanCount := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dbName := entry.Name()

		// Skip special directories
		if dbName == "rqlite" || dbName == "." || dbName == ".." {
			continue
		}

		// Check if this database exists in metadata
		dbMeta := cm.metadataStore.GetDatabase(dbName)
		if dbMeta == nil {
			// Orphaned directory - no metadata exists
			cm.logger.Warn("Found orphaned database directory",
				zap.String("database", dbName))
			orphanCount++

			// Delete the orphaned directory
			dbPath := filepath.Join(cm.dataDir, dbName)
			if err := os.RemoveAll(dbPath); err != nil {
				cm.logger.Error("Failed to remove orphaned directory",
					zap.String("database", dbName),
					zap.String("path", dbPath),
					zap.Error(err))
			} else {
				cm.logger.Info("Removed orphaned database directory",
					zap.String("database", dbName))
			}
			continue
		}

		// Check if this node is a member of the database
		isMember := false
		for _, nodeID := range dbMeta.NodeIDs {
			if nodeID == cm.nodeID {
				isMember = true
				break
			}
		}

		if !isMember {
			// This node is not a member - orphaned data
			cm.logger.Warn("Found database directory for non-member database",
				zap.String("database", dbName))
			orphanCount++

			dbPath := filepath.Join(cm.dataDir, dbName)
			if err := os.RemoveAll(dbPath); err != nil {
				cm.logger.Error("Failed to remove non-member directory",
					zap.String("database", dbName),
					zap.String("path", dbPath),
					zap.Error(err))
			} else {
				cm.logger.Info("Removed non-member database directory",
					zap.String("database", dbName))
			}
		}
	}

	cm.logger.Info("Orphaned data reconciliation complete",
		zap.Int("orphans_found", orphanCount))
}
