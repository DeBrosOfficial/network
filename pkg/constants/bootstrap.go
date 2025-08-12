package constants

import (
	"os"

	"git.debros.io/DeBros/network/pkg/config"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

// Bootstrap node configuration
var (
	// BootstrapAddresses are the full multiaddrs for bootstrap nodes
	BootstrapAddresses []string

	// BootstrapPort is the default port for bootstrap nodes (LibP2P)
	BootstrapPort int = 4001

	// Primary bootstrap address (first in the list) - for backward compatibility
	BootstrapAddress string
)

// Initialize bootstrap configuration (no .env loading; defaults only)
func init() {
	setDefaultBootstrapConfig()
	updateBackwardCompatibilityConstants()
}

// setDefaultBootstrapConfig sets default bootstrap configuration for local development
func setDefaultBootstrapConfig() {
	var cfg *config.Config
	BootstrapAddresses = cfg.Discovery.BootstrapPeers
	BootstrapPort = cfg.Discovery.BootstrapPort
}

// updateBackwardCompatibilityConstants updates the single constants for backward compatibility
func updateBackwardCompatibilityConstants() {
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

// GetBootstrapPeerIDs extracts and returns peer IDs from bootstrap addresses
func GetBootstrapPeerIDs() []string {
	if len(BootstrapAddresses) == 0 {
		setDefaultBootstrapConfig()
		updateBackwardCompatibilityConstants()
	}

	var ids []string
	for _, addr := range BootstrapAddresses {
		if ma, err := multiaddr.NewMultiaddr(addr); err == nil {
			if pi, err := peer.AddrInfoFromP2pAddr(ma); err == nil {
				ids = append(ids, pi.ID.String())
			}
		}
	}
	return ids
}

// AddBootstrapPeer adds a new bootstrap peer address (runtime only)
func AddBootstrapPeer(address string) {
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
