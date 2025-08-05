package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"git.debros.io/DeBros/network/pkg/config"
	"git.debros.io/DeBros/network/pkg/constants"
	"git.debros.io/DeBros/network/pkg/logging"
	"git.debros.io/DeBros/network/pkg/node"
)

func main() {
	var (
		dataDir   = flag.String("data", "", "Data directory (auto-detected if not provided)")
		nodeID    = flag.String("id", "", "Node identifier (for running multiple local nodes)")
		bootstrap = flag.String("bootstrap", "", "Bootstrap peer address (for manual override)")
		help      = flag.Bool("help", false, "Show help")
	)
	flag.Parse()

	if *help {
		flag.Usage()
		return
	}

	// Auto-detect if this is a bootstrap node based on configuration
	isBootstrap := isBootstrapNode()

	// Set default data directory if not provided
	if *dataDir == "" {
		if isBootstrap {
			*dataDir = "./data/bootstrap"
		} else {
			if *nodeID != "" {
				*dataDir = fmt.Sprintf("./data/node-%s", *nodeID)
			} else {
				*dataDir = "./data/node"
			}
		}
	}

	// All nodes use port 4001 for consistency
	port := 4001

	// Create logger with appropriate component type
	var logger *logging.StandardLogger
	var err error
	if isBootstrap {
		logger, err = logging.NewStandardLogger(logging.ComponentBootstrap)
	} else {
		logger, err = logging.NewStandardLogger(logging.ComponentNode)
	}
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}

	// Load configuration based on node type
	var cfg *config.Config
	if isBootstrap {
		cfg = config.BootstrapConfig()
		logger.Printf("Starting bootstrap node...")
	} else {
		cfg = config.DefaultConfig()
		logger.Printf("Starting regular node...")
	}

	// Set basic configuration
	cfg.Node.DataDir = *dataDir
	cfg.Node.ListenAddresses = []string{
		fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", port),
		fmt.Sprintf("/ip4/0.0.0.0/udp/%d/quic", port),
	}

	// All nodes use the same RQLite port (4001) to join the same cluster
	cfg.Database.RQLitePort = 4001
	cfg.Database.RQLiteRaftPort = 4002

	if isBootstrap {
		// Bootstrap node doesn't join anyone - it starts the cluster
		cfg.Database.RQLiteJoinAddress = ""
		logger.Printf("Bootstrap node - starting new RQLite cluster")
	} else {
		// Regular nodes join the bootstrap node's RQLite cluster
		cfg.Database.RQLiteJoinAddress = "http://localhost:4001"

		// Configure bootstrap peers for P2P discovery
		if *bootstrap != "" {
			// Use command line bootstrap if provided
			cfg.Discovery.BootstrapPeers = []string{*bootstrap}
			logger.Printf("Using command line bootstrap peer: %s", *bootstrap)
		} else {
			// Use environment-configured bootstrap peers
			bootstrapPeers := constants.GetBootstrapPeers()
			if len(bootstrapPeers) > 0 {
				cfg.Discovery.BootstrapPeers = bootstrapPeers
				logger.Printf("Using environment bootstrap peers: %v", bootstrapPeers)
			} else {
				logger.Printf("Warning: No bootstrap peers configured")
			}
		}
		logger.Printf("Regular node - joining RQLite cluster at: %s", cfg.Database.RQLiteJoinAddress)
	}

	logger.Printf("Data directory: %s", cfg.Node.DataDir)
	logger.Printf("Listen addresses: %v", cfg.Node.ListenAddresses)
	logger.Printf("RQLite HTTP port: %d", cfg.Database.RQLitePort)
	logger.Printf("RQLite Raft port: %d", cfg.Database.RQLiteRaftPort)

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start node in a goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := startNode(ctx, cfg, port, isBootstrap, logger); err != nil {
			errChan <- err
		}
	}()

	// Wait for interrupt signal or error
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errChan:
		logger.Printf("Failed to start node: %v", err)
		os.Exit(1)
	case <-c:
		logger.Printf("Shutting down node...")
		cancel()
	}
}

// isBootstrapNode determines if this should be a bootstrap node
// by checking the local machine's configuration and bootstrap peer list
func isBootstrapNode() bool {
	// Get the bootstrap peer addresses to check if this machine should be a bootstrap
	bootstrapPeers := constants.GetBootstrapPeers()

	// Check if any bootstrap peer is localhost/127.0.0.1 (development)
	// or if we're running on a production bootstrap server
	hostname, _ := os.Hostname()

	for _, peerAddr := range bootstrapPeers {
		// Parse the multiaddr to extract the host
		host := parseHostFromMultiaddr(peerAddr)

		// Check if this is a local bootstrap (development)
		if host == "127.0.0.1" || host == "localhost" {
			return true // In development, assume we're running the bootstrap
		}

		// Check if this is a production bootstrap server
		// You could add more sophisticated host matching here
		if hostname != "" && strings.Contains(peerAddr, hostname) {
			return true
		}
	}

	// Default: if no specific match, run as regular node
	return false
}

// parseHostFromMultiaddr extracts the host from a multiaddr
func parseHostFromMultiaddr(multiaddr string) string {
	// Simple parsing for /ip4/host/tcp/port/p2p/peerid format
	parts := strings.Split(multiaddr, "/")

	// Look for ip4/ip6/dns host in the multiaddr
	for i, part := range parts {
		if (part == "ip4" || part == "ip6" || part == "dns" || part == "dns4" || part == "dns6") && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

func startNode(ctx context.Context, cfg *config.Config, port int, isBootstrap bool, logger *logging.StandardLogger) error {
	// Create and start node using the unified node implementation
	n, err := node.NewNode(cfg)
	if err != nil {
		return fmt.Errorf("failed to create node: %w", err)
	}

	if err := n.Start(ctx); err != nil {
		return fmt.Errorf("failed to start node: %w", err)
	}

	// Save the peer ID to a file for CLI access (especially useful for bootstrap)
	if isBootstrap {
		peerID := n.GetPeerID()
		peerInfoFile := filepath.Join(cfg.Node.DataDir, "peer.info")
		peerMultiaddr := fmt.Sprintf("/ip4/127.0.0.1/tcp/%d/p2p/%s", port, peerID)

		if err := os.WriteFile(peerInfoFile, []byte(peerMultiaddr), 0644); err != nil {
			logger.Printf("Warning: Failed to save peer info: %v", err)
		} else {
			logger.Printf("Peer info saved to: %s", peerInfoFile)
			logger.Printf("Bootstrap multiaddr: %s", peerMultiaddr)
		}
	}

	logger.Printf("Node started successfully")

	// Wait for context cancellation
	<-ctx.Done()

	// Stop node
	return n.Stop()
}
