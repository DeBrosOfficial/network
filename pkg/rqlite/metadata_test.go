package rqlite

import (
	"sync"
	"testing"
	"time"
)

func TestMetadataStore_GetSetDatabase(t *testing.T) {
	store := NewMetadataStore()

	dbMeta := &DatabaseMetadata{
		DatabaseName: "testdb",
		NodeIDs:      []string{"node1", "node2", "node3"},
		PortMappings: map[string]PortPair{
			"node1": {HTTPPort: 5001, RaftPort: 7001},
			"node2": {HTTPPort: 5002, RaftPort: 7002},
			"node3": {HTTPPort: 5003, RaftPort: 7003},
		},
		Status:       StatusActive,
		CreatedAt:    time.Now(),
		LastAccessed: time.Now(),
		VectorClock:  NewVectorClock(),
	}

	store.UpsertDatabase(dbMeta)

	retrieved := store.GetDatabase("testdb")
	if retrieved == nil {
		t.Fatal("Expected to retrieve database, got nil")
	}

	if retrieved.DatabaseName != "testdb" {
		t.Errorf("Expected database name testdb, got %s", retrieved.DatabaseName)
	}

	if len(retrieved.NodeIDs) != 3 {
		t.Errorf("Expected 3 nodes, got %d", len(retrieved.NodeIDs))
	}

	if retrieved.Status != StatusActive {
		t.Errorf("Expected status active, got %s", retrieved.Status)
	}
}

func TestMetadataStore_DeleteDatabase(t *testing.T) {
	store := NewMetadataStore()

	dbMeta := &DatabaseMetadata{
		DatabaseName: "testdb",
		NodeIDs:      []string{"node1"},
		PortMappings: make(map[string]PortPair),
		Status:       StatusActive,
		VectorClock:  NewVectorClock(),
	}

	store.UpsertDatabase(dbMeta)

	// Verify it's there
	if store.GetDatabase("testdb") == nil {
		t.Fatal("Expected database to exist")
	}

	// Delete it
	store.DeleteDatabase("testdb")

	// Verify it's gone
	if store.GetDatabase("testdb") != nil {
		t.Error("Expected database to be deleted")
	}
}

func TestMetadataStore_ListDatabases(t *testing.T) {
	store := NewMetadataStore()

	// Add multiple databases
	for i := 1; i <= 5; i++ {
		dbMeta := &DatabaseMetadata{
			DatabaseName: string(rune('a' + i)),
			NodeIDs:      []string{"node1"},
			PortMappings: make(map[string]PortPair),
			Status:       StatusActive,
			VectorClock:  NewVectorClock(),
		}
		store.UpsertDatabase(dbMeta)
	}

	databases := store.GetAllDatabases()
	if len(databases) != 5 {
		t.Errorf("Expected 5 databases, got %d", len(databases))
	}
}

func TestMetadataStore_ConcurrentAccess(t *testing.T) {
	store := NewMetadataStore()
	var wg sync.WaitGroup

	// Spawn multiple goroutines for concurrent writes
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			dbMeta := &DatabaseMetadata{
				DatabaseName: string(rune('a' + id)),
				NodeIDs:      []string{"node1"},
				PortMappings: make(map[string]PortPair),
				Status:       StatusActive,
				VectorClock:  NewVectorClock(),
			}
			store.UpsertDatabase(dbMeta)
		}(i)
	}

	// Spawn multiple goroutines for concurrent reads
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			_ = store.GetDatabase(string(rune('a' + id)))
			_ = store.GetAllDatabases()
		}(i)
	}

	wg.Wait()

	// Verify all databases were added
	databases := store.GetAllDatabases()
	if len(databases) != 10 {
		t.Errorf("Expected 10 databases after concurrent writes, got %d", len(databases))
	}
}

func TestMetadataStore_NodeCapacity(t *testing.T) {
	store := NewMetadataStore()

	nodeCapacity := &NodeCapacity{
		NodeID:           "node1",
		MaxDatabases:     100,
		CurrentDatabases: 5,
		AvailablePortRange: PortRange{
			HTTPStart: 5001,
			HTTPEnd:   5999,
			RaftStart: 7001,
			RaftEnd:   7999,
		},
		LastHealthCheck: time.Now(),
		IsHealthy:       true,
	}

	store.UpsertNodeCapacity(nodeCapacity)

	retrieved := store.GetNode("node1")
	if retrieved == nil {
		t.Fatal("Expected to retrieve node capacity, got nil")
	}

	if retrieved.MaxDatabases != 100 {
		t.Errorf("Expected max databases 100, got %d", retrieved.MaxDatabases)
	}

	if retrieved.CurrentDatabases != 5 {
		t.Errorf("Expected current databases 5, got %d", retrieved.CurrentDatabases)
	}

	if !retrieved.IsHealthy {
		t.Error("Expected node to be healthy")
	}
}

func TestMetadataStore_UpdateNodeCapacity(t *testing.T) {
	store := NewMetadataStore()

	nodeCapacity := &NodeCapacity{
		NodeID:           "node1",
		MaxDatabases:     100,
		CurrentDatabases: 5,
		IsHealthy:        true,
	}

	store.UpsertNodeCapacity(nodeCapacity)

	// Update capacity
	nodeCapacity.CurrentDatabases = 10
	nodeCapacity.IsHealthy = false
	store.UpsertNodeCapacity(nodeCapacity)

	retrieved := store.GetNode("node1")
	if retrieved.CurrentDatabases != 10 {
		t.Errorf("Expected current databases 10, got %d", retrieved.CurrentDatabases)
	}

	if retrieved.IsHealthy {
		t.Error("Expected node to be unhealthy after update")
	}
}

func TestMetadataStore_ListNodes(t *testing.T) {
	store := NewMetadataStore()

	// Add multiple nodes
	for i := 1; i <= 3; i++ {
		nodeCapacity := &NodeCapacity{
			NodeID:           string(rune('A' + i)),
			MaxDatabases:     100,
			CurrentDatabases: i * 5,
			IsHealthy:        true,
		}
		store.UpsertNodeCapacity(nodeCapacity)
	}

	nodes := store.GetAllNodeCapacities()
	if len(nodes) != 3 {
		t.Errorf("Expected 3 nodes, got %d", len(nodes))
	}
}

func TestMetadataStore_UpdateDatabase(t *testing.T) {
	store := NewMetadataStore()

	dbMeta := &DatabaseMetadata{
		DatabaseName: "testdb",
		NodeIDs:      []string{"node1"},
		PortMappings: make(map[string]PortPair),
		Status:       StatusInitializing,
		VectorClock:  NewVectorClock(),
	}

	store.UpsertDatabase(dbMeta)

	// Update status
	dbMeta.Status = StatusActive
	dbMeta.LastAccessed = time.Now()
	store.UpsertDatabase(dbMeta)

	retrieved := store.GetDatabase("testdb")
	if retrieved.Status != StatusActive {
		t.Errorf("Expected status active, got %s", retrieved.Status)
	}
}
