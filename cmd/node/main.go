package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/DeBrosOfficial/network/pkg/config"
	"github.com/DeBrosOfficial/network/pkg/logging"
	"github.com/DeBrosOfficial/network/pkg/node"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

// setup_logger initializes a logger for the given component.
func setup_logger(component logging.Component) (logger *logging.ColoredLogger) {
	var err error

	logger, err = logging.NewColoredLogger(component, true)
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}

	return logger
}

// parse_and_return_network_flags it initializes all the network flags coming from the .yaml files
func parse_and_return_network_flags() (configPath *string, dataDir, nodeID *string, p2pPort, rqlHTTP, rqlRaft *int, rqlJoinAddr *string, advAddr, httpAdvAddr, raftAdvAddr *string, help *bool) {
	logger := setup_logger(logging.ComponentNode)

	configPath = flag.String("config", "", "Path to config YAML file (overrides defaults)")
	dataDir = flag.String("data", "", "Data directory (auto-detected if not provided)")
	nodeID = flag.String("id", "", "Node identifier (for running multiple local nodes)")
	p2pPort = flag.Int("p2p-port", 4001, "LibP2P listen port")
	rqlHTTP = flag.Int("rqlite-http-port", 5001, "RQLite HTTP API port")
	rqlRaft = flag.Int("rqlite-raft-port", 7001, "RQLite Raft port")
	rqlJoinAddr = flag.String("rqlite-join-address", "", "RQLite address to join (e.g., /ip4/)")
	advAddr = flag.String("adv-addr", "127.0.0.1", "Default Advertise address for rqlite and rafts")
	httpAdvAddr = flag.String("http-adv-addr", "", "HTTP advertise address (overrides adv-addr for HTTP)")
	raftAdvAddr = flag.String("raft-adv-addr", "", "Raft advertise address (overrides adv-addr for Raft)")
	help = flag.Bool("help", false, "Show help")
	flag.Parse()

	logger.Info("Successfully parsed all flags and arguments.")

	// Always return the parsed command line flags
	// Config file loading will be handled separately in main()
	return
}

// LoadConfigFromYAML loads a config from a YAML file
func LoadConfigFromYAML(path string) (*config.Config, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	var cfg config.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML: %w", err)
	}
	return &cfg, nil
}

// check_if_should_open_help checks if the help flag is set and opens the help if it is
func check_if_should_open_help(help *bool) {
	if *help {
		flag.Usage()
		return
	}
}

// select_data_dir selects the data directory for the node
func select_data_dir(dataDir *string, nodeID *string) {
	logger := setup_logger(logging.ComponentNode)

	if *nodeID == "" {
		*dataDir = "./data/node"
	}

	logger.Info("Successfully selected Data Directory of: %s", zap.String("dataDir", *dataDir))
}

// startNode starts the node with the given configuration and port
func startNode(ctx context.Context, cfg *config.Config, port int) error {
	logger := setup_logger(logging.ComponentNode)

	n, err := node.NewNode(cfg)
	if err != nil {
		logger.Error("failed to create node: %v", zap.Error(err))
	}

	if err := n.Start(ctx); err != nil {
		logger.Error("failed to start node: %v", zap.Error(err))
	}

	// Save the peer ID to a file for CLI access (especially useful for bootstrap)
	peerID := n.GetPeerID()
	peerInfoFile := filepath.Join(cfg.Node.DataDir, "peer.info")
	peerMultiaddr := fmt.Sprintf("/ip4/0.0.0.0/tcp/%d/p2p/%s", port, peerID)

	if err := os.WriteFile(peerInfoFile, []byte(peerMultiaddr), 0644); err != nil {
		logger.Error("Failed to save peer info: %v", zap.Error(err))
	} else {
		logger.Info("Peer info saved to: %s", zap.String("path", peerInfoFile))
		logger.Info("Bootstrap multiaddr: %s", zap.String("path", peerMultiaddr))
	}

	logger.Info("Node started successfully")

	// Wait for context cancellation
	<-ctx.Done()

	// Stop node
	return n.Stop()
}

// load_args_into_config applies command line argument overrides to the config
func load_args_into_config(cfg *config.Config, p2pPort, rqlHTTP, rqlRaft *int, rqlJoinAddr *string, advAddr *string, dataDir *string) {
	logger := setup_logger(logging.ComponentNode)

	// Apply RQLite HTTP port override
	if *rqlHTTP != 5001 {
		cfg.Database.RQLitePort = *rqlHTTP
		logger.ComponentInfo(logging.ComponentNode, "Overriding RQLite HTTP port", zap.Int("port", *rqlHTTP))
	}

	// Apply RQLite Raft port override
	if *rqlRaft != 7001 {
		cfg.Database.RQLiteRaftPort = *rqlRaft
		logger.ComponentInfo(logging.ComponentNode, "Overriding RQLite Raft port", zap.Int("port", *rqlRaft))
	}

	// Apply P2P port override
	if *p2pPort != 4001 {
		cfg.Node.ListenAddresses = []string{
			fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", *p2pPort),
		}
		logger.ComponentInfo(logging.ComponentNode, "Overriding P2P port", zap.Int("port", *p2pPort))
	}

	// Apply RQLite join address
	if *rqlJoinAddr != "" {
		cfg.Database.RQLiteJoinAddress = *rqlJoinAddr
		logger.ComponentInfo(logging.ComponentNode, "Setting RQLite join address", zap.String("address", *rqlJoinAddr))
	}

	// Only override advertise addresses if they're not already set in config and advAddr is not the default
	if *advAddr != "" && *advAddr != "127.0.0.1" {
		cfg.Discovery.HttpAdvAddress = fmt.Sprintf("%s:%d", *advAddr, *rqlHTTP)
		cfg.Discovery.RaftAdvAddress = fmt.Sprintf("%s:%d", *advAddr, *rqlRaft)
		logger.ComponentInfo(logging.ComponentNode, "Overriding advertise addresses",
			zap.String("http_adv_addr", cfg.Discovery.HttpAdvAddress),
			zap.String("raft_adv_addr", cfg.Discovery.RaftAdvAddress))
	}

	if *dataDir != "" {
		cfg.Node.DataDir = *dataDir
	}
}

// load_args_into_config_from_yaml applies only explicit command line overrides when loading from YAML
func load_args_into_config_from_yaml(cfg *config.Config, p2pPort, rqlHTTP, rqlRaft *int, rqlJoinAddr, advAddr, httpAdvAddr, raftAdvAddr, dataDir *string) {
	logger := setup_logger(logging.ComponentNode)

	// Only override if explicitly set via command line (check if flag was actually provided)
	if *dataDir != "" {
		cfg.Node.DataDir = *dataDir
		logger.ComponentInfo(logging.ComponentNode, "Overriding data directory from command line", zap.String("dataDir", *dataDir))
	}

	// Check environment variable for RQLite join address (Docker-specific)
	if rqlJoinEnv := os.Getenv("RQLITE_JOIN_ADDRESS"); rqlJoinEnv != "" {
		cfg.Database.RQLiteJoinAddress = rqlJoinEnv
		logger.ComponentInfo(logging.ComponentNode, "Setting RQLite join address from environment", zap.String("address", rqlJoinEnv))
	}

	// Handle RQLite join address override from command line (takes precedence over env var)
	if *rqlJoinAddr != "" {
		cfg.Database.RQLiteJoinAddress = *rqlJoinAddr
		logger.ComponentInfo(logging.ComponentNode, "Setting RQLite join address from command line", zap.String("address", *rqlJoinAddr))
	}

	// Check environment variables for advertise addresses (Docker-specific)
	if httpAdvEnv := os.Getenv("HTTP_ADV_ADDRESS"); httpAdvEnv != "" {
		cfg.Discovery.HttpAdvAddress = httpAdvEnv
		logger.ComponentInfo(logging.ComponentNode, "Setting HTTP advertise address from environment", zap.String("http_adv_addr", httpAdvEnv))
	}
	if raftAdvEnv := os.Getenv("RAFT_ADV_ADDRESS"); raftAdvEnv != "" {
		cfg.Discovery.RaftAdvAddress = raftAdvEnv
		logger.ComponentInfo(logging.ComponentNode, "Setting Raft advertise address from environment", zap.String("raft_adv_addr", raftAdvEnv))
	}

	// Handle specific advertise address overrides from command line (takes precedence over env vars)
	if *httpAdvAddr != "" {
		cfg.Discovery.HttpAdvAddress = *httpAdvAddr
		logger.ComponentInfo(logging.ComponentNode, "Overriding HTTP advertise address from command line", zap.String("http_adv_addr", *httpAdvAddr))
	}
	if *raftAdvAddr != "" {
		cfg.Discovery.RaftAdvAddress = *raftAdvAddr
		logger.ComponentInfo(logging.ComponentNode, "Overriding Raft advertise address from command line", zap.String("raft_adv_addr", *raftAdvAddr))
	}

	// Handle general advertise address (only if specific ones weren't set)
	if *advAddr != "" && *advAddr != "127.0.0.1" && *httpAdvAddr == "" && *raftAdvAddr == "" && os.Getenv("HTTP_ADV_ADDRESS") == "" && os.Getenv("RAFT_ADV_ADDRESS") == "" {
		cfg.Discovery.HttpAdvAddress = fmt.Sprintf("%s:%d", *advAddr, *rqlHTTP)
		cfg.Discovery.RaftAdvAddress = fmt.Sprintf("%s:%d", *advAddr, *rqlRaft)
		logger.ComponentInfo(logging.ComponentNode, "Overriding advertise addresses from adv-addr flag",
			zap.String("http_adv_addr", cfg.Discovery.HttpAdvAddress),
			zap.String("raft_adv_addr", cfg.Discovery.RaftAdvAddress))
	}

	logger.ComponentInfo(logging.ComponentNode, "YAML configuration loaded with overrides applied")
}

func main() {
	logger := setup_logger(logging.ComponentNode)

	configPath, dataDir, nodeID, p2pPort, rqlHTTP, rqlRaft, rqlJoinAddr, advAddr, httpAdvAddr, raftAdvAddr, help := parse_and_return_network_flags()

	check_if_should_open_help(help)
	select_data_dir(dataDir, nodeID)

	// Load Node Configuration
	var cfg *config.Config
	if *configPath != "" {
		// Load config from YAML file
		var err error
		cfg, err = LoadConfigFromYAML(*configPath)
		if err != nil {
			logger.Error("Failed to load config from YAML", zap.Error(err))
			os.Exit(1)
		}
		logger.ComponentInfo(logging.ComponentNode, "Configuration loaded from YAML file", zap.String("path", *configPath))

		// Only apply command line overrides if they were explicitly set (not defaults)
		load_args_into_config_from_yaml(cfg, p2pPort, rqlHTTP, rqlRaft, rqlJoinAddr, advAddr, httpAdvAddr, raftAdvAddr, dataDir)
	} else {
		cfg = config.DefaultConfig()
		logger.ComponentInfo(logging.ComponentNode, "Default configuration loaded successfully")

		// Apply command line argument overrides
		load_args_into_config(cfg, p2pPort, rqlHTTP, rqlRaft, rqlJoinAddr, advAddr, dataDir)
	}
	logger.ComponentInfo(logging.ComponentNode, "Command line arguments applied to configuration")

	// LibP2P uses configurable port (default 4001); RQLite uses 5001 (HTTP) and 7001 (Raft)
	port := *p2pPort

	logger.ComponentInfo(logging.ComponentNode, "Node configuration summary",
		zap.Strings("listen_addresses", cfg.Node.ListenAddresses),
		zap.Int("rqlite_http_port", cfg.Database.RQLitePort),
		zap.Int("rqlite_raft_port", cfg.Database.RQLiteRaftPort),
		zap.Int("p2p_port", port),
		zap.Strings("bootstrap_peers", cfg.Discovery.BootstrapPeers),
		zap.String("rqlite_join_address", cfg.Database.RQLiteJoinAddress),
		zap.String("data_directory", *dataDir))

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start node in a goroutine
	errChan := make(chan error, 1)
	doneChan := make(chan struct{})
	go func() {
		if err := startNode(ctx, cfg, port); err != nil {
			errChan <- err
		}
		close(doneChan)
	}()

	// Wait for interrupt signal or error
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errChan:
		logger.ComponentError(logging.ComponentNode, "Failed to start node", zap.Error(err))
		os.Exit(1)
	case <-c:
		logger.ComponentInfo(logging.ComponentNode, "Shutting down node...")
		cancel()
		// Wait for node goroutine to finish cleanly
		<-doneChan
		logger.ComponentInfo(logging.ComponentNode, "Node shutdown complete")
	}
}
