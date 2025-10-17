package rqlite

import (
	"testing"
)

func TestSelectCoordinator_SingleNode(t *testing.T) {
	nodeIDs := []string{"node1"}
	coordinator := SelectCoordinator(nodeIDs)

	if coordinator != "node1" {
		t.Errorf("Expected node1, got %s", coordinator)
	}
}

func TestSelectCoordinator_MultipleNodes(t *testing.T) {
	nodeIDs := []string{"node3", "node1", "node2"}
	coordinator := SelectCoordinator(nodeIDs)

	// Should select lowest lexicographical ID
	if coordinator != "node1" {
		t.Errorf("Expected node1 (lowest ID), got %s", coordinator)
	}
}

func TestSelectCoordinator_EmptyList(t *testing.T) {
	nodeIDs := []string{}
	coordinator := SelectCoordinator(nodeIDs)

	if coordinator != "" {
		t.Errorf("Expected empty string for empty list, got %s", coordinator)
	}
}

func TestSelectCoordinator_Deterministic(t *testing.T) {
	nodeIDs := []string{"nodeZ", "nodeA", "nodeM", "nodeB"}

	// Run multiple times
	results := make(map[string]int)
	for i := 0; i < 10; i++ {
		coordinator := SelectCoordinator(nodeIDs)
		results[coordinator]++
	}

	// Should always return the same result
	if len(results) != 1 {
		t.Errorf("Expected deterministic result, got multiple: %v", results)
	}

	// Should be nodeA
	if _, exists := results["nodeA"]; !exists {
		t.Errorf("Expected nodeA to be selected, got %v", results)
	}
}

func TestIsCoordinator_True(t *testing.T) {
	nodeIDs := []string{"node3", "node1", "node2"}

	if !IsCoordinator("node1", nodeIDs) {
		t.Error("Expected node1 to be coordinator")
	}
}

func TestIsCoordinator_False(t *testing.T) {
	nodeIDs := []string{"node3", "node1", "node2"}

	if IsCoordinator("node2", nodeIDs) {
		t.Error("Expected node2 to NOT be coordinator")
	}
}

func TestIsCoordinator_EmptyList(t *testing.T) {
	nodeIDs := []string{}

	if IsCoordinator("node1", nodeIDs) {
		t.Error("Expected false for empty node list")
	}
}
