package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"git.debros.io/DeBros/network/pkg/config"
	"git.debros.io/DeBros/network/pkg/logging"
	"git.debros.io/DeBros/network/pkg/node"
)

func main() {
	var (
		dataDir = flag.String("data", "./data/bootstrap", "Data directory")
		port    = flag.Int("port", 4001, "Listen port")
		help    = flag.Bool("help", false, "Show help")
	)
	flag.Parse()

	if *help {
		flag.Usage()
		return
	}

	// Create colored logger for bootstrap
	logger, err := logging.NewStandardLogger(logging.ComponentBootstrap)
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}

	// Load configuration
	cfg := config.BootstrapConfig()
	cfg.Node.DataDir = *dataDir
	cfg.Node.ListenAddresses = []string{
		fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", *port),
		fmt.Sprintf("/ip4/0.0.0.0/udp/%d/quic", *port),
	}

	// Configure RQLite ports for bootstrap node
	cfg.Database.RQLitePort = *port + 1000     // e.g., 5001 for bootstrap port 4001
	cfg.Database.RQLiteRaftPort = *port + 3000 // e.g., 7001 for bootstrap port 4001 (changed to avoid conflicts)
	cfg.Database.RQLiteJoinAddress = ""        // Bootstrap node doesn't join anyone

	logger.Printf("Starting bootstrap node...")
	logger.Printf("Data directory: %s", cfg.Node.DataDir)
	logger.Printf("Listen addresses: %v", cfg.Node.ListenAddresses)
	logger.Printf("RQLite HTTP port: %d", cfg.Database.RQLitePort)
	logger.Printf("RQLite Raft port: %d", cfg.Database.RQLiteRaftPort)

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start bootstrap node in a goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := startBootstrapNode(ctx, cfg, *port, logger); err != nil {
			errChan <- err
		}
	}()

	// Wait for interrupt signal or error
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errChan:
		logger.Printf("Failed to start bootstrap node: %v", err)
		os.Exit(1)
	case <-c:
		logger.Printf("Shutting down bootstrap node...")
		cancel()
	}
}

func startBootstrapNode(ctx context.Context, cfg *config.Config, port int, logger *logging.StandardLogger) error {
	// Create and start bootstrap node using the new node implementation
	n, err := node.NewNode(cfg)
	if err != nil {
		return fmt.Errorf("failed to create bootstrap node: %w", err)
	}

	if err := n.Start(ctx); err != nil {
		return fmt.Errorf("failed to start bootstrap node: %w", err)
	}

	// Save the peer ID to a file for CLI access
	peerID := n.GetPeerID()
	peerInfoFile := filepath.Join(cfg.Node.DataDir, "peer.info")
	peerMultiaddr := fmt.Sprintf("/ip4/127.0.0.1/tcp/%d/p2p/%s", port, peerID)

	if err := os.WriteFile(peerInfoFile, []byte(peerMultiaddr), 0644); err != nil {
		logger.Printf("Warning: Failed to save peer info: %v", err)
	} else {
		logger.Printf("Peer info saved to: %s", peerInfoFile)
		logger.Printf("Bootstrap multiaddr: %s", peerMultiaddr)
	}

	logger.Printf("Bootstrap node started successfully")

	// Wait for context cancellation
	<-ctx.Done()

	// Stop node
	return n.Stop()
}
