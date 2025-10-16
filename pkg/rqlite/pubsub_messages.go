package rqlite

import (
	"encoding/json"
	"time"
)

// MessageType represents the type of metadata message
type MessageType string

const (
	// Database lifecycle
	MsgDatabaseCreateRequest  MessageType = "DATABASE_CREATE_REQUEST"
	MsgDatabaseCreateResponse MessageType = "DATABASE_CREATE_RESPONSE"
	MsgDatabaseCreateConfirm  MessageType = "DATABASE_CREATE_CONFIRM"
	MsgDatabaseStatusUpdate   MessageType = "DATABASE_STATUS_UPDATE"
	MsgDatabaseDelete         MessageType = "DATABASE_DELETE"

	// Hibernation
	MsgDatabaseIdleNotification    MessageType = "DATABASE_IDLE_NOTIFICATION"
	MsgDatabaseShutdownCoordinated MessageType = "DATABASE_SHUTDOWN_COORDINATED"
	MsgDatabaseWakeupRequest       MessageType = "DATABASE_WAKEUP_REQUEST"

	// Node management
	MsgNodeCapacityAnnouncement MessageType = "NODE_CAPACITY_ANNOUNCEMENT"
	MsgNodeHealthPing           MessageType = "NODE_HEALTH_PING"
	MsgNodeHealthPong           MessageType = "NODE_HEALTH_PONG"

	// Failure handling
	MsgNodeReplacementNeeded  MessageType = "NODE_REPLACEMENT_NEEDED"
	MsgNodeReplacementOffer   MessageType = "NODE_REPLACEMENT_OFFER"
	MsgNodeReplacementConfirm MessageType = "NODE_REPLACEMENT_CONFIRM"
	MsgDatabaseCleanup        MessageType = "DATABASE_CLEANUP"

	// Gossip
	MsgMetadataSync        MessageType = "METADATA_SYNC"
	MsgMetadataChecksumReq MessageType = "METADATA_CHECKSUM_REQUEST"
	MsgMetadataChecksumRes MessageType = "METADATA_CHECKSUM_RESPONSE"
)

// MetadataMessage is the envelope for all metadata messages
type MetadataMessage struct {
	Type      MessageType     `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	NodeID    string          `json:"node_id"` // Sender
	Payload   json.RawMessage `json:"payload"`
}

// DatabaseCreateRequest is sent when a client wants to create a new database
type DatabaseCreateRequest struct {
	DatabaseName      string `json:"database_name"`
	RequesterNodeID   string `json:"requester_node_id"`
	ReplicationFactor int    `json:"replication_factor"`
}

// DatabaseCreateResponse is sent by eligible nodes offering to host the database
type DatabaseCreateResponse struct {
	DatabaseName   string   `json:"database_name"`
	NodeID         string   `json:"node_id"`
	AvailablePorts PortPair `json:"available_ports"`
}

// DatabaseCreateConfirm is sent by the coordinator with the final membership
type DatabaseCreateConfirm struct {
	DatabaseName      string           `json:"database_name"`
	SelectedNodes     []NodeAssignment `json:"selected_nodes"`
	CoordinatorNodeID string           `json:"coordinator_node_id"`
}

// NodeAssignment represents a node assignment in a database cluster
type NodeAssignment struct {
	NodeID   string `json:"node_id"`
	HTTPPort int    `json:"http_port"`
	RaftPort int    `json:"raft_port"`
	Host     string `json:"host"`
	Role     string `json:"role"` // "leader" or "follower"
}

// DatabaseStatusUpdate is sent when a database changes status
type DatabaseStatusUpdate struct {
	DatabaseName string         `json:"database_name"`
	NodeID       string         `json:"node_id"`
	Status       DatabaseStatus `json:"status"`
	HTTPPort     int            `json:"http_port,omitempty"`
	RaftPort     int            `json:"raft_port,omitempty"`
}

// DatabaseIdleNotification is sent when a node detects idle database
type DatabaseIdleNotification struct {
	DatabaseName string    `json:"database_name"`
	NodeID       string    `json:"node_id"`
	LastActivity time.Time `json:"last_activity"`
}

// DatabaseShutdownCoordinated is sent to coordinate hibernation shutdown
type DatabaseShutdownCoordinated struct {
	DatabaseName string    `json:"database_name"`
	ShutdownTime time.Time `json:"shutdown_time"` // When to actually shutdown
}

// DatabaseWakeupRequest is sent to wake up a hibernating database
type DatabaseWakeupRequest struct {
	DatabaseName    string `json:"database_name"`
	RequesterNodeID string `json:"requester_node_id"`
}

// NodeCapacityAnnouncement is sent periodically to announce node capacity
type NodeCapacityAnnouncement struct {
	NodeID           string    `json:"node_id"`
	MaxDatabases     int       `json:"max_databases"`
	CurrentDatabases int       `json:"current_databases"`
	PortRangeHTTP    PortRange `json:"port_range_http"`
	PortRangeRaft    PortRange `json:"port_range_raft"`
}

// NodeHealthPing is sent periodically for health checks
type NodeHealthPing struct {
	NodeID           string `json:"node_id"`
	CurrentDatabases int    `json:"current_databases"`
}

// NodeHealthPong is the response to a health ping
type NodeHealthPong struct {
	NodeID   string `json:"node_id"`
	Healthy  bool   `json:"healthy"`
	PingFrom string `json:"ping_from"`
}

// NodeReplacementNeeded is sent when a node failure is detected
type NodeReplacementNeeded struct {
	DatabaseName      string   `json:"database_name"`
	FailedNodeID      string   `json:"failed_node_id"`
	CurrentNodes      []string `json:"current_nodes"`
	ReplicationFactor int      `json:"replication_factor"`
}

// NodeReplacementOffer is sent by nodes offering to replace a failed node
type NodeReplacementOffer struct {
	DatabaseName   string   `json:"database_name"`
	NodeID         string   `json:"node_id"`
	AvailablePorts PortPair `json:"available_ports"`
}

// NodeReplacementConfirm is sent when a replacement node is selected
type NodeReplacementConfirm struct {
	DatabaseName   string   `json:"database_name"`
	NewNodeID      string   `json:"new_node_id"`
	ReplacedNodeID string   `json:"replaced_node_id"`
	NewNodePorts   PortPair `json:"new_node_ports"`
	JoinAddress    string   `json:"join_address"`
}

// DatabaseCleanup is sent to trigger cleanup of orphaned data
type DatabaseCleanup struct {
	DatabaseName string `json:"database_name"`
	NodeID       string `json:"node_id"`
	Action       string `json:"action"` // e.g., "deleted_orphaned_data"
}

// MetadataSync contains full database metadata for synchronization
type MetadataSync struct {
	Metadata *DatabaseMetadata `json:"metadata"`
}

// MetadataChecksumRequest requests checksums from other nodes
type MetadataChecksumRequest struct {
	RequestID string `json:"request_id"`
}

// MetadataChecksumResponse contains checksums for all databases
type MetadataChecksumResponse struct {
	RequestID string             `json:"request_id"`
	Checksums []MetadataChecksum `json:"checksums"`
}

// MarshalMetadataMessage creates a MetadataMessage with the given payload
func MarshalMetadataMessage(msgType MessageType, nodeID string, payload interface{}) ([]byte, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	msg := MetadataMessage{
		Type:      msgType,
		Timestamp: time.Now(),
		NodeID:    nodeID,
		Payload:   payloadBytes,
	}

	return json.Marshal(msg)
}

// UnmarshalMetadataMessage parses a MetadataMessage
func UnmarshalMetadataMessage(data []byte) (*MetadataMessage, error) {
	var msg MetadataMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// UnmarshalPayload unmarshals the payload into the given type
func (msg *MetadataMessage) UnmarshalPayload(v interface{}) error {
	return json.Unmarshal(msg.Payload, v)
}
