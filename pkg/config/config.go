package config

import (
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
}

// DatabaseConfig contains database-related configuration
type DatabaseConfig struct {
	ReplicationFactor int           `yaml:"replication_factor"`
	ShardCount        int           `yaml:"shard_count"`
	MaxDatabaseSize   int64         `yaml:"max_database_size"` // In bytes
	BackupInterval    time.Duration `yaml:"backup_interval"`

	// Dynamic database clustering
	HibernationTimeout time.Duration `yaml:"hibernation_timeout"`   // Seconds before hibernation
	MaxDatabases       int           `yaml:"max_databases"`         // Max databases per node
	PortRangeHTTPStart int           `yaml:"port_range_http_start"` // HTTP port range start
	PortRangeHTTPEnd   int           `yaml:"port_range_http_end"`   // HTTP port range end
	PortRangeRaftStart int           `yaml:"port_range_raft_start"` // Raft port range start
	PortRangeRaftEnd   int           `yaml:"port_range_raft_end"`   // Raft port range end

	// System database (always-on, holds API keys, wallets, etc.)
	SystemDatabaseName string `yaml:"system_database_name"` // Name of the system database (default: "_system")
	SystemHTTPPort     int    `yaml:"rqlite_port"`          // Fixed HTTP port for _system database
	SystemRaftPort     int    `yaml:"rqlite_raft_port"`     // Fixed Raft port for _system database
	MigrationsPath     string `yaml:"migrations_path"`      // Path to SQL migrations directory
}

// DiscoveryConfig contains peer discovery configuration
type DiscoveryConfig struct {
	BootstrapPeers      []string      `yaml:"bootstrap_peers"`       // Bootstrap peer addresses
	DiscoveryInterval   time.Duration `yaml:"discovery_interval"`    // Discovery announcement interval
	BootstrapPort       int           `yaml:"bootstrap_port"`        // Default port for bootstrap nodes
	HttpAdvAddress      string        `yaml:"http_adv_address"`      // HTTP advertisement address
	RaftAdvAddress      string        `yaml:"raft_adv_address"`      // Raft advertisement
	NodeNamespace       string        `yaml:"node_namespace"`        // Namespace for node identifiers
	HealthCheckInterval time.Duration `yaml:"health_check_interval"` // Health check interval for node monitoring
}

// SecurityConfig contains security-related configuration
type SecurityConfig struct {
	EnableTLS       bool   `yaml:"enable_tls"`
	PrivateKeyFile  string `yaml:"private_key_file"`
	CertificateFile string `yaml:"certificate_file"`
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

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		Node: NodeConfig{
			Type: "node",
			ListenAddresses: []string{
				"/ip4/0.0.0.0/tcp/4001", // TCP only - compatible with Anyone proxy/SOCKS5
			},
			DataDir:        "./data",
			MaxConnections: 50,
		},
		Database: DatabaseConfig{
			ReplicationFactor: 3,
			ShardCount:        16,
			MaxDatabaseSize:   1024 * 1024 * 1024, // 1GB
			BackupInterval:    time.Hour * 24,     // Daily backups

			// Dynamic database clustering
			HibernationTimeout: 60 * time.Second,
			MaxDatabases:       100,
			PortRangeHTTPStart: 5001,
			PortRangeHTTPEnd:   5999,
			PortRangeRaftStart: 7001,
			PortRangeRaftEnd:   7999,

			// System database
			SystemDatabaseName: "_system",
			SystemHTTPPort:     5001,
			SystemRaftPort:     7001,
			MigrationsPath:     "./migrations",
		},
		Discovery: DiscoveryConfig{
			BootstrapPeers: []string{
				"/ip4/127.0.0.1/tcp/4001/p2p/12D3KooWD2GoaF6Y6XYZ9d3xKpYqGPsjcKseRGFMnpvHMjScx8w7",
				// "/ip4/217.76.54.168/tcp/4001/p2p/12D3KooWDp7xeShVY9uHfqNVPSsJeCKUatAviFZV8Y1joox5nUvx",
				// "/ip4/217.76.54.178/tcp/4001/p2p/12D3KooWKZnirPwNT4URtNSWK45f6vLkEs4xyUZ792F8Uj1oYnm1",
				// "/ip4/51.83.128.181/tcp/4001/p2p/12D3KooWBn2Zf1R8v9pEfmz7hDZ5b3oADxfejA3zJBYzKRCzgvhR",
				// "/ip4/155.133.27.199/tcp/4001/p2p/12D3KooWC69SBzM5QUgrLrfLWUykE8au32X5LwT7zwv9bixrQPm1",
				// "/ip4/217.76.56.2/tcp/4001/p2p/12D3KooWEiqJHvznxqJ5p2y8mUs6Ky6dfU1xTYFQbyKRCABfcZz4",
			},
			BootstrapPort:       4001,             // Default LibP2P port
			DiscoveryInterval:   time.Second * 15, // Back to 15 seconds for testing
			HttpAdvAddress:      "",
			RaftAdvAddress:      "",
			NodeNamespace:       "default",
			HealthCheckInterval: 10 * time.Second, // Health check interval
		},
		Security: SecurityConfig{
			EnableTLS: false,
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "console",
		},
	}
}
