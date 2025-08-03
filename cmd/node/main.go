package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"git.debros.io/DeBros/network/pkg/config"
	"git.debros.io/DeBros/network/pkg/constants"
	"git.debros.io/DeBros/network/pkg/node"
)

func main() {
	var (
		dataDir   = flag.String("data", "./data/node", "Data directory")
		port      = flag.Int("port", 4002, "Listen port")
		bootstrap = flag.String("bootstrap", "", "Bootstrap peer address")
		help      = flag.Bool("help", false, "Show help")
	)
	flag.Parse()

	if *help {
		flag.Usage()
		return
	}

	// Load configuration
	cfg := config.DefaultConfig()
	cfg.Node.DataDir = *dataDir
	cfg.Node.ListenAddresses = []string{
		fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", *port),
		fmt.Sprintf("/ip4/0.0.0.0/udp/%d/quic", *port),
	}

	// Configure RQLite ports based on node port
	cfg.Database.RQLitePort = *port + 1000     // e.g., 5002 for node port 4002
	cfg.Database.RQLiteRaftPort = *port + 3000 // e.g., 7002 for node port 4002 (changed to avoid conflicts)

	// Configure bootstrap peers
	if *bootstrap != "" {
		// Use command line bootstrap if provided
		cfg.Discovery.BootstrapPeers = []string{*bootstrap}
		log.Printf("Using command line bootstrap peer: %s", *bootstrap)
	} else {
		// Use environment-configured bootstrap peers
		bootstrapPeers := constants.GetBootstrapPeers()
		if len(bootstrapPeers) > 0 {
			cfg.Discovery.BootstrapPeers = bootstrapPeers
			log.Printf("Using environment bootstrap peers: %v", bootstrapPeers)
		} else {
			log.Printf("Warning: No bootstrap peers configured")
		}
	}

	// For LibP2P peer discovery testing, don't join RQLite cluster
	// Each node will have its own independent RQLite instance
	cfg.Database.RQLiteJoinAddress = "" // Keep RQLite independent

	log.Printf("Starting network node...")
	log.Printf("Data directory: %s", cfg.Node.DataDir)
	log.Printf("Listen addresses: %v", cfg.Node.ListenAddresses)
	log.Printf("Bootstrap peers: %v", cfg.Discovery.BootstrapPeers)
	log.Printf("RQLite HTTP port: %d", cfg.Database.RQLitePort)
	log.Printf("RQLite Raft port: %d", cfg.Database.RQLiteRaftPort)

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start node in a goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := startNode(ctx, cfg); err != nil {
			errChan <- err
		}
	}()

	// Wait for interrupt signal or error
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errChan:
		log.Fatalf("Failed to start node: %v", err)
	case <-c:
		log.Printf("Shutting down node...")
		cancel()
	}
}

func startNode(ctx context.Context, cfg *config.Config) error {
	// Create and start node
	n, err := node.NewNode(cfg)
	if err != nil {
		return fmt.Errorf("failed to create node: %w", err)
	}

	if err := n.Start(ctx); err != nil {
		return fmt.Errorf("failed to start node: %w", err)
	}

	// Wait for context cancellation
	<-ctx.Done()

	// Stop node
	return n.Stop()
}
