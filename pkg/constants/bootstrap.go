package constants

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Bootstrap node configuration
var (
	// BootstrapPeerIDs are the fixed peer IDs for bootstrap nodes
	// Each corresponds to a specific Ed25519 private key
	BootstrapPeerIDs []string

	// BootstrapAddresses are the full multiaddrs for bootstrap nodes
	BootstrapAddresses []string

	// BootstrapPort is the default port for bootstrap nodes
	BootstrapPort int = 4001
)

// Load environment variables and initialize bootstrap configuration
func init() {
	loadEnvironmentConfig()
}

// loadEnvironmentConfig loads bootstrap configuration from .env file
func loadEnvironmentConfig() {
	// Try to load .env file from current directory and parent directories
	envPaths := []string{
		".env",
		"../.env",
		"../../.env", // For when running from anchat subdirectory
	}

	var envLoaded bool
	for _, path := range envPaths {
		if _, err := os.Stat(path); err == nil {
			if err := godotenv.Load(path); err == nil {
				envLoaded = true
				break
			}
		}
	}

	if !envLoaded {
		// Fallback to default values if no .env file found
		setDefaultBootstrapConfig()
		return
	}

	// Load bootstrap peers from environment
	if peersEnv := os.Getenv("BOOTSTRAP_PEERS"); peersEnv != "" {
		// Split by comma and trim whitespace
		peerAddrs := strings.Split(peersEnv, ",")
		BootstrapAddresses = make([]string, 0, len(peerAddrs))
		BootstrapPeerIDs = make([]string, 0, len(peerAddrs))

		for _, addr := range peerAddrs {
			addr = strings.TrimSpace(addr)
			if addr != "" {
				BootstrapAddresses = append(BootstrapAddresses, addr)

				// Extract peer ID from multiaddr
				if peerID := extractPeerIDFromMultiaddr(addr); peerID != "" {
					BootstrapPeerIDs = append(BootstrapPeerIDs, peerID)
				}
			}
		}
	}

	// Load bootstrap port from environment
	if portEnv := os.Getenv("BOOTSTRAP_PORT"); portEnv != "" {
		if port, err := strconv.Atoi(portEnv); err == nil && port > 0 {
			BootstrapPort = port
		}
	}

	// If no environment config found, use defaults
	if len(BootstrapAddresses) == 0 {
		setDefaultBootstrapConfig()
	}
}

// setDefaultBootstrapConfig sets default bootstrap configuration
func setDefaultBootstrapConfig() {
	BootstrapPeerIDs = []string{
		"12D3KooWN3AQHuxAzXfu98tiFYw7W3N2SyDwdxDRANXJp3ktVf8j",
	}
	BootstrapAddresses = []string{
		"/ip4/127.0.0.1/tcp/4001/p2p/12D3KooWN3AQHuxAzXfu98tiFYw7W3N2SyDwdxDRANXJp3ktVf8j",
		"/ip4/57.129.81.31/tcp/4001/p2p/12D3KooWQRK2duw5B5LXi8gA7HBBFiCsLvwyph2ZU9VBmvbE1Nei",
		"/ip4/38.242.250.186/tcp/4001/p2p/12D3KooWGbdnA22bN24X2gyY1o9jozwTBq9wbfvwtJ7G4XQ9JgFm",
	}
	BootstrapPort = 4001
}

// extractPeerIDFromMultiaddr extracts the peer ID from a multiaddr string
func extractPeerIDFromMultiaddr(multiaddr string) string {
	// Look for /p2p/ followed by the peer ID
	parts := strings.Split(multiaddr, "/p2p/")
	if len(parts) >= 2 {
		return parts[1]
	}
	return ""
}

// Constants for backward compatibility
var (
	// Primary bootstrap peer ID (first in the list)
	BootstrapPeerID string

	// Primary bootstrap address (first in the list)
	BootstrapAddress string
)

// updateBackwardCompatibilityConstants updates the single constants for backward compatibility
func updateBackwardCompatibilityConstants() {
	if len(BootstrapPeerIDs) > 0 {
		BootstrapPeerID = BootstrapPeerIDs[0]
	}
	if len(BootstrapAddresses) > 0 {
		BootstrapAddress = BootstrapAddresses[0]
	}
}

// Call this after loading environment config
func init() {
	// This runs after the first init() that calls loadEnvironmentConfig()
	updateBackwardCompatibilityConstants()
}

// Helper functions for working with bootstrap peers

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

// ReloadEnvironmentConfig reloads the configuration from environment
func ReloadEnvironmentConfig() {
	loadEnvironmentConfig()
	updateBackwardCompatibilityConstants()
}

// GetEnvironmentInfo returns information about the current configuration
func GetEnvironmentInfo() map[string]interface{} {
	return map[string]interface{}{
		"bootstrap_peers":    GetBootstrapPeers(),
		"bootstrap_peer_ids": GetBootstrapPeerIDs(),
		"bootstrap_port":     BootstrapPort,
		"environment":        os.Getenv("ENVIRONMENT"),
		"config_loaded_from": getConfigSource(),
	}
}

// getConfigSource returns where the configuration was loaded from
func getConfigSource() string {
	envPaths := []string{".env", "../.env", "../../.env"}
	for _, path := range envPaths {
		if _, err := os.Stat(path); err == nil {
			abs, _ := filepath.Abs(path)
			return abs
		}
	}
	return "default values (no .env file found)"
}
