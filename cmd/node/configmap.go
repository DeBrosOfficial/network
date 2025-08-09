package main

import (
	"fmt"
	"strings"

	"git.debros.io/DeBros/network/pkg/config"
	"git.debros.io/DeBros/network/pkg/constants"
	"git.debros.io/DeBros/network/pkg/logging"
)

// NodeFlagValues holds parsed CLI flag values in a structured form.
type NodeFlagValues struct {
	DataDir    string
	NodeID     string
	Bootstrap  string
	Role       string
	P2PPort    int
	RqlHTTP    int
	RqlRaft    int
	Advertise  string
}

// MapFlagsAndEnvToConfig applies environment overrides and CLI flags to cfg.
// Precedence: flags > env > defaults. Behavior mirrors previous inline logic in main.go.
// Returns the derived RQLite Raft join address for non-bootstrap nodes (empty for bootstrap nodes).
func MapFlagsAndEnvToConfig(cfg *config.Config, fv NodeFlagValues, isBootstrap bool, logger *logging.StandardLogger) string {
	// Apply environment variable overrides first so that flags can override them after
	config.ApplyEnvOverrides(cfg)

	// Determine data directory if not provided
	if fv.DataDir == "" {
		if isBootstrap {
			fv.DataDir = "./data/bootstrap"
		} else {
			if fv.NodeID != "" {
				fv.DataDir = fmt.Sprintf("./data/node-%s", fv.NodeID)
			} else {
				fv.DataDir = "./data/node"
			}
		}
	}

	// Node basics
	cfg.Node.DataDir = fv.DataDir
	cfg.Node.ListenAddresses = []string{
		fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", fv.P2PPort),
		fmt.Sprintf("/ip4/0.0.0.0/udp/%d/quic", fv.P2PPort),
	}

	// Database port settings
	cfg.Database.RQLitePort = fv.RqlHTTP
	cfg.Database.RQLiteRaftPort = fv.RqlRaft
	cfg.Database.AdvertiseMode = strings.ToLower(fv.Advertise)
	logger.Printf("RQLite advertise mode: %s", cfg.Database.AdvertiseMode)

	// Bootstrap-specific vs regular-node logic
	if isBootstrap {
		bootstrapPeers := constants.GetBootstrapPeers()
		isSecondaryBootstrap := false
		if len(bootstrapPeers) > 1 {
			for i := 1; i < len(bootstrapPeers); i++ {
				host := parseHostFromMultiaddr(bootstrapPeers[i])
				if host != "" && isLocalIP(host) {
					isSecondaryBootstrap = true
					break
				}
			}
		}

		if isSecondaryBootstrap {
			primaryBootstrapHost := parseHostFromMultiaddr(bootstrapPeers[0])
			cfg.Database.RQLiteJoinAddress = fmt.Sprintf("%s:%d", primaryBootstrapHost, 7001)
			logger.Printf("Secondary bootstrap node - joining primary bootstrap (raft) at: %s", cfg.Database.RQLiteJoinAddress)
		} else {
			cfg.Database.RQLiteJoinAddress = ""
			logger.Printf("Primary bootstrap node - starting new RQLite cluster")
		}

		return ""
	}

	// Regular node: compute bootstrap peers and join address
	var rqliteJoinAddr string
	if fv.Bootstrap != "" {
		cfg.Discovery.BootstrapPeers = []string{fv.Bootstrap}
		bootstrapHost := parseHostFromMultiaddr(fv.Bootstrap)
		if bootstrapHost != "" {
			if (bootstrapHost == "127.0.0.1" || strings.EqualFold(bootstrapHost, "localhost")) && cfg.Database.AdvertiseMode != "localhost" {
				if extIP, err := getPreferredLocalIP(); err == nil && extIP != "" {
					logger.Printf("Translating localhost bootstrap to external IP %s for RQLite join", extIP)
					bootstrapHost = extIP
				} else {
					logger.Printf("Warning: Failed to resolve external IP, keeping localhost for RQLite join")
				}
			}
			rqliteJoinAddr = fmt.Sprintf("%s:%d", bootstrapHost, 7001)
			logger.Printf("Using extracted bootstrap host %s for RQLite Raft join (port 7001)", bootstrapHost)
		} else {
			logger.Printf("Warning: Could not extract host from bootstrap peer %s, using localhost fallback", fv.Bootstrap)
			rqliteJoinAddr = fmt.Sprintf("localhost:%d", 7001)
		}
		logger.Printf("Using command line bootstrap peer: %s", fv.Bootstrap)
	} else {
		bootstrapPeers := cfg.Discovery.BootstrapPeers
		if len(bootstrapPeers) == 0 {
			bootstrapPeers = constants.GetBootstrapPeers()
		}
		if len(bootstrapPeers) > 0 {
			cfg.Discovery.BootstrapPeers = bootstrapPeers
			bootstrapHost := parseHostFromMultiaddr(bootstrapPeers[0])
			if bootstrapHost != "" {
				rqliteJoinAddr = fmt.Sprintf("%s:%d", bootstrapHost, 7001)
				logger.Printf("Using extracted bootstrap host %s for RQLite Raft join", bootstrapHost)
			} else {
				logger.Printf("Warning: Could not extract host from bootstrap peer %s", bootstrapPeers[0])
				rqliteJoinAddr = "localhost:7001"
			}
			logger.Printf("Using environment bootstrap peers: %v", bootstrapPeers)
		} else {
			logger.Printf("Warning: No bootstrap peers configured")
			rqliteJoinAddr = "localhost:7001"
			logger.Printf("Using localhost fallback for RQLite Raft join")
		}

		logger.Printf("=== NETWORK DIAGNOSTICS ===")
		logger.Printf("Target RQLite Raft join address: %s", rqliteJoinAddr)
		runNetworkDiagnostics(rqliteJoinAddr, logger)
	}

	cfg.Database.RQLiteJoinAddress = rqliteJoinAddr
	logger.Printf("Regular node - joining RQLite cluster (raft) at: %s", cfg.Database.RQLiteJoinAddress)
	return rqliteJoinAddr
}
