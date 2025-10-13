package rqlite

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"go.uber.org/zap"
)

// CreateCoordinator coordinates the database creation process
type CreateCoordinator struct {
	dbName            string
	replicationFactor int
	requesterID       string
	responses         []DatabaseCreateResponse
	mu                sync.Mutex
	logger            *zap.Logger
}

// NewCreateCoordinator creates a new coordinator for database creation
func NewCreateCoordinator(dbName string, replicationFactor int, requesterID string, logger *zap.Logger) *CreateCoordinator {
	return &CreateCoordinator{
		dbName:            dbName,
		replicationFactor: replicationFactor,
		requesterID:       requesterID,
		responses:         make([]DatabaseCreateResponse, 0),
		logger:            logger,
	}
}

// AddResponse adds a response from a node
func (cc *CreateCoordinator) AddResponse(response DatabaseCreateResponse) {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	cc.responses = append(cc.responses, response)
}

// GetResponses returns all collected responses
func (cc *CreateCoordinator) GetResponses() []DatabaseCreateResponse {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	return append([]DatabaseCreateResponse(nil), cc.responses...)
}

// ResponseCount returns the number of responses received
func (cc *CreateCoordinator) ResponseCount() int {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	return len(cc.responses)
}

// SelectNodes selects the best nodes for the database cluster
func (cc *CreateCoordinator) SelectNodes() []DatabaseCreateResponse {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	if len(cc.responses) < cc.replicationFactor {
		cc.logger.Warn("Insufficient responses for database creation",
			zap.String("database", cc.dbName),
			zap.Int("required", cc.replicationFactor),
			zap.Int("received", len(cc.responses)))
		// Return what we have if less than required
		return cc.responses
	}

	// Sort responses by node ID for deterministic selection
	sorted := make([]DatabaseCreateResponse, len(cc.responses))
	copy(sorted, cc.responses)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].NodeID < sorted[j].NodeID
	})

	// Select first N nodes
	return sorted[:cc.replicationFactor]
}

// WaitForResponses waits for responses with a timeout
func (cc *CreateCoordinator) WaitForResponses(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				return fmt.Errorf("timeout waiting for responses")
			}
			if cc.ResponseCount() >= cc.replicationFactor {
				return nil
			}
		}
	}
}

// CoordinatorRegistry manages active coordinators for database creation
type CoordinatorRegistry struct {
	coordinators map[string]*CreateCoordinator // dbName -> coordinator
	mu           sync.RWMutex
}

// NewCoordinatorRegistry creates a new coordinator registry
func NewCoordinatorRegistry() *CoordinatorRegistry {
	return &CoordinatorRegistry{
		coordinators: make(map[string]*CreateCoordinator),
	}
}

// Register registers a new coordinator
func (cr *CoordinatorRegistry) Register(coordinator *CreateCoordinator) {
	cr.mu.Lock()
	defer cr.mu.Unlock()
	cr.coordinators[coordinator.dbName] = coordinator
}

// Get retrieves a coordinator by database name
func (cr *CoordinatorRegistry) Get(dbName string) *CreateCoordinator {
	cr.mu.RLock()
	defer cr.mu.RUnlock()
	return cr.coordinators[dbName]
}

// Remove removes a coordinator
func (cr *CoordinatorRegistry) Remove(dbName string) {
	cr.mu.Lock()
	defer cr.mu.Unlock()
	delete(cr.coordinators, dbName)
}

// HandleCreateResponse handles a CREATE_RESPONSE message
func (cr *CoordinatorRegistry) HandleCreateResponse(response DatabaseCreateResponse) {
	cr.mu.RLock()
	coordinator := cr.coordinators[response.DatabaseName]
	cr.mu.RUnlock()

	if coordinator != nil {
		coordinator.AddResponse(response)
	}
}
