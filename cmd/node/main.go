package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"git.debros.io/DeBros/network/pkg/anyoneproxy"
	"git.debros.io/DeBros/network/pkg/client"
	"git.debros.io/DeBros/network/pkg/config"
	"git.debros.io/DeBros/network/pkg/constants"
	"git.debros.io/DeBros/network/pkg/logging"
	"git.debros.io/DeBros/network/pkg/node"
)

func main() {
	var (
		dataDir     = flag.String("data", "", "Data directory (auto-detected if not provided)")
		nodeID      = flag.String("id", "", "Node identifier (for running multiple local nodes)")
		bootstrap   = flag.String("bootstrap", "", "Bootstrap peer address (for manual override)")
		role        = flag.String("role", "auto", "Node role: auto|bootstrap|node (auto detects based on config)")
		p2pPort     = flag.Int("p2p-port", 4001, "LibP2P listen port")
		rqlHTTP     = flag.Int("rqlite-http-port", 5001, "RQLite HTTP API port")
		rqlRaft     = flag.Int("rqlite-raft-port", 7001, "RQLite Raft port")
		advertise   = flag.String("advertise", "auto", "Advertise mode: auto|localhost|ip")
		devLocal    = flag.Bool("dev-local", false, "Enable development localhost defaults for the client library (sets NETWORK_DEV_LOCAL=1)")
		disableAnon = flag.Bool("disable-anonrc", false, "Disable Anyone proxy routing (defaults to enabled on 127.0.0.1:9050)")
		help        = flag.Bool("help", false, "Show help")
	)
	flag.Parse()

	// Apply proxy disable flag early
	anyoneproxy.SetDisabled(*disableAnon)

	if *help {
		flag.Usage()
		return
	}

	// Enable development localhost defaults for the client library if requested
	if *devLocal {
		os.Setenv("NETWORK_DEV_LOCAL", "1")
		log.Printf("Development local mode enabled (NETWORK_DEV_LOCAL=1)")
	}

	// Determine node role
	var isBootstrap bool
	switch strings.ToLower(*role) {
	case "bootstrap":
		isBootstrap = true
	case "node":
		isBootstrap = false
	default:
		// Auto-detect if this is a bootstrap node based on configuration
		isBootstrap = isBootstrapNode()
	}

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

	// LibP2P uses configurable port (default 4001); RQLite uses 5001 (HTTP) and 7001 (Raft)
	port := *p2pPort

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

	// Centralized mapping from flags/env to config (flags > env > defaults)
	_ = MapFlagsAndEnvToConfig(cfg, NodeFlagValues{
		DataDir:   *dataDir,
		NodeID:    *nodeID,
		Bootstrap: *bootstrap,
		Role:      *role,
		P2PPort:   port,
		RqlHTTP:   *rqlHTTP,
		RqlRaft:   *rqlRaft,
		Advertise: *advertise,
	}, isBootstrap, logger)

	logger.Printf("Data directory: %s", cfg.Node.DataDir)
	logger.Printf("Listen addresses: %v", cfg.Node.ListenAddresses)
	logger.Printf("RQLite HTTP port: %d", cfg.Database.RQLitePort)
	logger.Printf("RQLite Raft port: %d", cfg.Database.RQLiteRaftPort)

	// For development visibility, print what the CLIENT library will return by default
	clientBootstrap := client.DefaultBootstrapPeers()
	clientDB := client.DefaultDatabaseEndpoints()
	logger.Printf("[Client Defaults] Bootstrap peers: %v", clientBootstrap)
	logger.Printf("[Client Defaults] Database endpoints: %v", clientDB)
	// Also show node-configured values
	logger.Printf("[Node Config] Bootstrap peers: %v", cfg.Discovery.BootstrapPeers)
	if cfg.Database.RQLiteJoinAddress != "" {
		logger.Printf("[Node Config] RQLite Raft join: %s", cfg.Database.RQLiteJoinAddress)
	} else if isBootstrap {
		logger.Printf("[Node Config] Bootstrap node: starting new RQLite cluster (no join)")
	}

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start node in a goroutine
	errChan := make(chan error, 1)
	doneChan := make(chan struct{})
	go func() {
		if err := startNode(ctx, cfg, port, isBootstrap, logger); err != nil {
			errChan <- err
		}
		close(doneChan)
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
		// Wait for node goroutine to finish cleanly
		<-doneChan
		logger.Printf("Node shutdown complete")
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

		// Check if this is a production bootstrap server by IP
		if host != "" && isLocalIP(host) {
			return true
		}

		// Check if this is a production bootstrap server by hostname
		if hostname != "" && strings.Contains(peerAddr, hostname) {
			return true
		}
	}

	// Default: if no specific match, run as regular node
	return false
}

// getPreferredLocalIP returns a non-loopback IPv4 address of this machine
func getPreferredLocalIP() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, iface := range ifaces {
		if (iface.Flags&net.FlagUp) == 0 || (iface.Flags&net.FlagLoopback) != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			ip = ip.To4()
			if ip == nil {
				continue
			}
			return ip.String(), nil
		}
	}
	return "", fmt.Errorf("no non-loopback IPv4 found")
}

// isLocalIP checks if the given IP address belongs to this machine
func isLocalIP(ip string) bool {
	if ip == "127.0.0.1" || strings.EqualFold(ip, "localhost") {
		return true
	}
	ifaces, err := net.Interfaces()
	if err != nil {
		return false
	}
	for _, iface := range ifaces {
		if (iface.Flags & net.FlagUp) == 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var a net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				a = v.IP
			case *net.IPAddr:
				a = v.IP
			}
			if a != nil && a.String() == ip {
				return true
			}
		}
	}
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

// runNetworkDiagnostics performs network connectivity tests
func runNetworkDiagnostics(target string, logger *logging.StandardLogger) {
	// If target has scheme, treat as HTTP URL. Otherwise treat as host:port raft.
	var host, port string
	if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
		url := strings.TrimPrefix(strings.TrimPrefix(target, "http://"), "https://")
		parts := strings.Split(url, ":")
		if len(parts) == 2 {
			host, port = parts[0], parts[1]
		}
	} else {
		parts := strings.Split(target, ":")
		if len(parts) == 2 {
			host, port = parts[0], parts[1]
		}
	}
	if host == "" || port == "" {
		logger.Printf("Cannot parse host:port from %s", target)
		return
	}

	logger.Printf("Testing TCP connectivity to %s:%s", host, port)
	if output, err := exec.Command("timeout", "5", "nc", "-z", "-v", host, port).CombinedOutput(); err == nil {
		logger.Printf("✅ Port %s:%s is reachable", host, port)
		logger.Printf("netcat output: %s", strings.TrimSpace(string(output)))
	} else {
		logger.Printf("❌ Port %s:%s is NOT reachable", host, port)
		logger.Printf("netcat error: %v", err)
		logger.Printf("netcat output: %s", strings.TrimSpace(string(output)))
	}

	// Also probe HTTP status on port 5001 of the same host, which is the default HTTP API
	httpURL := fmt.Sprintf("http://%s:%d/status", host, 5001)
	if output, err := exec.Command("timeout", "5", "curl", "-s", "-o", "/dev/null", "-w", "%{http_code}", httpURL).Output(); err == nil {
		httpCode := strings.TrimSpace(string(output))
		if httpCode == "200" {
			logger.Printf("✅ HTTP service on %s is responding correctly (status: %s)", httpURL, httpCode)
		} else {
			logger.Printf("⚠️  HTTP service on %s responded with status: %s", httpURL, httpCode)
		}
	} else {
		logger.Printf("❌ HTTP request to %s failed: %v", httpURL, err)
	}

	// Ping test
	if output, err := exec.Command("ping", "-c", "3", "-W", "2", host).Output(); err == nil {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.Contains(line, "packet loss") {
				logger.Printf("🏓 Ping result: %s", strings.TrimSpace(line))
				break
			}
		}
	} else {
		logger.Printf("❌ Ping test failed: %v", err)
	}

	// DNS resolution
	if output, err := exec.Command("nslookup", host).Output(); err == nil {
		logger.Printf("🔍 DNS resolution successful")
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.Contains(line, "Address:") && !strings.Contains(line, "#53") {
				logger.Printf("DNS result: %s", strings.TrimSpace(line))
			}
		}
	} else {
		logger.Printf("❌ DNS resolution failed: %v", err)
	}

	logger.Printf("=== END DIAGNOSTICS ===")
}
