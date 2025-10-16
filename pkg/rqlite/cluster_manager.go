package rqlite

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/DeBrosOfficial/network/pkg/config"
	"github.com/DeBrosOfficial/network/pkg/pubsub"
	"github.com/rqlite/gorqlite"
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
	initializingDBs     map[string]bool // Track databases currently being initialized

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
		initializingDBs:     make(map[string]bool),
		ctx:                 ctx,
		cancel:              cancel,
	}
}

// Start starts the cluster manager
func (cm *ClusterManager) Start() error {
	cm.logger.Info("Starting cluster manager",
		zap.String("node_id", cm.nodeID),
		zap.Int("max_databases", cm.config.MaxDatabases))

	cm.metadataStore.SetLogger(cm.logger.With(zap.String("component", "metadata_store")))

	// Subscribe to metadata topic
	metadataTopic := "/debros/metadata/v1"
	if err := cm.pubsubAdapter.Subscribe(cm.ctx, metadataTopic, cm.handleMetadataMessage); err != nil {
		return fmt.Errorf("failed to subscribe to metadata topic: %w", err)
	}

	// Initialize system database
	if err := cm.initializeSystemDatabase(); err != nil {
		return fmt.Errorf("failed to initialize system database: %w", err)
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

	// Skip messages from self (except DATABASE_CREATE_CONFIRM which coordinator needs to process)
	if msg.NodeID == cm.nodeID && msg.Type != MsgDatabaseCreateConfirm {
		return nil
	}

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

	// Check if this node can also participate
	cm.mu.RLock()
	currentCount := len(cm.activeClusters)
	cm.mu.RUnlock()

	if currentCount < cm.config.MaxDatabases {
		// This node can host - add self-response
		systemDBName := cm.config.SystemDatabaseName
		if systemDBName == "" {
			systemDBName = "_system"
		}

		var selfPorts PortPair
		var portErr error

		if dbName == systemDBName && cm.config.SystemHTTPPort > 0 {
			// Try fixed ports for system database
			selfPorts = PortPair{
				HTTPPort: cm.config.SystemHTTPPort,
				RaftPort: cm.config.SystemRaftPort,
			}
			portErr = cm.portManager.AllocateSpecificPortPair(dbName, selfPorts)
			if portErr != nil {
				// Fixed ports unavailable - use dynamic
				cm.logger.Info("Fixed system ports unavailable on requester, using dynamic",
					zap.String("database", dbName))
				selfPorts, portErr = cm.portManager.AllocatePortPair(dbName)
			}
		} else {
			// Dynamic ports for non-system databases
			selfPorts, portErr = cm.portManager.AllocatePortPair(dbName)
		}

		if portErr == nil {
			// Add self as a candidate
			selfResponse := DatabaseCreateResponse{
				DatabaseName:   dbName,
				NodeID:         cm.nodeID,
				AvailablePorts: selfPorts,
			}
			coordinator.AddResponse(selfResponse)
			cm.logger.Debug("Added self as candidate for database",
				zap.String("database", dbName),
				zap.Int("http_port", selfPorts.HTTPPort),
				zap.Int("raft_port", selfPorts.RaftPort))
		}
	}

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

	// The requesting node is always the coordinator for its own request
	// This ensures deterministic coordination and avoids race conditions
	isCoordinator := true

	cm.logger.Info("This node is the requester and will coordinate",
		zap.String("database", dbName),
		zap.String("requester_node", cm.nodeID))

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

		// Create and broadcast metadata immediately
		metadata := &DatabaseMetadata{
			DatabaseName: dbName,
			NodeIDs:      make([]string, len(assignments)),
			PortMappings: make(map[string]PortPair),
			Status:       StatusInitializing,
			CreatedAt:    time.Now(),
			LastAccessed: time.Now(),
			LeaderNodeID: assignments[0].NodeID,
			Version:      1,
			VectorClock:  NewVectorClock(),
		}

		for i, node := range assignments {
			metadata.NodeIDs[i] = node.NodeID
			metadata.PortMappings[node.NodeID] = PortPair{
				HTTPPort: node.HTTPPort,
				RaftPort: node.RaftPort,
			}
		}

		// Store locally
		cm.metadataStore.SetDatabase(metadata)

		// Broadcast to all nodes
		syncMsg := MetadataSync{Metadata: metadata}
		syncData, _ := MarshalMetadataMessage(MsgMetadataSync, cm.nodeID, syncMsg)
		cm.pubsubAdapter.Publish(cm.ctx, topic, syncData)
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
	failedDatabases := make([]string, 0)
	for dbName, instance := range cm.activeClusters {
		if !instance.IsRunning() {
			failedDatabases = append(failedDatabases, dbName)
		}
	}
	cm.mu.RUnlock()

	// Attempt recovery for failed databases
	for _, dbName := range failedDatabases {
		cm.logger.Warn("Database instance is not running, attempting recovery",
			zap.String("database", dbName))

		// Get database metadata
		metadata := cm.metadataStore.GetDatabase(dbName)
		if metadata == nil {
			cm.logger.Error("Cannot recover database: metadata not found",
				zap.String("database", dbName))
			continue
		}

		// Check if this node is still supposed to host this database
		isMember := false
		for _, nodeID := range metadata.NodeIDs {
			if nodeID == cm.nodeID {
				isMember = true
				break
			}
		}

		if !isMember {
			cm.logger.Info("Node is no longer a member of database, removing from active clusters",
				zap.String("database", dbName))
			cm.mu.Lock()
			delete(cm.activeClusters, dbName)
			cm.mu.Unlock()
			continue
		}

		// Attempt to restart the database instance
		go cm.attemptDatabaseRecovery(dbName, metadata)
	}
}

// attemptDatabaseRecovery attempts to recover a failed database instance
func (cm *ClusterManager) attemptDatabaseRecovery(dbName string, metadata *DatabaseMetadata) {
	cm.logger.Info("Attempting database recovery",
		zap.String("database", dbName))

	// Check if we have quorum before attempting recovery
	if !cm.hasQuorum(metadata) {
		cm.logger.Warn("Cannot recover database: insufficient quorum",
			zap.String("database", dbName),
			zap.Int("required", (len(metadata.NodeIDs)/2)+1),
			zap.Int("active", len(cm.getActiveMembers(metadata))))
		return
	}

	// Determine if this node should be the leader
	isLeader := metadata.LeaderNodeID == cm.nodeID

	// Get ports for this database
	ports, exists := metadata.PortMappings[cm.nodeID]
	if !exists {
		cm.logger.Error("Cannot recover database: port mapping not found",
			zap.String("database", dbName),
			zap.String("node_id", cm.nodeID))
		return
	}

	// Create advertised addresses
	advHTTPAddr := fmt.Sprintf("%s:%d", cm.getAdvertiseAddress(), ports.HTTPPort)
	advRaftAddr := fmt.Sprintf("%s:%d", cm.getAdvertiseAddress(), ports.RaftPort)

	// Create new instance
	instance := NewRQLiteInstance(
		dbName,
		ports,
		cm.dataDir,
		advHTTPAddr,
		advRaftAddr,
		cm.logger.With(zap.String("database", dbName)),
	)

	// Determine join address if not leader
	var joinAddr string
	if !isLeader && len(metadata.NodeIDs) > 1 {
		// Get list of active members
		activeMembers := cm.getActiveMembers(metadata)

		// Prefer the leader if healthy
		for _, nodeID := range activeMembers {
			if nodeID == metadata.LeaderNodeID && nodeID != cm.nodeID {
				if leaderPorts, exists := metadata.PortMappings[nodeID]; exists {
					joinAddr = fmt.Sprintf("%s:%d", cm.getAdvertiseAddress(), leaderPorts.RaftPort)
					cm.logger.Info("Recovery: joining healthy leader",
						zap.String("database", dbName),
						zap.String("leader_node", nodeID),
						zap.String("join_address", joinAddr))
					break
				}
			}
		}

		// If leader not available, try any other healthy node
		if joinAddr == "" {
			for _, nodeID := range activeMembers {
				if nodeID != cm.nodeID {
					if nodePorts, exists := metadata.PortMappings[nodeID]; exists {
						joinAddr = fmt.Sprintf("%s:%d", cm.getAdvertiseAddress(), nodePorts.RaftPort)
						cm.logger.Info("Recovery: joining healthy follower",
							zap.String("database", dbName),
							zap.String("node", nodeID),
							zap.String("join_address", joinAddr))
						break
					}
				}
			}
		}

		// If no healthy nodes found, warn and fail
		if joinAddr == "" {
			cm.logger.Error("Cannot recover: no healthy nodes available to join",
				zap.String("database", dbName),
				zap.Int("total_nodes", len(metadata.NodeIDs)),
				zap.Int("active_nodes", len(activeMembers)))
			return
		}
	}

	// Check if instance has existing state
	if instance.hasExistingData() {
		wasInCluster := instance.wasInCluster()
		cm.logger.Info("Recovery: found existing RQLite state",
			zap.String("database", dbName),
			zap.Bool("is_leader", isLeader),
			zap.Bool("was_in_cluster", wasInCluster))

		// For leaders with existing cluster state, peer config will be cleared
		if isLeader && wasInCluster {
			cm.logger.Info("Recovery: leader will clear peer configuration for clean restart",
				zap.String("database", dbName))
		}

		// For followers, ensure join address is valid and will use join-as
		if !isLeader {
			if joinAddr == "" {
				cm.logger.Error("Cannot recover follower without join address",
					zap.String("database", dbName))
				return
			}
			if wasInCluster {
				cm.logger.Info("Recovery: follower will rejoin cluster as voter",
					zap.String("database", dbName),
					zap.String("join_address", joinAddr))
			}
		}
	}

	// Start the instance
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := instance.Start(ctx, isLeader, joinAddr); err != nil {
		cm.logger.Error("Database recovery failed",
			zap.String("database", dbName),
			zap.Error(err))
		return
	}

	// Update active clusters
	cm.mu.Lock()
	cm.activeClusters[dbName] = instance
	cm.mu.Unlock()

	// Update metadata status
	metadata.Status = StatusActive
	UpdateDatabaseMetadata(metadata, cm.nodeID)
	cm.metadataStore.SetDatabase(metadata)

	// Broadcast status update
	statusUpdate := DatabaseStatusUpdate{
		DatabaseName: dbName,
		NodeID:       cm.nodeID,
		Status:       StatusActive,
		HTTPPort:     ports.HTTPPort,
		RaftPort:     ports.RaftPort,
	}

	msgData, err := MarshalMetadataMessage(MsgDatabaseStatusUpdate, cm.nodeID, statusUpdate)
	if err != nil {
		cm.logger.Warn("Failed to marshal status update during recovery",
			zap.String("database", dbName),
			zap.Error(err))
	} else {
		topic := "/debros/metadata/v1"
		if err := cm.pubsubAdapter.Publish(cm.ctx, topic, msgData); err != nil {
			cm.logger.Warn("Failed to publish status update during recovery",
				zap.String("database", dbName),
				zap.Error(err))
		}
	}

	cm.logger.Info("Database recovery completed successfully",
		zap.String("database", dbName),
		zap.Bool("is_leader", isLeader))
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

// isNodeHealthy checks if a node is healthy and recently active
func (cm *ClusterManager) isNodeHealthy(nodeID string) bool {
	node := cm.metadataStore.GetNode(nodeID)
	if node == nil {
		return false
	}

	// Consider node stale if not heard from in 30 seconds
	staleDuration := 30 * time.Second
	if time.Since(node.LastHealthCheck) > staleDuration {
		return false
	}

	return node.IsHealthy
}

// hasQuorum checks if there are enough healthy nodes for a database
func (cm *ClusterManager) hasQuorum(metadata *DatabaseMetadata) bool {
	if metadata == nil {
		return false
	}

	activeNodes := 0
	for _, nodeID := range metadata.NodeIDs {
		if cm.isNodeHealthy(nodeID) || nodeID == cm.nodeID {
			activeNodes++
		}
	}

	requiredQuorum := (len(metadata.NodeIDs) / 2) + 1
	return activeNodes >= requiredQuorum
}

// getActiveMembers returns list of active member node IDs for a database
func (cm *ClusterManager) getActiveMembers(metadata *DatabaseMetadata) []string {
	activeMembers := make([]string, 0)
	for _, nodeID := range metadata.NodeIDs {
		if cm.isNodeHealthy(nodeID) || nodeID == cm.nodeID {
			activeMembers = append(activeMembers, nodeID)
		}
	}
	return activeMembers
}

// initializeSystemDatabase creates and starts the system database on this node
func (cm *ClusterManager) initializeSystemDatabase() error {
	systemDBName := cm.config.SystemDatabaseName
	if systemDBName == "" {
		systemDBName = "_system"
	}

	cm.logger.Info("Initializing system database",
		zap.String("database", systemDBName),
		zap.Int("replication_factor", cm.config.ReplicationFactor))

	// Wait longer for nodes to discover each other and for system DB metadata to propagate
	cm.logger.Info("Waiting for peer discovery before system database creation...")
	time.Sleep(15 * time.Second)

	// Check if system database already exists in metadata
	existingDB := cm.metadataStore.GetDatabase(systemDBName)
	if existingDB != nil {
		cm.logger.Info("System database already exists in metadata, checking local instance",
			zap.String("database", systemDBName),
			zap.Int("member_count", len(existingDB.NodeIDs)),
			zap.Strings("members", existingDB.NodeIDs))

		// Check quorum status
		hasQuorum := cm.hasQuorum(existingDB)
		cm.logger.Info("System database quorum status",
			zap.String("database", systemDBName),
			zap.Bool("has_quorum", hasQuorum),
			zap.Int("active_members", len(cm.getActiveMembers(existingDB))))

		// Check if this node is a member
		isMember := false
		for _, nodeID := range existingDB.NodeIDs {
			if nodeID == cm.nodeID {
				isMember = true
				break
			}
		}

		if !isMember {
			cm.logger.Info("This node is not a member of existing system database, skipping creation",
				zap.String("database", systemDBName))
			return nil
		}

		// Fall through to wait for activation
		cm.logger.Info("System database already exists in metadata, waiting for it to become active",
			zap.String("database", systemDBName))
	} else {
		// Only create if we don't see it in metadata yet
		cm.logger.Info("Creating system database",
			zap.String("database", systemDBName))

		// Try creating with retries (important for system database)
		maxRetries := 3
		var lastErr error
		for attempt := 1; attempt <= maxRetries; attempt++ {
			// Check again if it was created by another node (metadata may have been received via pubsub)
			existingDB = cm.metadataStore.GetDatabase(systemDBName)
			if existingDB != nil {
				cm.logger.Info("System database now exists (created by another node)",
					zap.Int("attempt", attempt))
				lastErr = nil
				break
			}

			lastErr = cm.CreateDatabase(systemDBName, cm.config.ReplicationFactor)
			if lastErr == nil {
				cm.logger.Info("System database creation initiated successfully")
				break
			}

			cm.logger.Warn("System database creation attempt failed",
				zap.Int("attempt", attempt),
				zap.Int("max_retries", maxRetries),
				zap.Error(lastErr))

			if attempt < maxRetries {
				// Wait before retry to allow more nodes to join and metadata to sync
				cm.logger.Info("Waiting before retry",
					zap.Duration("wait_time", 3*time.Second))
				time.Sleep(3 * time.Second)
			}
		}

		if lastErr != nil {
			cm.logger.Info("System database creation completed with errors, will wait for it to become active",
				zap.Error(lastErr),
				zap.String("note", "This node may not be selected for system database hosting"))
		}
	}

	// Wait for system database to become active (longer timeout)
	maxWait := 60 * time.Second
	checkInterval := 500 * time.Millisecond
	startTime := time.Now()

	for {
		if time.Since(startTime) > maxWait {
			// Don't fail startup - system database might be created later
			cm.logger.Warn("Timeout waiting for system database, continuing startup",
				zap.String("database", systemDBName),
				zap.Duration("waited", time.Since(startTime)))
			return nil // Return nil to allow node to start
		}

		cm.mu.RLock()
		instance, exists := cm.activeClusters[systemDBName]
		cm.mu.RUnlock()

		if exists && instance.Status == StatusActive {
			cm.logger.Info("System database is active",
				zap.String("database", systemDBName))

			// Run migrations if configured
			if cm.config.MigrationsPath != "" {
				if err := cm.runMigrations(systemDBName); err != nil {
					cm.logger.Error("Failed to run migrations on system database",
						zap.Error(err))
					// Don't fail startup, just log the error
				}
			}

			return nil
		}

		time.Sleep(checkInterval)
	}
}

// runMigrations executes SQL migrations on a database
func (cm *ClusterManager) runMigrations(dbName string) error {
	cm.logger.Info("Running migrations",
		zap.String("database", dbName),
		zap.String("migrations_path", cm.config.MigrationsPath))

	cm.mu.RLock()
	instance, exists := cm.activeClusters[dbName]
	cm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("database %s not found in active clusters", dbName)
	}

	conn := instance.Connection
	if conn == nil {
		return fmt.Errorf("no connection available for database %s", dbName)
	}

	// Read migration files
	files, err := filepath.Glob(filepath.Join(cm.config.MigrationsPath, "*.sql"))
	if err != nil {
		return fmt.Errorf("failed to read migration files: %w", err)
	}

	if len(files) == 0 {
		cm.logger.Info("No migration files found",
			zap.String("path", cm.config.MigrationsPath))
		return nil
	}

	// Sort files to ensure consistent order
	// Files are expected to be named like 001_initial.sql, 002_core.sql, etc.
	// filepath.Glob already returns them sorted

	cm.logger.Info("Found migration files",
		zap.Int("count", len(files)))

	// Execute each migration file
	for _, file := range files {
		cm.logger.Info("Executing migration",
			zap.String("file", filepath.Base(file)))

		content, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", file, err)
		}

		// Parse SQL content into individual statements
		sqlContent := string(content)

		// Split by semicolon but preserve multi-line statements
		// Simple approach: execute the whole file as one batch
		statements := []string{sqlContent}

		// Execute using WriteParameterized to avoid auto-transaction wrapping
		for _, stmt := range statements {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}

			_, err = conn.WriteOneParameterized(gorqlite.ParameterizedStatement{
				Query: stmt,
			})
			if err != nil {
				cm.logger.Error("Migration failed",
					zap.String("file", filepath.Base(file)),
					zap.Error(err))
				// Continue with other migrations even if one fails
				// (tables might already exist from previous runs)
				break
			}
		}

		cm.logger.Info("Migration completed",
			zap.String("file", filepath.Base(file)))
	}

	cm.logger.Info("All migrations completed",
		zap.String("database", dbName))

	return nil
}
