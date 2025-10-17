package rqlite

import (
	"errors"
	"fmt"
	"math/rand"
	"net"
	"sync"
)

// PortManager manages port allocation for database instances
type PortManager struct {
	allocatedPorts map[int]string // port -> database name
	httpRange      PortRange
	raftRange      PortRange
	mu             sync.RWMutex
}

// NewPortManager creates a new port manager
func NewPortManager(httpRange, raftRange PortRange) *PortManager {
	return &PortManager{
		allocatedPorts: make(map[int]string),
		httpRange:      httpRange,
		raftRange:      raftRange,
	}
}

// AllocatePortPair allocates a pair of ports (HTTP and Raft) for a database
func (pm *PortManager) AllocatePortPair(dbName string) (PortPair, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Try up to 20 times to find available ports
	for attempt := 0; attempt < 20; attempt++ {
		httpPort := pm.randomPortInRange(pm.httpRange)
		raftPort := pm.randomPortInRange(pm.raftRange)

		// Check if already allocated
		if _, exists := pm.allocatedPorts[httpPort]; exists {
			continue
		}
		if _, exists := pm.allocatedPorts[raftPort]; exists {
			continue
		}

		// Test if actually bindable
		if !pm.canBind(httpPort) || !pm.canBind(raftPort) {
			continue
		}

		// Allocate the ports
		pm.allocatedPorts[httpPort] = dbName
		pm.allocatedPorts[raftPort] = dbName

		return PortPair{HTTPPort: httpPort, RaftPort: raftPort}, nil
	}

	return PortPair{}, errors.New("no available ports after 20 attempts")
}

// ReleasePortPair releases a pair of ports back to the pool
func (pm *PortManager) ReleasePortPair(pair PortPair) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	delete(pm.allocatedPorts, pair.HTTPPort)
	delete(pm.allocatedPorts, pair.RaftPort)
}

// IsPortPairAvailable checks if a specific port pair is available
func (pm *PortManager) IsPortPairAvailable(pair PortPair) bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	// Check if ports are in range
	if !pm.isInRange(pair.HTTPPort, pm.httpRange) {
		return false
	}
	if !pm.isInRange(pair.RaftPort, pm.raftRange) {
		return false
	}

	// Check if already allocated
	if _, exists := pm.allocatedPorts[pair.HTTPPort]; exists {
		return false
	}
	if _, exists := pm.allocatedPorts[pair.RaftPort]; exists {
		return false
	}

	// Test if actually bindable
	return pm.canBind(pair.HTTPPort) && pm.canBind(pair.RaftPort)
}

// AllocateSpecificPortPair attempts to allocate a specific port pair
func (pm *PortManager) AllocateSpecificPortPair(dbName string, pair PortPair) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Check if ports are in range
	if !pm.isInRange(pair.HTTPPort, pm.httpRange) {
		return fmt.Errorf("HTTP port %d not in range %d-%d", pair.HTTPPort, pm.httpRange.Start, pm.httpRange.End)
	}
	if !pm.isInRange(pair.RaftPort, pm.raftRange) {
		return fmt.Errorf("Raft port %d not in range %d-%d", pair.RaftPort, pm.raftRange.Start, pm.raftRange.End)
	}

	// Check if already allocated
	if _, exists := pm.allocatedPorts[pair.HTTPPort]; exists {
		return fmt.Errorf("HTTP port %d already allocated", pair.HTTPPort)
	}
	if _, exists := pm.allocatedPorts[pair.RaftPort]; exists {
		return fmt.Errorf("Raft port %d already allocated", pair.RaftPort)
	}

	// Test if actually bindable
	if !pm.canBind(pair.HTTPPort) {
		return fmt.Errorf("HTTP port %d not bindable", pair.HTTPPort)
	}
	if !pm.canBind(pair.RaftPort) {
		return fmt.Errorf("Raft port %d not bindable", pair.RaftPort)
	}

	// Allocate the ports
	pm.allocatedPorts[pair.HTTPPort] = dbName
	pm.allocatedPorts[pair.RaftPort] = dbName

	return nil
}

// GetAllocatedPorts returns all currently allocated ports
func (pm *PortManager) GetAllocatedPorts() map[int]string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	// Return a copy
	copy := make(map[int]string, len(pm.allocatedPorts))
	for port, db := range pm.allocatedPorts {
		copy[port] = db
	}
	return copy
}

// GetAvailablePortCount returns the approximate number of available ports
func (pm *PortManager) GetAvailablePortCount() int {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	httpCount := pm.httpRange.End - pm.httpRange.Start + 1
	raftCount := pm.raftRange.End - pm.raftRange.Start + 1

	// Return the minimum of the two (since we need pairs)
	totalPairs := httpCount
	if raftCount < httpCount {
		totalPairs = raftCount
	}

	return totalPairs - len(pm.allocatedPorts)/2
}

// IsPortAllocated checks if a port is currently allocated
func (pm *PortManager) IsPortAllocated(port int) bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	_, exists := pm.allocatedPorts[port]
	return exists
}

// AllocateSpecificPorts allocates specific ports for a database
func (pm *PortManager) AllocateSpecificPorts(dbName string, ports PortPair) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Check if ports are already allocated
	if _, exists := pm.allocatedPorts[ports.HTTPPort]; exists {
		return fmt.Errorf("HTTP port %d already allocated", ports.HTTPPort)
	}
	if _, exists := pm.allocatedPorts[ports.RaftPort]; exists {
		return fmt.Errorf("Raft port %d already allocated", ports.RaftPort)
	}

	// Allocate the ports
	pm.allocatedPorts[ports.HTTPPort] = dbName
	pm.allocatedPorts[ports.RaftPort] = dbName

	return nil
}

// randomPortInRange returns a random port within the given range
func (pm *PortManager) randomPortInRange(portRange PortRange) int {
	return portRange.Start + rand.Intn(portRange.End-portRange.Start+1)
}

// isInRange checks if a port is within the given range
func (pm *PortManager) isInRange(port int, portRange PortRange) bool {
	return port >= portRange.Start && port <= portRange.End
}

// canBind tests if a port can be bound
func (pm *PortManager) canBind(port int) bool {
	// Test bind to check if port is actually available
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	listener.Close()
	return true
}
