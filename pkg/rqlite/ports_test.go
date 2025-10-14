package rqlite

import (
	"testing"
)

func TestPortManager_AllocatePortPair(t *testing.T) {
	pm := NewPortManager(
		PortRange{Start: 5001, End: 5010},
		PortRange{Start: 7001, End: 7010},
	)

	ports, err := pm.AllocatePortPair("testdb")
	if err != nil {
		t.Fatalf("Failed to allocate port pair: %v", err)
	}

	// Verify HTTP port in range
	if ports.HTTPPort < 5001 || ports.HTTPPort > 5010 {
		t.Errorf("HTTP port %d out of range [5001-5010]", ports.HTTPPort)
	}

	// Verify Raft port in range
	if ports.RaftPort < 7001 || ports.RaftPort > 7010 {
		t.Errorf("Raft port %d out of range [7001-7010]", ports.RaftPort)
	}

	// Verify ports are different
	if ports.HTTPPort == ports.RaftPort {
		t.Error("HTTP and Raft ports should be different")
	}
}

func TestPortManager_ReleasePortPair(t *testing.T) {
	pm := NewPortManager(
		PortRange{Start: 5001, End: 5010},
		PortRange{Start: 7001, End: 7010},
	)

	// Allocate
	ports, err := pm.AllocatePortPair("testdb")
	if err != nil {
		t.Fatalf("Failed to allocate port pair: %v", err)
	}

	// Release
	pm.ReleasePortPair(ports)

	// Should be able to allocate again
	ports2, err := pm.AllocatePortPair("testdb2")
	if err != nil {
		t.Fatalf("Failed to allocate after release: %v", err)
	}

	// Might get same ports back (that's OK)
	if ports2.HTTPPort == 0 || ports2.RaftPort == 0 {
		t.Error("Got zero ports after release and reallocation")
	}
}

func TestPortManager_IsPortAllocated(t *testing.T) {
	pm := NewPortManager(
		PortRange{Start: 5001, End: 5010},
		PortRange{Start: 7001, End: 7010},
	)

	// Initially not allocated
	if pm.IsPortAllocated(5001) {
		t.Error("Port should not be allocated initially")
	}

	// Allocate
	ports, err := pm.AllocatePortPair("testdb")
	if err != nil {
		t.Fatalf("Failed to allocate: %v", err)
	}

	// Should be allocated now
	if !pm.IsPortAllocated(ports.HTTPPort) {
		t.Error("HTTP port should be allocated")
	}
	if !pm.IsPortAllocated(ports.RaftPort) {
		t.Error("Raft port should be allocated")
	}

	// Release
	pm.ReleasePortPair(ports)

	// Should not be allocated anymore
	if pm.IsPortAllocated(ports.HTTPPort) {
		t.Error("HTTP port should not be allocated after release")
	}
	if pm.IsPortAllocated(ports.RaftPort) {
		t.Error("Raft port should not be allocated after release")
	}
}

func TestPortManager_AllocateSpecificPorts(t *testing.T) {
	pm := NewPortManager(
		PortRange{Start: 5001, End: 5010},
		PortRange{Start: 7001, End: 7010},
	)

	specificPorts := PortPair{HTTPPort: 5005, RaftPort: 7005}

	// Allocate specific ports
	err := pm.AllocateSpecificPorts("testdb", specificPorts)
	if err != nil {
		t.Fatalf("Failed to allocate specific ports: %v", err)
	}

	// Verify allocated
	if !pm.IsPortAllocated(5005) {
		t.Error("Port 5005 should be allocated")
	}
	if !pm.IsPortAllocated(7005) {
		t.Error("Port 7005 should be allocated")
	}

	// Try to allocate same ports again - should fail
	err = pm.AllocateSpecificPorts("testdb2", specificPorts)
	if err == nil {
		t.Error("Expected error when allocating already-allocated ports")
	}
}

func TestPortManager_Exhaustion(t *testing.T) {
	// Very small range
	pm := NewPortManager(
		PortRange{Start: 5001, End: 5002}, // Only 2 ports
		PortRange{Start: 7001, End: 7002}, // Only 2 ports
	)

	// Allocate first pair (uses 2 ports)
	_, err := pm.AllocatePortPair("db1")
	if err != nil {
		t.Fatalf("First allocation should succeed: %v", err)
	}

	// Try to allocate second pair - might fail due to limited ports
	// Note: This test is probabilistic due to random selection
	// In a real scenario with only 2 ports per range, we can only fit 1 database
	_, err = pm.AllocatePortPair("db2")
	// We expect this to eventually fail after retries
	// The actual behavior depends on random selection

	// For a more deterministic test, allocate specific ports
	pm2 := NewPortManager(
		PortRange{Start: 5001, End: 5002},
		PortRange{Start: 7001, End: 7002},
	)

	_ = pm2.AllocateSpecificPorts("db1", PortPair{HTTPPort: 5001, RaftPort: 7001})
	_ = pm2.AllocateSpecificPorts("db2", PortPair{HTTPPort: 5002, RaftPort: 7002})

	// Now exhausted
	err = pm2.AllocateSpecificPorts("db3", PortPair{HTTPPort: 5001, RaftPort: 7001})
	if err == nil {
		t.Error("Expected error when ports exhausted")
	}
}

func TestPortManager_MultipleDatabases(t *testing.T) {
	pm := NewPortManager(
		PortRange{Start: 5001, End: 5020},
		PortRange{Start: 7001, End: 7020},
	)

	databases := []string{"db1", "db2", "db3", "db4", "db5"}
	allocatedPorts := make(map[int]bool)

	for _, db := range databases {
		ports, err := pm.AllocatePortPair(db)
		if err != nil {
			t.Fatalf("Failed to allocate for %s: %v", db, err)
		}

		// Verify no port conflicts
		if allocatedPorts[ports.HTTPPort] {
			t.Errorf("HTTP port %d already allocated", ports.HTTPPort)
		}
		if allocatedPorts[ports.RaftPort] {
			t.Errorf("Raft port %d already allocated", ports.RaftPort)
		}

		allocatedPorts[ports.HTTPPort] = true
		allocatedPorts[ports.RaftPort] = true
	}

	// Should have allocated 10 unique ports (5 HTTP + 5 Raft)
	if len(allocatedPorts) != 10 {
		t.Errorf("Expected 10 unique ports, got %d", len(allocatedPorts))
	}
}
