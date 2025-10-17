package rqlite

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"time"
)

// SelectCoordinator deterministically selects a coordinator from a list of node IDs
// Uses lexicographic ordering (lowest ID wins)
func SelectCoordinator(nodeIDs []string) string {
	if len(nodeIDs) == 0 {
		return ""
	}

	sorted := make([]string, len(nodeIDs))
	copy(sorted, nodeIDs)
	sort.Strings(sorted)

	return sorted[0]
}

// ResolveConflict resolves a conflict between two database metadata entries
// Returns the winning metadata entry
func ResolveConflict(local, remote *DatabaseMetadata) *DatabaseMetadata {
	if local == nil {
		return remote
	}
	if remote == nil {
		return local
	}

	// Compare vector clocks
	localVC := VectorClock(local.VectorClock)
	remoteVC := VectorClock(remote.VectorClock)

	comparison := localVC.Compare(remoteVC)

	if comparison == -1 {
		// Local happens before remote, remote wins
		return remote
	} else if comparison == 1 {
		// Remote happens before local, local wins
		return local
	}

	// Concurrent: use version number as tiebreaker
	if remote.Version > local.Version {
		return remote
	} else if local.Version > remote.Version {
		return local
	}

	// Same version: use timestamp as tiebreaker
	if remote.CreatedAt.After(local.CreatedAt) {
		return remote
	} else if local.CreatedAt.After(remote.CreatedAt) {
		return local
	}

	// Same timestamp: use lexicographic comparison of database name
	if remote.DatabaseName < local.DatabaseName {
		return remote
	}

	return local
}

// MetadataChecksum represents a checksum of database metadata
type MetadataChecksum struct {
	DatabaseName string `json:"database_name"`
	Version      uint64 `json:"version"`
	Hash         string `json:"hash"`
}

// ComputeMetadataChecksum computes a checksum for database metadata
func ComputeMetadataChecksum(db *DatabaseMetadata) MetadataChecksum {
	if db == nil {
		return MetadataChecksum{}
	}

	// Create a canonical representation for hashing
	canonical := struct {
		DatabaseName string
		NodeIDs      []string
		PortMappings map[string]PortPair
		Status       DatabaseStatus
	}{
		DatabaseName: db.DatabaseName,
		NodeIDs:      make([]string, len(db.NodeIDs)),
		PortMappings: db.PortMappings,
		Status:       db.Status,
	}

	// Sort node IDs for deterministic hashing
	copy(canonical.NodeIDs, db.NodeIDs)
	sort.Strings(canonical.NodeIDs)

	// Serialize and hash
	data, _ := json.Marshal(canonical)
	hash := sha256.Sum256(data)

	return MetadataChecksum{
		DatabaseName: db.DatabaseName,
		Version:      db.Version,
		Hash:         hex.EncodeToString(hash[:]),
	}
}

// ComputeFullStateChecksum computes checksums for all databases in the store
func ComputeFullStateChecksum(store *MetadataStore) []MetadataChecksum {
	checksums := make([]MetadataChecksum, 0)

	for _, name := range store.ListDatabases() {
		if db := store.GetDatabase(name); db != nil {
			checksums = append(checksums, ComputeMetadataChecksum(db))
		}
	}

	// Sort by database name for deterministic ordering
	sort.Slice(checksums, func(i, j int) bool {
		return checksums[i].DatabaseName < checksums[j].DatabaseName
	})

	return checksums
}

// SelectNodesForDatabase selects N nodes from the list of healthy nodes
// Returns up to replicationFactor nodes
func SelectNodesForDatabase(healthyNodes []string, replicationFactor int) []string {
	if len(healthyNodes) == 0 {
		return []string{}
	}

	// Sort for deterministic selection
	sorted := make([]string, len(healthyNodes))
	copy(sorted, healthyNodes)
	sort.Strings(sorted)

	// Select first N nodes
	n := replicationFactor
	if n > len(sorted) {
		n = len(sorted)
	}

	return sorted[:n]
}

// IsNodeInCluster checks if a node is part of a database cluster
func IsNodeInCluster(nodeID string, db *DatabaseMetadata) bool {
	if db == nil {
		return false
	}

	for _, id := range db.NodeIDs {
		if id == nodeID {
			return true
		}
	}
	return false
}

// UpdateDatabaseMetadata updates metadata with vector clock and version increment
func UpdateDatabaseMetadata(db *DatabaseMetadata, nodeID string) {
	if db.VectorClock == nil {
		db.VectorClock = NewVectorClock()
	}

	// Increment vector clock for this node
	vc := VectorClock(db.VectorClock)
	vc.Increment(nodeID)

	// Increment version
	db.Version++

	// Update last accessed time
	db.LastAccessed = time.Now()
}
