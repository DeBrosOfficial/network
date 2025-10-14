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
func parse_and_return_network_flags() (configPath *string, dataDir, nodeID *string, p2pPort *int, advAddr *string, help *bool, loadedConfig *config.Config) {
	logger := setup_logger(logging.ComponentNode)

	configPath = flag.String("config", "", "Path to config YAML file (overrides defaults)")
	dataDir = flag.String("data", "", "Data directory (auto-detected if not provided)")
	nodeID = flag.String("id", "", "Node identifier (for running multiple local nodes)")
	p2pPort = flag.Int("p2p-port", 4001, "LibP2P listen port")
	advAddr = flag.String("adv-addr", "0.0.0.0", "Default Advertise address for rqlite and rafts")
	help = flag.Bool("help", false, "Show help")
	flag.Parse()

	logger.Info("Successfully parsed all flags and arguments.")

	if *configPath != "" {
		cfg, err := LoadConfigFromYAML(*configPath)
		if err != nil {
			logger.Error("Failed to load config from YAML", zap.Error(err))
			os.Exit(1)
		}
		logger.ComponentInfo(logging.ComponentNode, "Configuration loaded from YAML file", zap.String("path", *configPath))

		// Return config values but preserve command line flag values for overrides
		// The command line flags will be applied later in load_args_into_config
		// Create separate variables to avoid modifying config directly
		configDataDir := cfg.Node.DataDir
		configAdvAddr := cfg.Discovery.HttpAdvAddress

		return configPath,
			&configDataDir,
			nodeID, // Keep the command line flag value, not config value
			p2pPort, // Keep the command line flag value
			&configAdvAddr,
			help,
			cfg // Return the loaded config
	}

	return configPath, dataDir, nodeID, p2pPort, advAddr, help, nil
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

	// If nodeID is provided via command line, use it to override the data directory
	if *nodeID != "" {
		*dataDir = fmt.Sprintf("./data/%s", *nodeID)
	} else if *dataDir == "" {
		// Fallback to default if neither nodeID nor dataDir is set
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
func load_args_into_config(cfg *config.Config, p2pPort *int, advAddr *string, dataDir *string) {
	logger := setup_logger(logging.ComponentNode)

	// Apply P2P port override - check if command line port differs from config
	var configPort int = 4001 // default
	if len(cfg.Node.ListenAddresses) > 0 {
		// Try to parse port from multiaddr string in config
		_, err := fmt.Sscanf(cfg.Node.ListenAddresses[0], "/ip4/0.0.0.0/tcp/%d", &configPort)
		if err != nil {
			configPort = 4001 // fallback to default
		}
	}

	// Override if command line port is different from config port
	if *p2pPort != configPort {
		cfg.Node.ListenAddresses = []string{
			fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", *p2pPort),
		}
		logger.ComponentInfo(logging.ComponentNode, "Overriding P2P port", zap.Int("port", *p2pPort))
	}

	if *advAddr != "" {
		cfg.Discovery.HttpAdvAddress = *advAddr
		cfg.Discovery.RaftAdvAddress = *advAddr
	}

	if *dataDir != "" {
		cfg.Node.DataDir = *dataDir
	}
}

func main() {
	logger := setup_logger(logging.ComponentNode)

	_, dataDir, nodeID, p2pPort, advAddr, help, loadedConfig := parse_and_return_network_flags()

	check_if_should_open_help(help)

	// Load Node Configuration - use loaded config if available, otherwise use default
	var cfg *config.Config
	if loadedConfig != nil {
		cfg = loadedConfig
		logger.ComponentInfo(logging.ComponentNode, "Using configuration from YAML file")
	} else {
		cfg = config.DefaultConfig()
		logger.ComponentInfo(logging.ComponentNode, "Using default configuration")
	}

	// Select data directory based on node ID (this overrides config)
	select_data_dir(dataDir, nodeID)

	// Apply command line argument overrides
	load_args_into_config(cfg, p2pPort, advAddr, dataDir)
	logger.ComponentInfo(logging.ComponentNode, "Command line arguments applied to configuration")

	// LibP2P uses configurable port (default 4001)
	port := *p2pPort

	logger.ComponentInfo(logging.ComponentNode, "Node configuration summary",
		zap.Strings("listen_addresses", cfg.Node.ListenAddresses),
		zap.Int("p2p_port", port),
		zap.Strings("bootstrap_peers", cfg.Discovery.BootstrapPeers),
		zap.Int("max_databases", cfg.Database.MaxDatabases),
		zap.String("port_range_http", fmt.Sprintf("%d-%d", cfg.Database.PortRangeHTTPStart, cfg.Database.PortRangeHTTPEnd)),
		zap.String("port_range_raft", fmt.Sprintf("%d-%d", cfg.Database.PortRangeRaftStart, cfg.Database.PortRangeRaftEnd)),
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
