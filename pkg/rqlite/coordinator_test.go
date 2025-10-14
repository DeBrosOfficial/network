package rqlite

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestCreateCoordinator_AddResponse(t *testing.T) {
	logger := zap.NewNop()
	coordinator := NewCreateCoordinator("testdb", 3, "node1", logger)

	response := DatabaseCreateResponse{
		DatabaseName: "testdb",
		NodeID:       "node2",
		AvailablePorts: PortPair{
			HTTPPort: 5001,
			RaftPort: 7001,
		},
	}

	coordinator.AddResponse(response)

	responses := coordinator.GetResponses()
	if len(responses) != 1 {
		t.Errorf("Expected 1 response, got %d", len(responses))
	}

	if responses[0].NodeID != "node2" {
		t.Errorf("Expected node2, got %s", responses[0].NodeID)
	}
}

func TestCreateCoordinator_SelectNodes(t *testing.T) {
	logger := zap.NewNop()
	coordinator := NewCreateCoordinator("testdb", 3, "node1", logger)

	// Add more responses than needed
	for i := 1; i <= 5; i++ {
		response := DatabaseCreateResponse{
			DatabaseName: "testdb",
			NodeID:       string(rune('A' + i)),
			AvailablePorts: PortPair{
				HTTPPort: 5000 + i,
				RaftPort: 7000 + i,
			},
		}
		coordinator.AddResponse(response)
	}

	selected := coordinator.SelectNodes()

	// Should select exactly 3 nodes
	if len(selected) != 3 {
		t.Errorf("Expected 3 selected nodes, got %d", len(selected))
	}

	// Verify deterministic selection (should be first 3 added)
	expectedNodes := []string{"B", "C", "D"}
	for i, node := range selected {
		if node.NodeID != expectedNodes[i] {
			t.Errorf("Expected node %s at position %d, got %s", expectedNodes[i], i, node.NodeID)
		}
	}
}

func TestCreateCoordinator_SelectNodes_InsufficientResponses(t *testing.T) {
	logger := zap.NewNop()
	coordinator := NewCreateCoordinator("testdb", 3, "node1", logger)

	// Add only 2 responses
	coordinator.AddResponse(DatabaseCreateResponse{
		DatabaseName:   "testdb",
		NodeID:         "node2",
		AvailablePorts: PortPair{HTTPPort: 5001, RaftPort: 7001},
	})
	coordinator.AddResponse(DatabaseCreateResponse{
		DatabaseName:   "testdb",
		NodeID:         "node3",
		AvailablePorts: PortPair{HTTPPort: 5002, RaftPort: 7002},
	})

	selected := coordinator.SelectNodes()

	// Should return all available responses even if less than requested
	if len(selected) != 2 {
		t.Errorf("Expected 2 selected nodes, got %d", len(selected))
	}
}

func TestCreateCoordinator_WaitForResponses_Success(t *testing.T) {
	logger := zap.NewNop()
	coordinator := NewCreateCoordinator("testdb", 2, "node1", logger)

	// Add responses in goroutine
	go func() {
		time.Sleep(100 * time.Millisecond)
		coordinator.AddResponse(DatabaseCreateResponse{
			DatabaseName:   "testdb",
			NodeID:         "node2",
			AvailablePorts: PortPair{HTTPPort: 5001, RaftPort: 7001},
		})
		coordinator.AddResponse(DatabaseCreateResponse{
			DatabaseName:   "testdb",
			NodeID:         "node3",
			AvailablePorts: PortPair{HTTPPort: 5002, RaftPort: 7002},
		})
	}()

	ctx := context.Background()
	err := coordinator.WaitForResponses(ctx, 2*time.Second)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	responses := coordinator.GetResponses()
	if len(responses) != 2 {
		t.Errorf("Expected 2 responses after wait, got %d", len(responses))
	}
}

func TestCreateCoordinator_WaitForResponses_Timeout(t *testing.T) {
	logger := zap.NewNop()
	coordinator := NewCreateCoordinator("testdb", 3, "node1", logger)

	// Add only 1 response
	coordinator.AddResponse(DatabaseCreateResponse{
		DatabaseName:   "testdb",
		NodeID:         "node2",
		AvailablePorts: PortPair{HTTPPort: 5001, RaftPort: 7001},
	})

	ctx := context.Background()
	err := coordinator.WaitForResponses(ctx, 500*time.Millisecond)

	// Should timeout since we need 3 but only have 1
	if err == nil {
		t.Error("Expected timeout error, got nil")
	}
}

func TestCreateCoordinator_WaitForResponses_ContextCanceled(t *testing.T) {
	logger := zap.NewNop()
	coordinator := NewCreateCoordinator("testdb", 3, "node1", logger)

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel context immediately
	cancel()

	err := coordinator.WaitForResponses(ctx, 5*time.Second)
	if err == nil {
		t.Error("Expected context canceled error, got nil")
	}
}

func TestCoordinatorRegistry_Register(t *testing.T) {
	registry := NewCoordinatorRegistry()
	logger := zap.NewNop()

	coordinator := NewCreateCoordinator("testdb", 3, "node1", logger)
	registry.Register(coordinator)

	retrieved := registry.Get("testdb")
	if retrieved == nil {
		t.Fatal("Expected to retrieve coordinator, got nil")
	}

	if retrieved.dbName != "testdb" {
		t.Errorf("Expected database name testdb, got %s", retrieved.dbName)
	}
}

func TestCoordinatorRegistry_Remove(t *testing.T) {
	registry := NewCoordinatorRegistry()
	logger := zap.NewNop()

	coordinator := NewCreateCoordinator("testdb", 3, "node1", logger)
	registry.Register(coordinator)

	// Verify it's there
	if registry.Get("testdb") == nil {
		t.Fatal("Expected coordinator to be registered")
	}

	// Remove it
	registry.Remove("testdb")

	// Verify it's gone
	if registry.Get("testdb") != nil {
		t.Error("Expected coordinator to be removed")
	}
}

func TestCoordinatorRegistry_GetNonexistent(t *testing.T) {
	registry := NewCoordinatorRegistry()

	retrieved := registry.Get("nonexistent")
	if retrieved != nil {
		t.Error("Expected nil for nonexistent coordinator")
	}
}

func TestCoordinatorRegistry_MultipleCoordinators(t *testing.T) {
	registry := NewCoordinatorRegistry()
	logger := zap.NewNop()

	coord1 := NewCreateCoordinator("db1", 3, "node1", logger)
	coord2 := NewCreateCoordinator("db2", 3, "node1", logger)
	coord3 := NewCreateCoordinator("db3", 3, "node1", logger)

	registry.Register(coord1)
	registry.Register(coord2)
	registry.Register(coord3)

	// Verify all registered
	if registry.Get("db1") == nil {
		t.Error("Expected db1 coordinator")
	}
	if registry.Get("db2") == nil {
		t.Error("Expected db2 coordinator")
	}
	if registry.Get("db3") == nil {
		t.Error("Expected db3 coordinator")
	}

	// Remove one
	registry.Remove("db2")

	// Verify others still there
	if registry.Get("db1") == nil {
		t.Error("Expected db1 coordinator to still exist")
	}
	if registry.Get("db2") != nil {
		t.Error("Expected db2 coordinator to be removed")
	}
	if registry.Get("db3") == nil {
		t.Error("Expected db3 coordinator to still exist")
	}
}
