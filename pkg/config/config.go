package config

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/multiformats/go-multiaddr"
)

// Config represents the main configuration for a network node
type Config struct {
	Node      NodeConfig      `yaml:"node"`
	Database  DatabaseConfig  `yaml:"database"`
	Discovery DiscoveryConfig `yaml:"discovery"`
	Security  SecurityConfig  `yaml:"security"`
	Logging   LoggingConfig   `yaml:"logging"`
}

// NodeConfig contains node-specific configuration
type NodeConfig struct {
	ID              string   `yaml:"id"`               // Auto-generated if empty
	Type            string   `yaml:"type"`             // "bootstrap" or "node"
	ListenAddresses []string `yaml:"listen_addresses"` // LibP2P listen addresses
	DataDir         string   `yaml:"data_dir"`         // Data directory
	MaxConnections  int      `yaml:"max_connections"`  // Maximum peer connections

	// Bootstrap configuration (only for bootstrap nodes)
	IsBootstrap bool `yaml:"is_bootstrap"`
}

// DatabaseConfig contains database-related configuration
type DatabaseConfig struct {
	DataDir           string        `yaml:"data_dir"`
	ReplicationFactor int           `yaml:"replication_factor"`
	ShardCount        int           `yaml:"shard_count"`
	MaxDatabaseSize   int64         `yaml:"max_database_size"` // In bytes
	BackupInterval    time.Duration `yaml:"backup_interval"`

	// RQLite-specific configuration
	RQLitePort        int    `yaml:"rqlite_port"`         // RQLite HTTP API port
	RQLiteRaftPort    int    `yaml:"rqlite_raft_port"`    // RQLite Raft consensus port
	RQLiteJoinAddress string `yaml:"rqlite_join_address"` // Address to join RQLite cluster
	AdvertiseMode     string `yaml:"advertise_mode"`      // Advertise mode: "auto" (default), "localhost", or "ip"
}

// DiscoveryConfig contains peer discovery configuration
type DiscoveryConfig struct {
	BootstrapPeers    []string      `yaml:"bootstrap_peers"`    // Bootstrap peer addresses
	EnableMDNS        bool          `yaml:"enable_mdns"`        // Enable mDNS discovery
	EnableDHT         bool          `yaml:"enable_dht"`         // Enable DHT discovery
	DHTPrefix         string        `yaml:"dht_prefix"`         // DHT protocol prefix
	DiscoveryInterval time.Duration `yaml:"discovery_interval"` // Discovery announcement interval
}

// SecurityConfig contains security-related configuration
type SecurityConfig struct {
	EnableTLS       bool   `yaml:"enable_tls"`
	PrivateKeyFile  string `yaml:"private_key_file"`
	CertificateFile string `yaml:"certificate_file"`
	AuthEnabled     bool   `yaml:"auth_enabled"`
}

// LoggingConfig contains logging configuration
type LoggingConfig struct {
	Level      string `yaml:"level"`       // debug, info, warn, error
	Format     string `yaml:"format"`      // json, console
	OutputFile string `yaml:"output_file"` // Empty for stdout
}

// ClientConfig represents configuration for network clients
type ClientConfig struct {
	AppName        string        `yaml:"app_name"`
	DatabaseName   string        `yaml:"database_name"`
	BootstrapPeers []string      `yaml:"bootstrap_peers"`
	ConnectTimeout time.Duration `yaml:"connect_timeout"`
	RetryAttempts  int           `yaml:"retry_attempts"`
}

// ParseMultiaddrs converts string addresses to multiaddr objects
func (c *Config) ParseMultiaddrs() ([]multiaddr.Multiaddr, error) {
	var addrs []multiaddr.Multiaddr
	for _, addr := range c.Node.ListenAddresses {
		ma, err := multiaddr.NewMultiaddr(addr)
		if err != nil {
			return nil, err
		}
		addrs = append(addrs, ma)
	}
	return addrs, nil
}

// GetBootstrapMultiaddrs converts bootstrap peer strings to multiaddr objects
func (c *Config) GetBootstrapMultiaddrs() ([]multiaddr.Multiaddr, error) {
	var addrs []multiaddr.Multiaddr
	for _, addr := range c.Discovery.BootstrapPeers {
		ma, err := multiaddr.NewMultiaddr(addr)
		if err != nil {
			return nil, err
		}
		addrs = append(addrs, ma)
	}
	return addrs, nil
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		Node: NodeConfig{
			Type: "node",
			ListenAddresses: []string{
				"/ip4/0.0.0.0/tcp/0",
				"/ip4/0.0.0.0/udp/0/quic",
			},
			DataDir:        "./data",
			MaxConnections: 50,
			IsBootstrap:    false,
		},
		Database: DatabaseConfig{
			DataDir:           "./data/db",
			ReplicationFactor: 3,
			ShardCount:        16,
			MaxDatabaseSize:   1024 * 1024 * 1024, // 1GB
			BackupInterval:    time.Hour * 24,     // Daily backups

			// RQLite-specific configuration
			RQLitePort:        5001,
			RQLiteRaftPort:    7001,
			RQLiteJoinAddress: "", // Empty for bootstrap node
			AdvertiseMode:     "auto",
		},
		Discovery: DiscoveryConfig{
			BootstrapPeers:    []string{},
			EnableMDNS:        true,
			EnableDHT:         true,
			DHTPrefix:         "/network/kad/1.0.0",
			DiscoveryInterval: time.Minute * 5,
		},
		Security: SecurityConfig{
			EnableTLS:   false,
			AuthEnabled: false,
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "console",
		},
	}
}

// BootstrapConfig returns a default configuration for bootstrap nodes
func BootstrapConfig() *Config {
	config := DefaultConfig()
	config.Node.Type = "bootstrap"
	config.Node.IsBootstrap = true
	config.Node.ListenAddresses = []string{
		"/ip4/0.0.0.0/tcp/4001",
		"/ip4/0.0.0.0/udp/4001/quic",
	}
	return config
}

// NewConfigFromEnv constructs a config (bootstrap or regular) and applies environment overrides.
// If isBootstrap is true, starts from BootstrapConfig; otherwise from DefaultConfig.
func NewConfigFromEnv(isBootstrap bool) *Config {
	var cfg *Config
	if isBootstrap {
		cfg = BootstrapConfig()
	} else {
		cfg = DefaultConfig()
	}
	ApplyEnvOverrides(cfg)
	return cfg
}

// ApplyEnvOverrides mutates cfg based on environment variables.
// Precedence: CLI flags (outside this function) > ENV variables > defaults in code.
func ApplyEnvOverrides(cfg *Config) {
	// Node
	if v := os.Getenv("NODE_ID"); v != "" {
		cfg.Node.ID = v
	}
	if v := os.Getenv("NODE_TYPE"); v != "" { // "bootstrap" or "node"
		cfg.Node.Type = strings.ToLower(v)
		cfg.Node.IsBootstrap = cfg.Node.Type == "bootstrap"
	}
	if v := os.Getenv("NODE_LISTEN_ADDRESSES"); v != "" {
		parts := splitAndTrim(v)
		if len(parts) > 0 {
			cfg.Node.ListenAddresses = parts
		}
	}
	if v := os.Getenv("DATA_DIR"); v != "" {
		cfg.Node.DataDir = v
	}
	if v := os.Getenv("MAX_CONNECTIONS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Node.MaxConnections = n
		}
	}

	// Database
	if v := os.Getenv("DB_DATA_DIR"); v != "" {
		cfg.Database.DataDir = v
	}
	if v := os.Getenv("REPLICATION_FACTOR"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Database.ReplicationFactor = n
		}
	}
	if v := os.Getenv("SHARD_COUNT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Database.ShardCount = n
		}
	}
	if v := os.Getenv("MAX_DB_SIZE"); v != "" { // bytes
		if n, err := parseInt64(v); err == nil {
			cfg.Database.MaxDatabaseSize = n
		}
	}
	if v := os.Getenv("BACKUP_INTERVAL"); v != "" { // duration, e.g. 24h
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Database.BackupInterval = d
		}
	}
	if v := os.Getenv("RQLITE_HTTP_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Database.RQLitePort = n
		}
	}
	if v := os.Getenv("RQLITE_RAFT_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Database.RQLiteRaftPort = n
		}
	}
	if v := os.Getenv("RQLITE_JOIN_ADDRESS"); v != "" {
		cfg.Database.RQLiteJoinAddress = v
	}
	if v := os.Getenv("ADVERTISE_MODE"); v != "" { // auto | localhost | ip
		cfg.Database.AdvertiseMode = strings.ToLower(v)
	}

	// Discovery
	if v := os.Getenv("BOOTSTRAP_PEERS"); v != "" {
		parts := splitAndTrim(v)
		if len(parts) > 0 {
			cfg.Discovery.BootstrapPeers = parts
		}
	}
	if v := os.Getenv("ENABLE_MDNS"); v != "" {
		if b, err := parseBool(v); err == nil {
			cfg.Discovery.EnableMDNS = b
		}
	}
	if v := os.Getenv("ENABLE_DHT"); v != "" {
		if b, err := parseBool(v); err == nil {
			cfg.Discovery.EnableDHT = b
		}
	}
	if v := os.Getenv("DHT_PREFIX"); v != "" {
		cfg.Discovery.DHTPrefix = v
	}
	if v := os.Getenv("DISCOVERY_INTERVAL"); v != "" { // e.g. 5m
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Discovery.DiscoveryInterval = d
		}
	}

	// Security
	if v := os.Getenv("ENABLE_TLS"); v != "" {
		if b, err := parseBool(v); err == nil {
			cfg.Security.EnableTLS = b
		}
	}
	if v := os.Getenv("PRIVATE_KEY_FILE"); v != "" {
		cfg.Security.PrivateKeyFile = v
	}
	if v := os.Getenv("CERT_FILE"); v != "" {
		cfg.Security.CertificateFile = v
	}
	if v := os.Getenv("AUTH_ENABLED"); v != "" {
		if b, err := parseBool(v); err == nil {
			cfg.Security.AuthEnabled = b
		}
	}

	// Logging
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		cfg.Logging.Level = strings.ToLower(v)
	}
	if v := os.Getenv("LOG_FORMAT"); v != "" {
		cfg.Logging.Format = strings.ToLower(v)
	}
	if v := os.Getenv("LOG_OUTPUT_FILE"); v != "" {
		cfg.Logging.OutputFile = v
	}
}

// Helpers
func splitAndTrim(csv string) []string {
	parts := strings.Split(csv, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		s := strings.TrimSpace(p)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func parseBool(s string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "t", "yes", "y", "on":
		return true, nil
	case "0", "false", "f", "no", "n", "off":
		return false, nil
	default:
		return strconv.ParseBool(s)
	}
}

func parseInt64(s string) (int64, error) {
	// Allow plain int or with optional suffixes k, m, g (base-1024)
	s = strings.TrimSpace(strings.ToLower(s))
	mul := int64(1)
	if strings.HasSuffix(s, "k") {
		mul = 1024
		s = strings.TrimSuffix(s, "k")
	} else if strings.HasSuffix(s, "m") {
		mul = 1024 * 1024
		s = strings.TrimSuffix(s, "m")
	} else if strings.HasSuffix(s, "g") {
		mul = 1024 * 1024 * 1024
		s = strings.TrimSuffix(s, "g")
	}
	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return 0, err
	}
	return n * mul, nil
}
