package constants

import (
	"os"
)

// Bootstrap node configuration
var (
	// BootstrapPeerIDs are the fixed peer IDs for bootstrap nodes
	// Each corresponds to a specific Ed25519 private key
	BootstrapPeerIDs []string

	// BootstrapAddresses are the full multiaddrs for bootstrap nodes
	BootstrapAddresses []string

	// BootstrapPort is the default port for bootstrap nodes (LibP2P)
	BootstrapPort int = 4001

	// Primary bootstrap peer ID (first in the list)
	BootstrapPeerID string

	// Primary bootstrap address (first in the list)
	BootstrapAddress string
)

// Initialize bootstrap configuration (no .env loading; defaults only)
func init() {
	setDefaultBootstrapConfig()
	updateBackwardCompatibilityConstants()
}

// setDefaultBootstrapConfig sets default bootstrap configuration
func setDefaultBootstrapConfig() {
	// Check if we're in production environment
	BootstrapPeerIDs = []string{
		"12D3KooWNxt9bNvqftdqXg98JcUHreGxedWSZRUbyqXJ6CW7GaD4",
		"12D3KooWGbdnA22bN24X2gyY1o9jozwTBq9wbfvwtJ7G4XQ9JgFm",
	}
	BootstrapAddresses = []string{
		"/ip4/57.129.81.31/tcp/4001/p2p/12D3KooWNxt9bNvqftdqXg98JcUHreGxedWSZRUbyqXJ6CW7GaD4",
		"/ip4/38.242.250.186/tcp/4001/p2p/12D3KooWGbdnA22bN24X2gyY1o9jozwTBq9wbfvwtJ7G4XQ9JgFm",
	}

	BootstrapPort = 4001
}

// updateBackwardCompatibilityConstants updates the single constants for backward compatibility
func updateBackwardCompatibilityConstants() {
	if len(BootstrapPeerIDs) > 0 {
		BootstrapPeerID = BootstrapPeerIDs[0]
	}
	if len(BootstrapAddresses) > 0 {
		BootstrapAddress = BootstrapAddresses[0]
	}
}

// GetBootstrapPeers returns a copy of all bootstrap peer addresses
func GetBootstrapPeers() []string {
	if len(BootstrapAddresses) == 0 {
		setDefaultBootstrapConfig()
		updateBackwardCompatibilityConstants()
	}
	peers := make([]string, len(BootstrapAddresses))
	copy(peers, BootstrapAddresses)
	return peers
}

// GetBootstrapPeerIDs returns a copy of all bootstrap peer IDs
func GetBootstrapPeerIDs() []string {
	if len(BootstrapPeerIDs) == 0 {
		setDefaultBootstrapConfig()
		updateBackwardCompatibilityConstants()
	}
	ids := make([]string, len(BootstrapPeerIDs))
	copy(ids, BootstrapPeerIDs)
	return ids
}

// AddBootstrapPeer adds a new bootstrap peer to the lists (runtime only)
func AddBootstrapPeer(peerID, address string) {
	BootstrapPeerIDs = append(BootstrapPeerIDs, peerID)
	BootstrapAddresses = append(BootstrapAddresses, address)
	updateBackwardCompatibilityConstants()
}

// GetEnvironmentInfo returns information about the current configuration
func GetEnvironmentInfo() map[string]interface{} {
	return map[string]interface{}{
		"bootstrap_peers":    GetBootstrapPeers(),
		"bootstrap_peer_ids": GetBootstrapPeerIDs(),
		"bootstrap_port":     BootstrapPort,
		"environment":        os.Getenv("ENVIRONMENT"),
	}
}
