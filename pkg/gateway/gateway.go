package gateway

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"strconv"
	"time"

	"git.debros.io/DeBros/network/pkg/client"
	"git.debros.io/DeBros/network/pkg/logging"
	"go.uber.org/zap"
)

// Config holds configuration for the gateway server
type Config struct {
	ListenAddr      string
	ClientNamespace string
	BootstrapPeers  []string
	RequireAuth     bool
	APIKeys         map[string]string // key -> optional namespace override
}

type Gateway struct {
	logger    *logging.ColoredLogger
	cfg       *Config
	client    client.NetworkClient
	startedAt time.Time
	signingKey *rsa.PrivateKey
	keyID      string
}

// New creates and initializes a new Gateway instance
func New(logger *logging.ColoredLogger, cfg *Config) (*Gateway, error) {
	// Build client config from gateway cfg
	cliCfg := client.DefaultClientConfig(cfg.ClientNamespace)
	if len(cfg.BootstrapPeers) > 0 {
		cliCfg.BootstrapPeers = cfg.BootstrapPeers
	}

	c, err := client.NewClient(cliCfg)
	if err != nil {
		logger.ComponentError(logging.ComponentClient, "failed to create network client", zap.Error(err))
		return nil, err
	}

	if err := c.Connect(); err != nil {
		logger.ComponentError(logging.ComponentClient, "failed to connect network client", zap.Error(err))
		return nil, err
	}

	logger.ComponentInfo(logging.ComponentClient, "Network client connected",
		zap.String("namespace", cliCfg.AppName),
		zap.Int("bootstrap_peer_count", len(cliCfg.BootstrapPeers)),
	)

	gw := &Gateway{
		logger:    logger,
		cfg:       cfg,
		client:    c,
		startedAt: time.Now(),
	}

	// Generate local RSA signing key for JWKS/JWT (ephemeral for now)
	if key, err := rsa.GenerateKey(rand.Reader, 2048); err == nil {
		gw.signingKey = key
		gw.keyID = "gw-" + strconv.FormatInt(time.Now().Unix(), 10)
	} else {
		logger.ComponentWarn(logging.ComponentGeneral, "failed to generate RSA key; jwks will be empty", zap.Error(err))
	}

	// Seed configured API keys into DB (best-effort)
	_ = gw.seedConfiguredAPIKeys(context.Background())

	return gw, nil
}

// Close disconnects the gateway client
func (g *Gateway) Close() {
	if g.client != nil {
		if err := g.client.Disconnect(); err != nil {
			g.logger.ComponentWarn(logging.ComponentClient, "error during client disconnect", zap.Error(err))
		}
	}
}
