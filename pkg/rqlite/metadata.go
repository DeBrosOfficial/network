package rqlite

import (
	"sync"
	"time"
)

// DatabaseStatus represents the state of a database cluster
type DatabaseStatus string

const (
	StatusInitializing DatabaseStatus = "initializing"
	StatusActive       DatabaseStatus = "active"
	StatusHibernating  DatabaseStatus = "hibernating"
	StatusWaking       DatabaseStatus = "waking"
)

// PortPair represents HTTP and Raft ports for a database instance
type PortPair struct {
	HTTPPort int `json:"http_port"`
	RaftPort int `json:"raft_port"`
}

// DatabaseMetadata contains metadata for a single database cluster
type DatabaseMetadata struct {
	DatabaseName string              `json:"database_name"` // e.g., "my_app_exampledb_1"
	NodeIDs      []string            `json:"node_ids"`      // Peer IDs hosting this database
	PortMappings map[string]PortPair `json:"port_mappings"` // nodeID -> {HTTP port, Raft port}
	Status       DatabaseStatus      `json:"status"`        // Current status
	CreatedAt    time.Time           `json:"created_at"`
	LastAccessed time.Time           `json:"last_accessed"`
	LeaderNodeID string              `json:"leader_node_id"` // Which node is rqlite leader
	Version      uint64              `json:"version"`        // For conflict resolution
	VectorClock  map[string]uint64   `json:"vector_clock"`   // For distributed consensus
}

// NodeCapacity represents capacity information for a node
type NodeCapacity struct {
	NodeID           string    `json:"node_id"`
	MaxDatabases     int       `json:"max_databases"`     // Configured limit
	CurrentDatabases int       `json:"current_databases"` // How many currently active
	PortRangeHTTP    PortRange `json:"port_range_http"`
	PortRangeRaft    PortRange `json:"port_range_raft"`
	LastHealthCheck  time.Time `json:"last_health_check"`
	IsHealthy        bool      `json:"is_healthy"`
}

// PortRange represents a range of available ports
type PortRange struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

// MetadataStore is an in-memory store for database metadata
type MetadataStore struct {
	databases map[string]*DatabaseMetadata // key = database name
	nodes     map[string]*NodeCapacity     // key = node ID
	mu        sync.RWMutex
}

// NewMetadataStore creates a new metadata store
func NewMetadataStore() *MetadataStore {
	return &MetadataStore{
		databases: make(map[string]*DatabaseMetadata),
		nodes:     make(map[string]*NodeCapacity),
	}
}

// GetDatabase retrieves metadata for a database
func (ms *MetadataStore) GetDatabase(name string) *DatabaseMetadata {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	if db, exists := ms.databases[name]; exists {
		// Return a copy to prevent external modification
		dbCopy := *db
		return &dbCopy
	}
	return nil
}

// SetDatabase stores or updates metadata for a database
func (ms *MetadataStore) SetDatabase(db *DatabaseMetadata) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.databases[db.DatabaseName] = db
}

// DeleteDatabase removes metadata for a database
func (ms *MetadataStore) DeleteDatabase(name string) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	delete(ms.databases, name)
}

// ListDatabases returns all database names
func (ms *MetadataStore) ListDatabases() []string {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	names := make([]string, 0, len(ms.databases))
	for name := range ms.databases {
		names = append(names, name)
	}
	return names
}

// GetNode retrieves capacity info for a node
func (ms *MetadataStore) GetNode(nodeID string) *NodeCapacity {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	if node, exists := ms.nodes[nodeID]; exists {
		nodeCopy := *node
		return &nodeCopy
	}
	return nil
}

// SetNode stores or updates capacity info for a node
func (ms *MetadataStore) SetNode(node *NodeCapacity) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.nodes[node.NodeID] = node
}

// DeleteNode removes capacity info for a node
func (ms *MetadataStore) DeleteNode(nodeID string) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	delete(ms.nodes, nodeID)
}

// ListNodes returns all node IDs
func (ms *MetadataStore) ListNodes() []string {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	ids := make([]string, 0, len(ms.nodes))
	for id := range ms.nodes {
		ids = append(ids, id)
	}
	return ids
}

// GetHealthyNodes returns IDs of healthy nodes
func (ms *MetadataStore) GetHealthyNodes() []string {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	healthy := make([]string, 0)
	for id, node := range ms.nodes {
		if node.IsHealthy && node.CurrentDatabases < node.MaxDatabases {
			healthy = append(healthy, id)
		}
	}
	return healthy
}
