package gateway

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/DeBrosOfficial/network/pkg/client"
	"github.com/DeBrosOfficial/network/pkg/logging"
	"github.com/multiformats/go-multiaddr"
	"go.uber.org/zap"
)

// Config holds configuration for the gateway server
type Config struct {
	ListenAddr      string
	ClientNamespace string
	BootstrapPeers  []string
}

type Gateway struct {
	logger     *logging.ColoredLogger
	cfg        *Config
	client     client.NetworkClient
	startedAt  time.Time
	signingKey *rsa.PrivateKey
	keyID      string
}

// deriveRQLiteEndpoints extracts IP addresses from bootstrap peer multiaddrs
// and constructs RQLite HTTP endpoints using the fixed system database port (5001)
func deriveRQLiteEndpoints(bootstrapPeers []string, systemHTTPPort int) []string {
	if systemHTTPPort == 0 {
		systemHTTPPort = 5001 // default
	}

	endpoints := make([]string, 0)
	seen := make(map[string]bool)

	for _, peerAddr := range bootstrapPeers {
		ma, err := multiaddr.NewMultiaddr(peerAddr)
		if err != nil {
			continue
		}

		// Extract IP address from multiaddr
		var ip string
		multiaddr.ForEach(ma, func(c multiaddr.Component) bool {
			if c.Protocol().Code == multiaddr.P_IP4 {
				ip = c.Value()
				return false // stop iteration
			}
			if c.Protocol().Code == multiaddr.P_IP6 {
				ip = "[" + c.Value() + "]" // IPv6 needs brackets
				return false
			}
			return true
		})

		if ip != "" && !seen[ip] {
			endpoint := fmt.Sprintf("http://%s:%d", ip, systemHTTPPort)
			endpoints = append(endpoints, endpoint)
			seen[ip] = true
		}
	}

	return endpoints
}

// New creates and initializes a new Gateway instance
func New(logger *logging.ColoredLogger, cfg *Config) (*Gateway, error) {
	logger.ComponentInfo(logging.ComponentGeneral, "Building client config...")

	// Build client config from gateway cfg
	// Gateway uses the system database for API keys, wallets, etc.
	cliCfg := client.DefaultClientConfig("_system")
	cliCfg.DatabaseName = "_system" // Override to use system database directly
	if len(cfg.BootstrapPeers) > 0 {
		cliCfg.BootstrapPeers = cfg.BootstrapPeers
	}

	// Derive RQLite endpoints from bootstrap peers
	// Check for env override first
	if envDSN := strings.TrimSpace(os.Getenv("GATEWAY_RQLITE_DSN")); envDSN != "" {
		cliCfg.DatabaseEndpoints = strings.Split(envDSN, ",")
		for i, ep := range cliCfg.DatabaseEndpoints {
			cliCfg.DatabaseEndpoints[i] = strings.TrimSpace(ep)
		}
		logger.ComponentInfo(logging.ComponentGeneral, "Using RQLite endpoints from GATEWAY_RQLITE_DSN env",
			zap.Strings("endpoints", cliCfg.DatabaseEndpoints))
	} else {
		// Auto-derive from bootstrap peers + system port (5001)
		// This will try port 5001 on each peer (works for single-node and distributed clusters)
		// For multi-node localhost, set GATEWAY_RQLITE_DSN to the actual ports
		cliCfg.DatabaseEndpoints = deriveRQLiteEndpoints(cfg.BootstrapPeers, 5001)
		logger.ComponentInfo(logging.ComponentGeneral, "Derived RQLite endpoints from bootstrap peers",
			zap.Strings("endpoints", cliCfg.DatabaseEndpoints),
			zap.String("note", "For multi-node localhost, set GATEWAY_RQLITE_DSN env to actual ports"))
	}

	logger.ComponentInfo(logging.ComponentGeneral, "Creating network client...")
	c, err := client.NewClient(cliCfg)
	if err != nil {
		logger.ComponentError(logging.ComponentClient, "failed to create network client", zap.Error(err))
		return nil, err
	}

	logger.ComponentInfo(logging.ComponentGeneral, "Connecting network client...")
	if err := c.Connect(); err != nil {
		logger.ComponentError(logging.ComponentClient, "failed to connect network client", zap.Error(err))
		return nil, err
	}

	logger.ComponentInfo(logging.ComponentClient, "Network client connected",
		zap.String("namespace", cliCfg.AppName),
		zap.Int("bootstrap_peer_count", len(cliCfg.BootstrapPeers)),
	)

	logger.ComponentInfo(logging.ComponentGeneral, "Creating gateway instance...")
	gw := &Gateway{
		logger:    logger,
		cfg:       cfg,
		client:    c,
		startedAt: time.Now(),
	}

	logger.ComponentInfo(logging.ComponentGeneral, "Generating RSA signing key...")
	// Generate local RSA signing key for JWKS/JWT (ephemeral for now)
	if key, err := rsa.GenerateKey(rand.Reader, 2048); err == nil {
		gw.signingKey = key
		gw.keyID = "gw-" + strconv.FormatInt(time.Now().Unix(), 10)
		logger.ComponentInfo(logging.ComponentGeneral, "RSA key generated successfully")
	} else {
		logger.ComponentWarn(logging.ComponentGeneral, "failed to generate RSA key; jwks will be empty", zap.Error(err))
	}

	logger.ComponentInfo(logging.ComponentGeneral, "Gateway initialized with dynamic database clustering")

	logger.ComponentInfo(logging.ComponentGeneral, "Gateway creation completed, returning...")
	return gw, nil
}

// withInternalAuth creates a context for internal gateway operations that bypass authentication
func (g *Gateway) withInternalAuth(ctx context.Context) context.Context {
	return client.WithInternalAuth(ctx)
}

// Close disconnects the gateway client
func (g *Gateway) Close() {
	if g.client != nil {
		if err := g.client.Disconnect(); err != nil {
			g.logger.ComponentWarn(logging.ComponentClient, "error during client disconnect", zap.Error(err))
		}
	}
	// No legacy database connections to close
}
