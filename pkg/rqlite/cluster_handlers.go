package rqlite

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// handleCreateRequest processes a database creation request
func (cm *ClusterManager) handleCreateRequest(msg *MetadataMessage) error {
	var req DatabaseCreateRequest
	if err := msg.UnmarshalPayload(&req); err != nil {
		return err
	}

	cm.logger.Info("Received database create request",
		zap.String("database", req.DatabaseName),
		zap.String("requester", req.RequesterNodeID),
		zap.Int("replication_factor", req.ReplicationFactor))

	// Check if we can host this database
	cm.mu.RLock()
	currentCount := len(cm.activeClusters)
	cm.mu.RUnlock()

	// Get system DB name for capacity check
	systemDBName := cm.config.SystemDatabaseName
	if systemDBName == "" {
		systemDBName = "_system"
	}

	// Bypass capacity check for system database (it replicates to all nodes)
	if req.DatabaseName != systemDBName && currentCount >= cm.config.MaxDatabases {
		cm.logger.Debug("Cannot host database: at capacity",
			zap.String("database", req.DatabaseName),
			zap.Int("current", currentCount),
			zap.Int("max", cm.config.MaxDatabases))
		return nil
	}

	// Allocate ports with sticky behavior
	var ports PortPair
	var err error

	// Try to load previously saved ports first (for sticky ports across restarts)
	savedPorts := LoadSavedPorts(cm.dataDir, req.DatabaseName, cm.logger)

	if req.DatabaseName == systemDBName && cm.config.SystemHTTPPort > 0 {
		// System database: MUST use fixed ports, do not fall back to dynamic
		ports = PortPair{
			HTTPPort: cm.config.SystemHTTPPort,
			RaftPort: cm.config.SystemRaftPort,
			Host:     cm.getAdvertiseAddress(),
		}
		err = cm.portManager.AllocateSpecificPortPair(req.DatabaseName, ports)
		if err != nil {
			// Fixed ports unavailable - DO NOT respond for system database
			cm.logger.Warn("System database requires fixed ports, but they are unavailable - not responding",
				zap.String("database", req.DatabaseName),
				zap.Int("attempted_http", ports.HTTPPort),
				zap.Int("attempted_raft", ports.RaftPort),
				zap.Error(err))
			return nil
		}
	} else if savedPorts != nil {
		// Try to reuse saved ports for sticky allocation
		ports = PortPair{
			HTTPPort: savedPorts.HTTPPort,
			RaftPort: savedPorts.RaftPort,
			Host:     cm.getAdvertiseAddress(),
		}
		err = cm.portManager.AllocateSpecificPortPair(req.DatabaseName, ports)
		if err != nil {
			// Saved ports unavailable, fall back to dynamic
			cm.logger.Info("Saved ports unavailable, allocating new ports",
				zap.String("database", req.DatabaseName),
				zap.Int("attempted_http", savedPorts.HTTPPort),
				zap.Int("attempted_raft", savedPorts.RaftPort))
			ports, err = cm.portManager.AllocatePortPair(req.DatabaseName)
		} else {
			cm.logger.Info("Reusing saved ports for database",
				zap.String("database", req.DatabaseName),
				zap.Int("http_port", ports.HTTPPort),
				zap.Int("raft_port", ports.RaftPort))
		}
	} else {
		// No saved ports, allocate dynamically
		ports, err = cm.portManager.AllocatePortPair(req.DatabaseName)
	}

	if err != nil {
		cm.logger.Warn("Cannot allocate ports for database",
			zap.String("database", req.DatabaseName),
			zap.Error(err))
		return nil
	}

	// Send response offering to host
	response := DatabaseCreateResponse{
		DatabaseName: req.DatabaseName,
		NodeID:       cm.nodeID,
		AvailablePorts: PortPair{
			HTTPPort: ports.HTTPPort,
			RaftPort: ports.RaftPort,
			Host:     cm.getAdvertiseAddress(),
		},
	}

	msgData, err := MarshalMetadataMessage(MsgDatabaseCreateResponse, cm.nodeID, response)
	if err != nil {
		cm.portManager.ReleasePortPair(ports)
		return fmt.Errorf("failed to marshal create response: %w", err)
	}

	topic := "/debros/metadata/v1"
	if err := cm.pubsubAdapter.Publish(cm.ctx, topic, msgData); err != nil {
		cm.portManager.ReleasePortPair(ports)
		return fmt.Errorf("failed to publish create response: %w", err)
	}

	cm.logger.Info("Sent database create response",
		zap.String("database", req.DatabaseName),
		zap.Int("http_port", ports.HTTPPort),
		zap.Int("raft_port", ports.RaftPort))

	return nil
}

// handleCreateResponse processes a database creation response
func (cm *ClusterManager) handleCreateResponse(msg *MetadataMessage) error {
	var response DatabaseCreateResponse
	if err := msg.UnmarshalPayload(&response); err != nil {
		return err
	}

	cm.logger.Debug("Received database create response",
		zap.String("database", response.DatabaseName),
		zap.String("node", response.NodeID))

	// Forward to coordinator registry
	cm.coordinatorRegistry.HandleCreateResponse(response)

	return nil
}

// handleCreateConfirm processes a database creation confirmation
func (cm *ClusterManager) handleCreateConfirm(msg *MetadataMessage) error {
	var confirm DatabaseCreateConfirm
	if err := msg.UnmarshalPayload(&confirm); err != nil {
		return err
	}

	cm.logger.Info("Received database create confirm",
		zap.String("database", confirm.DatabaseName),
		zap.String("coordinator", confirm.CoordinatorNodeID),
		zap.Int("nodes", len(confirm.SelectedNodes)))

	// Check if this node was selected first (before any locking)
	var myAssignment *NodeAssignment
	for i, node := range confirm.SelectedNodes {
		if node.NodeID == cm.nodeID {
			myAssignment = &confirm.SelectedNodes[i]
			break
		}
	}

	if myAssignment == nil {
		cm.logger.Debug("Not selected for this database",
			zap.String("database", confirm.DatabaseName))
		return nil
	}

	// Use atomic check-and-set to prevent race conditions
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Check if database already exists or is being initialized (atomic check)
	_, alreadyActive := cm.activeClusters[confirm.DatabaseName]
	_, alreadyInitializing := cm.initializingDBs[confirm.DatabaseName]

	if alreadyActive || alreadyInitializing {
		cm.logger.Debug("Database already active or initializing on this node, ignoring confirmation",
			zap.String("database", confirm.DatabaseName),
			zap.Bool("active", alreadyActive),
			zap.Bool("initializing", alreadyInitializing))
		return nil
	}

	// Atomically mark database as initializing to prevent duplicate confirmations
	cm.initializingDBs[confirm.DatabaseName] = true

	cm.logger.Info("Selected to host database",
		zap.String("database", confirm.DatabaseName),
		zap.String("role", myAssignment.Role))

	// Create database metadata
	portMappings := make(map[string]PortPair)
	nodeIDs := make([]string, len(confirm.SelectedNodes))
	for i, node := range confirm.SelectedNodes {
		nodeIDs[i] = node.NodeID
		portMappings[node.NodeID] = PortPair{
			HTTPPort: node.HTTPPort,
			RaftPort: node.RaftPort,
			Host:     node.Host,
		}
	}

	metadata := &DatabaseMetadata{
		DatabaseName: confirm.DatabaseName,
		NodeIDs:      nodeIDs,
		PortMappings: portMappings,
		Status:       StatusInitializing,
		CreatedAt:    time.Now(),
		LastAccessed: time.Now(),
		LeaderNodeID: confirm.SelectedNodes[0].NodeID, // First node is leader
		Version:      1,
		VectorClock:  NewVectorClock(),
	}

	// Update vector clock
	UpdateDatabaseMetadata(metadata, cm.nodeID)

	// Store metadata
	cm.metadataStore.SetDatabase(metadata)

	// Start the RQLite instance
	go cm.startDatabaseInstance(metadata, myAssignment.Role == "leader")

	return nil
}

// startDatabaseInstance starts a database instance on this node
func (cm *ClusterManager) startDatabaseInstance(metadata *DatabaseMetadata, isLeader bool) {
	ports := metadata.PortMappings[cm.nodeID]

	// Create advertised addresses
	advHTTPAddr := fmt.Sprintf("%s:%d", cm.getAdvertiseAddress(), ports.HTTPPort)
	advRaftAddr := fmt.Sprintf("%s:%d", cm.getAdvertiseAddress(), ports.RaftPort)

	// Create instance
	instance := NewRQLiteInstance(
		metadata.DatabaseName,
		ports,
		cm.dataDir,
		advHTTPAddr,
		advRaftAddr,
		cm.logger,
	)

	// Determine join address (if follower)
	var joinAddr string
	if !isLeader && len(metadata.NodeIDs) > 0 {
		// Join to the leader
		leaderNodeID := metadata.LeaderNodeID
		if leaderPorts, exists := metadata.PortMappings[leaderNodeID]; exists {
			// Use leader's host if available, fallback to this node's advertise address
			host := leaderPorts.Host
			if host == "" {
				host = cm.getAdvertiseAddress()
			}
			joinAddr = fmt.Sprintf("%s:%d", host, leaderPorts.RaftPort)
			cm.logger.Info("Follower joining leader",
				zap.String("database", metadata.DatabaseName),
				zap.String("leader_node", leaderNodeID),
				zap.String("join_address", joinAddr),
				zap.String("leader_host", host),
				zap.Int("leader_raft_port", leaderPorts.RaftPort))
		} else {
			cm.logger.Error("Leader node not found in port mappings",
				zap.String("database", metadata.DatabaseName),
				zap.String("leader_node", leaderNodeID))
		}
	}

	// For followers with existing data, ensure we have a join address
	if !isLeader && instance.hasExistingData() {
		if joinAddr == "" {
			cm.logger.Error("Follower has existing data but no join address available",
				zap.String("database", metadata.DatabaseName))
			// Clear initializing flag
			cm.mu.Lock()
			delete(cm.initializingDBs, metadata.DatabaseName)
			cm.mu.Unlock()
			return
		}
		cm.logger.Info("Follower restarting with existing data, will rejoin cluster",
			zap.String("database", metadata.DatabaseName),
			zap.String("join_address", joinAddr))
	}

	// Start the instance with appropriate timeout
	timeout := 60 * time.Second
	if isLeader {
		timeout = 90 * time.Second // Leaders need more time for bootstrap
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := instance.Start(ctx, isLeader, joinAddr); err != nil {
		cm.logger.Error("Failed to start database instance",
			zap.String("database", metadata.DatabaseName),
			zap.Bool("is_leader", isLeader),
			zap.Error(err))

		// Clear initializing flag on failure
		cm.mu.Lock()
		delete(cm.initializingDBs, metadata.DatabaseName)
		cm.mu.Unlock()

		// Broadcast failure status
		cm.broadcastStatusUpdate(metadata.DatabaseName, StatusInitializing)
		return
	}

	// Save ports for sticky allocation on restart
	if err := SavePorts(cm.dataDir, metadata.DatabaseName, ports, cm.logger); err != nil {
		cm.logger.Warn("Failed to save ports for database",
			zap.String("database", metadata.DatabaseName),
			zap.Error(err))
		// Don't fail startup, just log the warning
	}

	// For followers, start background SQL readiness check
	if !isLeader {
		instance.StartBackgroundSQLReadinessCheck(cm.ctx, func() {
			cm.logger.Info("Follower SQL became ready",
				zap.String("database", metadata.DatabaseName))
		})
	}

	// Store active instance and clear initializing flag
	cm.mu.Lock()
	cm.activeClusters[metadata.DatabaseName] = instance
	delete(cm.initializingDBs, metadata.DatabaseName)
	cm.mu.Unlock()

	// Broadcast active status
	cm.broadcastStatusUpdate(metadata.DatabaseName, StatusActive)

	cm.logger.Info("Database instance started and active",
		zap.String("database", metadata.DatabaseName),
		zap.Bool("is_leader", isLeader))

	// Broadcast metadata sync to all nodes
	syncMsg := MetadataSync{Metadata: metadata}
	syncData, err := MarshalMetadataMessage(MsgMetadataSync, cm.nodeID, syncMsg)
	if err == nil {
		topic := "/debros/metadata/v1"
		if err := cm.pubsubAdapter.Publish(cm.ctx, topic, syncData); err != nil {
			cm.logger.Warn("Failed to broadcast metadata sync",
				zap.String("database", metadata.DatabaseName),
				zap.Error(err))
		} else {
			cm.logger.Debug("Broadcasted metadata sync",
				zap.String("database", metadata.DatabaseName))
		}
	}
}

// handleStatusUpdate processes database status updates
func (cm *ClusterManager) handleStatusUpdate(msg *MetadataMessage) error {
	var update DatabaseStatusUpdate
	if err := msg.UnmarshalPayload(&update); err != nil {
		return err
	}

	cm.logger.Debug("Received status update",
		zap.String("database", update.DatabaseName),
		zap.String("node", update.NodeID),
		zap.String("status", string(update.Status)))

	// Update metadata
	if metadata := cm.metadataStore.GetDatabase(update.DatabaseName); metadata != nil {
		metadata.Status = update.Status
		metadata.LastAccessed = time.Now()
		cm.metadataStore.SetDatabase(metadata)
	}

	return nil
}

// handleCapacityAnnouncement processes node capacity announcements
func (cm *ClusterManager) handleCapacityAnnouncement(msg *MetadataMessage) error {
	var announcement NodeCapacityAnnouncement
	if err := msg.UnmarshalPayload(&announcement); err != nil {
		return err
	}

	capacity := &NodeCapacity{
		NodeID:           announcement.NodeID,
		MaxDatabases:     announcement.MaxDatabases,
		CurrentDatabases: announcement.CurrentDatabases,
		PortRangeHTTP:    announcement.PortRangeHTTP,
		PortRangeRaft:    announcement.PortRangeRaft,
		LastHealthCheck:  time.Now(),
		IsHealthy:        true,
	}

	cm.metadataStore.SetNode(capacity)

	return nil
}

// handleHealthPing processes health ping messages
func (cm *ClusterManager) handleHealthPing(msg *MetadataMessage) error {
	var ping NodeHealthPing
	if err := msg.UnmarshalPayload(&ping); err != nil {
		return err
	}

	// Respond with pong
	pong := NodeHealthPong{
		NodeID:   cm.nodeID,
		Healthy:  true,
		PingFrom: ping.NodeID,
	}

	msgData, err := MarshalMetadataMessage(MsgNodeHealthPong, cm.nodeID, pong)
	if err != nil {
		return err
	}

	topic := "/debros/metadata/v1"
	return cm.pubsubAdapter.Publish(cm.ctx, topic, msgData)
}

// handleMetadataSync processes metadata synchronization messages
func (cm *ClusterManager) handleMetadataSync(msg *MetadataMessage) error {
	var sync MetadataSync
	if err := msg.UnmarshalPayload(&sync); err != nil {
		return err
	}

	if sync.Metadata == nil {
		return nil
	}

	cm.logger.Debug("Received metadata sync",
		zap.String("database", sync.Metadata.DatabaseName),
		zap.String("from_node", msg.NodeID))

	// Check if we need to update local metadata
	existing := cm.metadataStore.GetDatabase(sync.Metadata.DatabaseName)
	if existing == nil {
		// New database we didn't know about
		cm.metadataStore.SetDatabase(sync.Metadata)
		cm.logger.Info("Learned about new database via sync",
			zap.String("database", sync.Metadata.DatabaseName),
			zap.Strings("node_ids", sync.Metadata.NodeIDs))
		return nil
	}

	// Resolve conflict if versions differ
	winner := ResolveConflict(existing, sync.Metadata)
	if winner != existing {
		cm.metadataStore.SetDatabase(winner)
		cm.logger.Info("Updated database metadata via sync",
			zap.String("database", sync.Metadata.DatabaseName),
			zap.Uint64("new_version", winner.Version))
	}

	return nil
}

// handleChecksumRequest processes checksum requests
func (cm *ClusterManager) handleChecksumRequest(msg *MetadataMessage) error {
	var req MetadataChecksumRequest
	if err := msg.UnmarshalPayload(&req); err != nil {
		return err
	}

	// Compute checksums for all databases
	checksums := ComputeFullStateChecksum(cm.metadataStore)

	// Send response
	response := MetadataChecksumResponse{
		RequestID: req.RequestID,
		Checksums: checksums,
	}

	msgData, err := MarshalMetadataMessage(MsgMetadataChecksumRes, cm.nodeID, response)
	if err != nil {
		return err
	}

	topic := "/debros/metadata/v1"
	return cm.pubsubAdapter.Publish(cm.ctx, topic, msgData)
}

// handleChecksumResponse processes checksum responses
func (cm *ClusterManager) handleChecksumResponse(msg *MetadataMessage) error {
	var response MetadataChecksumResponse
	if err := msg.UnmarshalPayload(&response); err != nil {
		return err
	}

	// Compare with local checksums
	localChecksums := ComputeFullStateChecksum(cm.metadataStore)
	localMap := make(map[string]MetadataChecksum)
	for _, cs := range localChecksums {
		localMap[cs.DatabaseName] = cs
	}

	// Check for differences
	for _, remoteCS := range response.Checksums {
		localCS, exists := localMap[remoteCS.DatabaseName]
		if !exists {
			// Database we don't know about - request full metadata
			cm.logger.Info("Discovered database via checksum",
				zap.String("database", remoteCS.DatabaseName))
			// TODO: Request full metadata for this database
			continue
		}

		if localCS.Hash != remoteCS.Hash {
			cm.logger.Info("Database metadata diverged",
				zap.String("database", remoteCS.DatabaseName))
			// TODO: Request full metadata for this database
		}
	}

	return nil
}

// broadcastStatusUpdate broadcasts a status update for a database
func (cm *ClusterManager) broadcastStatusUpdate(dbName string, status DatabaseStatus) {
	cm.mu.RLock()
	instance := cm.activeClusters[dbName]
	cm.mu.RUnlock()

	update := DatabaseStatusUpdate{
		DatabaseName: dbName,
		NodeID:       cm.nodeID,
		Status:       status,
	}

	if instance != nil {
		update.HTTPPort = instance.HTTPPort
		update.RaftPort = instance.RaftPort
	}

	msgData, err := MarshalMetadataMessage(MsgDatabaseStatusUpdate, cm.nodeID, update)
	if err != nil {
		cm.logger.Warn("Failed to marshal status update", zap.Error(err))
		return
	}

	topic := "/debros/metadata/v1"
	if err := cm.pubsubAdapter.Publish(cm.ctx, topic, msgData); err != nil {
		cm.logger.Warn("Failed to publish status update", zap.Error(err))
	}
}

// getAdvertiseAddress returns the advertise address for this node
func (cm *ClusterManager) getAdvertiseAddress() string {
	if cm.discoveryConfig.HttpAdvAddress != "" {
		// Extract just the host part (remove port if present)
		addr := cm.discoveryConfig.HttpAdvAddress
		if idx := len(addr) - 1; idx >= 0 {
			for i := len(addr) - 1; i >= 0; i-- {
				if addr[i] == ':' {
					return addr[:i]
				}
			}
		}
		return addr
	}
	return "0.0.0.0"
}

// handleIdleNotification processes idle notifications from other nodes
func (cm *ClusterManager) handleIdleNotification(msg *MetadataMessage) error {
	var notification DatabaseIdleNotification
	if err := msg.UnmarshalPayload(&notification); err != nil {
		return err
	}

	cm.logger.Debug("Received idle notification",
		zap.String("database", notification.DatabaseName),
		zap.String("from_node", notification.NodeID))

	// Get database metadata
	dbMeta := cm.metadataStore.GetDatabase(notification.DatabaseName)
	if dbMeta == nil {
		cm.logger.Debug("Idle notification for unknown database",
			zap.String("database", notification.DatabaseName))
		return nil
	}

	// Track idle count (simple approach: if we see idle from all nodes, coordinate shutdown)
	// In production, this would use a more sophisticated quorum mechanism
	idleCount := 0
	for _, nodeID := range dbMeta.NodeIDs {
		if nodeID == notification.NodeID || nodeID == cm.nodeID {
			idleCount++
		}
	}

	// If all nodes are idle, coordinate shutdown
	if idleCount >= len(dbMeta.NodeIDs) {
		cm.logger.Info("All nodes idle for database, coordinating shutdown",
			zap.String("database", notification.DatabaseName))

		// Elect coordinator
		coordinator := SelectCoordinator(dbMeta.NodeIDs)
		if coordinator == cm.nodeID {
			// This node is coordinator, initiate shutdown
			shutdown := DatabaseShutdownCoordinated{
				DatabaseName: notification.DatabaseName,
				ShutdownTime: time.Now().Add(5 * time.Second), // Grace period
			}

			msgData, err := MarshalMetadataMessage(MsgDatabaseShutdownCoordinated, cm.nodeID, shutdown)
			if err != nil {
				return fmt.Errorf("failed to marshal shutdown message: %w", err)
			}

			topic := "/debros/metadata/v1"
			if err := cm.pubsubAdapter.Publish(cm.ctx, topic, msgData); err != nil {
				return fmt.Errorf("failed to publish shutdown message: %w", err)
			}

			cm.logger.Info("Coordinated shutdown message sent",
				zap.String("database", notification.DatabaseName))
		}
	}

	return nil
}

// handleShutdownCoordinated processes coordinated shutdown messages
func (cm *ClusterManager) handleShutdownCoordinated(msg *MetadataMessage) error {
	var shutdown DatabaseShutdownCoordinated
	if err := msg.UnmarshalPayload(&shutdown); err != nil {
		return err
	}

	cm.logger.Info("Received coordinated shutdown",
		zap.String("database", shutdown.DatabaseName),
		zap.Time("shutdown_time", shutdown.ShutdownTime))

	// Get database metadata
	dbMeta := cm.metadataStore.GetDatabase(shutdown.DatabaseName)
	if dbMeta == nil {
		cm.logger.Debug("Shutdown for unknown database",
			zap.String("database", shutdown.DatabaseName))
		return nil
	}

	// Check if this node is a member
	isMember := false
	for _, nodeID := range dbMeta.NodeIDs {
		if nodeID == cm.nodeID {
			isMember = true
			break
		}
	}

	if !isMember {
		return nil
	}

	// Wait until shutdown time
	waitDuration := time.Until(shutdown.ShutdownTime)
	if waitDuration > 0 {
		cm.logger.Debug("Waiting for shutdown time",
			zap.String("database", shutdown.DatabaseName),
			zap.Duration("wait", waitDuration))
		time.Sleep(waitDuration)
	}

	// Stop the instance
	cm.mu.Lock()
	instance, exists := cm.activeClusters[shutdown.DatabaseName]
	if exists {
		cm.logger.Info("Stopping database instance for hibernation",
			zap.String("database", shutdown.DatabaseName))

		if err := instance.Stop(); err != nil {
			cm.logger.Error("Failed to stop instance", zap.Error(err))
			cm.mu.Unlock()
			return err
		}

		// Free ports
		ports := PortPair{HTTPPort: instance.HTTPPort, RaftPort: instance.RaftPort}
		cm.portManager.ReleasePortPair(ports)

		// Remove from active clusters
		delete(cm.activeClusters, shutdown.DatabaseName)
	}
	cm.mu.Unlock()

	// Update metadata status to hibernating
	dbMeta.Status = StatusHibernating
	dbMeta.LastAccessed = time.Now()
	cm.metadataStore.SetDatabase(dbMeta)

	// Broadcast status update
	cm.broadcastStatusUpdate(shutdown.DatabaseName, StatusHibernating)

	cm.logger.Info("Database hibernated successfully",
		zap.String("database", shutdown.DatabaseName))

	return nil
}

// handleWakeupRequest processes wake-up requests for hibernating databases
func (cm *ClusterManager) handleWakeupRequest(msg *MetadataMessage) error {
	var wakeup DatabaseWakeupRequest
	if err := msg.UnmarshalPayload(&wakeup); err != nil {
		return err
	}

	cm.logger.Info("Received wakeup request",
		zap.String("database", wakeup.DatabaseName),
		zap.String("requester", wakeup.RequesterNodeID))

	// Get database metadata
	dbMeta := cm.metadataStore.GetDatabase(wakeup.DatabaseName)
	if dbMeta == nil {
		cm.logger.Warn("Wakeup request for unknown database",
			zap.String("database", wakeup.DatabaseName))
		return nil
	}

	// Check if database is hibernating
	if dbMeta.Status != StatusHibernating {
		cm.logger.Debug("Database not hibernating, ignoring wakeup",
			zap.String("database", wakeup.DatabaseName),
			zap.String("status", string(dbMeta.Status)))
		return nil
	}

	// Check if this node is a member
	isMember := false
	for _, nodeID := range dbMeta.NodeIDs {
		if nodeID == cm.nodeID {
			isMember = true
			break
		}
	}

	if !isMember {
		return nil
	}

	// Update status to waking
	dbMeta.Status = StatusWaking
	dbMeta.LastAccessed = time.Now()
	cm.metadataStore.SetDatabase(dbMeta)

	// Start the instance
	go cm.wakeupDatabase(wakeup.DatabaseName, dbMeta)

	return nil
}

// wakeupDatabase starts a hibernating database
func (cm *ClusterManager) wakeupDatabase(dbName string, dbMeta *DatabaseMetadata) {
	cm.logger.Info("Waking up database", zap.String("database", dbName))

	// Get port mapping for this node
	ports, exists := dbMeta.PortMappings[cm.nodeID]
	if !exists {
		cm.logger.Error("No port mapping found for node",
			zap.String("database", dbName),
			zap.String("node", cm.nodeID))
		return
	}

	// Try to allocate the same ports (or new ones if taken)
	allocatedPorts := ports
	if cm.portManager.IsPortAllocated(ports.HTTPPort) || cm.portManager.IsPortAllocated(ports.RaftPort) {
		cm.logger.Warn("Original ports taken, allocating new ones",
			zap.String("database", dbName))
		newPorts, err := cm.portManager.AllocatePortPair(dbName)
		if err != nil {
			cm.logger.Error("Failed to allocate ports for wakeup", zap.Error(err))
			return
		}
		allocatedPorts = newPorts
		// Update port mapping in metadata
		dbMeta.PortMappings[cm.nodeID] = allocatedPorts
		cm.metadataStore.SetDatabase(dbMeta)
	} else {
		// Mark ports as allocated
		if err := cm.portManager.AllocateSpecificPorts(dbName, ports); err != nil {
			cm.logger.Error("Failed to allocate specific ports", zap.Error(err))
			return
		}
	}

	// Determine join address (first node in the list)
	joinAddr := ""
	if len(dbMeta.NodeIDs) > 0 && dbMeta.NodeIDs[0] != cm.nodeID {
		firstNodePorts := dbMeta.PortMappings[dbMeta.NodeIDs[0]]
		// Use first node's host if available, fallback to this node's advertise address
		host := firstNodePorts.Host
		if host == "" {
			host = cm.getAdvertiseAddress()
		}
		joinAddr = fmt.Sprintf("%s:%d", host, firstNodePorts.RaftPort)
	}

	// Create and start instance
	instance := NewRQLiteInstance(
		dbName,
		allocatedPorts,
		cm.dataDir,
		cm.getAdvertiseAddress(),
		cm.getAdvertiseAddress(),
		cm.logger,
	)

	// Determine if this is the leader (first node)
	isLeader := len(dbMeta.NodeIDs) > 0 && dbMeta.NodeIDs[0] == cm.nodeID

	if err := instance.Start(cm.ctx, isLeader, joinAddr); err != nil {
		cm.logger.Error("Failed to start instance during wakeup", zap.Error(err))
		cm.portManager.ReleasePortPair(allocatedPorts)
		return
	}

	// Save ports for sticky allocation on restart
	if err := SavePorts(cm.dataDir, dbName, allocatedPorts, cm.logger); err != nil {
		cm.logger.Warn("Failed to save ports for database during wakeup",
			zap.String("database", dbName),
			zap.Error(err))
	}

	// Add to active clusters
	cm.mu.Lock()
	cm.activeClusters[dbName] = instance
	cm.mu.Unlock()

	// Update metadata status to active
	dbMeta.Status = StatusActive
	dbMeta.LastAccessed = time.Now()
	cm.metadataStore.SetDatabase(dbMeta)

	// Broadcast status update
	cm.broadcastStatusUpdate(dbName, StatusActive)

	cm.logger.Info("Database woke up successfully", zap.String("database", dbName))
}

// handleNodeReplacementNeeded processes requests to replace a failed node
func (cm *ClusterManager) handleNodeReplacementNeeded(msg *MetadataMessage) error {
	var replacement NodeReplacementNeeded
	if err := msg.UnmarshalPayload(&replacement); err != nil {
		return err
	}

	cm.logger.Info("Received node replacement needed",
		zap.String("database", replacement.DatabaseName),
		zap.String("failed_node", replacement.FailedNodeID))

	// Get database metadata
	dbMeta := cm.metadataStore.GetDatabase(replacement.DatabaseName)
	if dbMeta == nil {
		cm.logger.Warn("Replacement needed for unknown database",
			zap.String("database", replacement.DatabaseName))
		return nil
	}

	// Check if we're eligible to replace (not at capacity and healthy)
	nodeCapacity := cm.metadataStore.GetNode(cm.nodeID)
	if nodeCapacity == nil || nodeCapacity.CurrentDatabases >= nodeCapacity.MaxDatabases {
		cm.logger.Debug("Not eligible for replacement - at capacity",
			zap.String("database", replacement.DatabaseName))
		return nil
	}

	// Check if we're not already a member
	for _, nodeID := range dbMeta.NodeIDs {
		if nodeID == cm.nodeID {
			cm.logger.Debug("Already a member of this database",
				zap.String("database", replacement.DatabaseName))
			return nil
		}
	}

	// Allocate ports for potential replacement
	ports, err := cm.portManager.AllocatePortPair(replacement.DatabaseName)
	if err != nil {
		cm.logger.Warn("Cannot allocate ports for replacement",
			zap.String("database", replacement.DatabaseName),
			zap.Error(err))
		return nil
	}

	// Send replacement offer
	response := NodeReplacementOffer{
		DatabaseName:   replacement.DatabaseName,
		NodeID:         cm.nodeID,
		AvailablePorts: ports,
	}

	msgData, err := MarshalMetadataMessage(MsgNodeReplacementOffer, cm.nodeID, response)
	if err != nil {
		cm.portManager.ReleasePortPair(ports)
		return fmt.Errorf("failed to marshal replacement offer: %w", err)
	}

	topic := "/debros/metadata/v1"
	if err := cm.pubsubAdapter.Publish(cm.ctx, topic, msgData); err != nil {
		cm.portManager.ReleasePortPair(ports)
		return fmt.Errorf("failed to publish replacement offer: %w", err)
	}

	cm.logger.Info("Sent replacement offer",
		zap.String("database", replacement.DatabaseName))

	return nil
}

// handleNodeReplacementOffer processes offers from nodes to replace a failed node
func (cm *ClusterManager) handleNodeReplacementOffer(msg *MetadataMessage) error {
	var offer NodeReplacementOffer
	if err := msg.UnmarshalPayload(&offer); err != nil {
		return err
	}

	cm.logger.Debug("Received replacement offer",
		zap.String("database", offer.DatabaseName),
		zap.String("from_node", offer.NodeID))

	// This would be handled by the coordinator who initiated the replacement request
	// For now, we'll implement a simple first-come-first-served approach
	// In production, this would involve collecting offers and selecting the best node

	dbMeta := cm.metadataStore.GetDatabase(offer.DatabaseName)
	if dbMeta == nil {
		return nil
	}

	// Check if we're a surviving member and should coordinate
	isMember := false
	for _, nodeID := range dbMeta.NodeIDs {
		if nodeID == cm.nodeID {
			isMember = true
			break
		}
	}

	if !isMember {
		return nil
	}

	// Simple approach: accept first offer
	// In production: collect offers, select based on capacity/health
	cm.logger.Info("Accepting replacement offer",
		zap.String("database", offer.DatabaseName),
		zap.String("new_node", offer.NodeID))

	// Find a surviving node to provide join address
	var joinAddr string
	for _, nodeID := range dbMeta.NodeIDs {
		if nodeID != cm.nodeID {
			continue // Skip failed nodes (would need proper tracking)
		}
		ports := dbMeta.PortMappings[nodeID]
		// Use node's host if available, fallback to this node's advertise address
		host := ports.Host
		if host == "" {
			host = cm.getAdvertiseAddress()
		}
		joinAddr = fmt.Sprintf("%s:%d", host, ports.RaftPort)
		break
	}

	// Broadcast confirmation
	confirm := NodeReplacementConfirm{
		DatabaseName:   offer.DatabaseName,
		NewNodeID:      offer.NodeID,
		ReplacedNodeID: "", // Would track which node failed
		NewNodePorts:   offer.AvailablePorts,
		JoinAddress:    joinAddr,
	}

	msgData, err := MarshalMetadataMessage(MsgNodeReplacementConfirm, cm.nodeID, confirm)
	if err != nil {
		return fmt.Errorf("failed to marshal replacement confirm: %w", err)
	}

	topic := "/debros/metadata/v1"
	if err := cm.pubsubAdapter.Publish(cm.ctx, topic, msgData); err != nil {
		return fmt.Errorf("failed to publish replacement confirm: %w", err)
	}

	return nil
}

// handleNodeReplacementConfirm processes confirmation of a replacement node
func (cm *ClusterManager) handleNodeReplacementConfirm(msg *MetadataMessage) error {
	var confirm NodeReplacementConfirm
	if err := msg.UnmarshalPayload(&confirm); err != nil {
		return err
	}

	cm.logger.Info("Received node replacement confirm",
		zap.String("database", confirm.DatabaseName),
		zap.String("new_node", confirm.NewNodeID),
		zap.String("replaced_node", confirm.ReplacedNodeID))

	// Get database metadata
	dbMeta := cm.metadataStore.GetDatabase(confirm.DatabaseName)
	if dbMeta == nil {
		cm.logger.Warn("Replacement confirm for unknown database",
			zap.String("database", confirm.DatabaseName))
		return nil
	}

	// Update metadata: replace old node with new node
	newNodes := make([]string, 0, len(dbMeta.NodeIDs))
	for _, nodeID := range dbMeta.NodeIDs {
		if nodeID == confirm.ReplacedNodeID {
			newNodes = append(newNodes, confirm.NewNodeID)
		} else {
			newNodes = append(newNodes, nodeID)
		}
	}
	dbMeta.NodeIDs = newNodes

	// Update port mappings
	delete(dbMeta.PortMappings, confirm.ReplacedNodeID)
	dbMeta.PortMappings[confirm.NewNodeID] = confirm.NewNodePorts

	cm.metadataStore.SetDatabase(dbMeta)

	// If we're the new node, start the instance and join
	if confirm.NewNodeID == cm.nodeID {
		cm.logger.Info("Starting as replacement node",
			zap.String("database", confirm.DatabaseName))

		go cm.startReplacementInstance(confirm.DatabaseName, confirm.NewNodePorts, confirm.JoinAddress)
	}

	return nil
}

// startReplacementInstance starts an instance as a replacement for a failed node
func (cm *ClusterManager) startReplacementInstance(dbName string, ports PortPair, joinAddr string) {
	cm.logger.Info("Starting replacement instance",
		zap.String("database", dbName),
		zap.String("join_address", joinAddr))

	// Create instance
	instance := NewRQLiteInstance(
		dbName,
		ports,
		cm.dataDir,
		cm.getAdvertiseAddress(),
		cm.getAdvertiseAddress(),
		cm.logger,
	)

	// Start with join address (always joining existing cluster)
	if err := instance.Start(cm.ctx, false, joinAddr); err != nil {
		cm.logger.Error("Failed to start replacement instance", zap.Error(err))
		cm.portManager.ReleasePortPair(ports)
		return
	}

	// Save ports for sticky allocation on restart
	if err := SavePorts(cm.dataDir, dbName, ports, cm.logger); err != nil {
		cm.logger.Warn("Failed to save ports for replacement instance",
			zap.String("database", dbName),
			zap.Error(err))
	}

	// Add to active clusters
	cm.mu.Lock()
	cm.activeClusters[dbName] = instance
	cm.mu.Unlock()

	// Broadcast active status
	cm.broadcastStatusUpdate(dbName, StatusActive)

	cm.logger.Info("Replacement instance started successfully",
		zap.String("database", dbName))
}
