package client

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/security/noise"
	libp2pquic "github.com/libp2p/go-libp2p/p2p/transport/quic"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	"github.com/multiformats/go-multiaddr"
	"go.uber.org/zap"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	libp2ppubsub "github.com/libp2p/go-libp2p-pubsub"

	"git.debros.io/DeBros/network/pkg/discovery"
	"git.debros.io/DeBros/network/pkg/pubsub"
	"git.debros.io/DeBros/network/pkg/storage"
)

// Client implements the NetworkClient interface
type Client struct {
	config *ClientConfig

	// Network components
	host     host.Host
	libp2pPS *libp2ppubsub.PubSub
	dht      *dht.IpfsDHT
	logger   *zap.Logger

	// Components
	database *DatabaseClientImpl
	storage  *StorageClientImpl
	network  *NetworkInfoImpl
	pubsub   *pubSubBridge

	// Managers
	discoveryMgr *discovery.Manager

	// State
	connected bool
	startTime time.Time
	mu        sync.RWMutex
}

// NewClient creates a new network client
func NewClient(config *ClientConfig) (NetworkClient, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	if config.AppName == "" {
		return nil, fmt.Errorf("app name is required")
	}

	// Create zap logger via helper for consistency
	logger, err := newClientLogger(config.QuietMode)
	if err != nil {
		return nil, fmt.Errorf("failed to create logger: %w", err)
	}

	client := &Client{
		config:    config,
		logger:    logger,
		startTime: time.Now(),
	}

	// Initialize components (will be configured when connected)
	client.database = &DatabaseClientImpl{client: client}
	client.network = &NetworkInfoImpl{client: client}

	return client, nil
}

// Database returns the database client
func (c *Client) Database() DatabaseClient {
	return c.database
}

// Storage returns the storage client
func (c *Client) Storage() StorageClient {
	return c.storage
}

// PubSub returns the pub/sub client
func (c *Client) PubSub() PubSubClient {
	return c.pubsub
}

// Network returns the network info client
func (c *Client) Network() NetworkInfo {
	return c.network
}

// Config returns a snapshot copy of the client's configuration
func (c *Client) Config() *ClientConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.config == nil {
		return nil
	}
	cp := *c.config
	if c.config.BootstrapPeers != nil {
		cp.BootstrapPeers = append([]string(nil), c.config.BootstrapPeers...)
	}
	if c.config.DatabaseEndpoints != nil {
		cp.DatabaseEndpoints = append([]string(nil), c.config.DatabaseEndpoints...)
	}
	return &cp
}

// Connect establishes connection to the network
func (c *Client) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected {
		return nil
	}

	// Create LibP2P host
	h, err := libp2p.New(
		libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0"), // Random port
		libp2p.Security(noise.ID, noise.New),
		libp2p.Transport(tcp.NewTCPTransport),
		libp2p.Transport(libp2pquic.NewTransport),
		libp2p.DefaultMuxers,
	)
	if err != nil {
		return fmt.Errorf("failed to create libp2p host: %w", err)
	}

	c.host = h

    // Log host identity and listen addresses
    addrs := c.host.Addrs()
    addrStrs := make([]string, 0, len(addrs))
    for _, a := range addrs { addrStrs = append(addrStrs, a.String()) }
    c.logger.Info("LibP2P host created",
        zap.String("peer_id", c.host.ID().String()),
        zap.Int("listen_addr_count", len(addrStrs)),
        zap.Strings("listen_addrs", addrStrs),
    )

	// Create LibP2P PubSub with enhanced discovery for Anchat
	var ps *libp2ppubsub.PubSub
	if c.config.AppName == "anchat" {
		// For Anchat, use more aggressive GossipSub settings for better peer discovery
		ps, err = libp2ppubsub.NewGossipSub(context.Background(), h,
			libp2ppubsub.WithPeerExchange(true), // Enable peer exchange
			libp2ppubsub.WithFloodPublish(true), // Flood publish for small networks
		)
	} else {
		// Standard GossipSub for other applications
		ps, err = libp2ppubsub.NewGossipSub(context.Background(), h)
	}
	if err != nil {
		h.Close()
		return fmt.Errorf("failed to create pubsub: %w", err)
	}
	c.libp2pPS = ps

	// Create pubsub bridge once and store it
	adapter := pubsub.NewClientAdapter(c.libp2pPS, c.getAppNamespace())
	c.pubsub = &pubSubBridge{adapter: adapter}

	// Create DHT for peer discovery - Use server mode for better peer discovery in small networks
	kademliaDHT, err := dht.New(context.Background(), h, dht.Mode(dht.ModeServer))
	if err != nil {
		h.Close()
		return fmt.Errorf("failed to create DHT: %w", err)
	}
	c.dht = kademliaDHT

	// Create storage client with the host
	storageClient := storage.NewClient(h, c.getAppNamespace(), c.logger)
	c.storage = &StorageClientImpl{
		client:        c,
		storageClient: storageClient,
	}

	// Connect to bootstrap peers FIRST
	ctx, cancel := context.WithTimeout(context.Background(), c.config.ConnectTimeout)
	defer cancel()

	bootstrapPeersConnected := 0
	for _, bootstrapAddr := range c.config.BootstrapPeers {
		if err := c.connectToBootstrap(ctx, bootstrapAddr); err != nil {
			c.logger.Warn("Failed to connect to bootstrap peer",
				zap.String("addr", bootstrapAddr),
				zap.Error(err))
			continue
		}
		bootstrapPeersConnected++
	}

	if bootstrapPeersConnected == 0 {
		c.logger.Warn("No bootstrap peers connected, continuing anyway")
	}

	// Add bootstrap peers to DHT routing table explicitly BEFORE bootstrapping
	for _, bootstrapAddr := range c.config.BootstrapPeers {
		if ma, err := multiaddr.NewMultiaddr(bootstrapAddr); err == nil {
			if peerInfo, err := peer.AddrInfoFromP2pAddr(ma); err == nil {
				c.host.Peerstore().AddAddrs(peerInfo.ID, peerInfo.Addrs, time.Hour*24)

				// Force add to DHT routing table
				if added, err := c.dht.RoutingTable().TryAddPeer(peerInfo.ID, true, true); err == nil && added {
					c.logger.Debug("Added bootstrap peer to DHT routing table",
						zap.String("peer", peerInfo.ID.String()))
				}
			}
		}
	}

	// Bootstrap the DHT AFTER connecting to bootstrap peers
	if err = kademliaDHT.Bootstrap(context.Background()); err != nil {
		c.logger.Warn("Failed to bootstrap DHT", zap.Error(err))
		// Don't fail - continue without DHT
	} else {
		c.logger.Debug("DHT bootstrap initiated successfully")
	}

	// Initialize discovery manager
	c.discoveryMgr = discovery.NewManager(c.host, c.dht, c.logger)

	// Start peer discovery
	discoveryConfig := discovery.Config{
		DiscoveryInterval: 5 * time.Second, // More frequent discovery
		MaxConnections:    10,              // Allow more connections
	}
	if err := c.discoveryMgr.Start(discoveryConfig); err != nil {
		c.logger.Warn("Failed to start peer discovery", zap.Error(err))
	}

    // Start generic aggressive peer discovery for all apps
    go c.startAggressivePeerDiscovery()

	// Start connection monitoring
	c.startConnectionMonitoring()

	c.connected = true

    c.logger.Info("Client connected", zap.String("namespace", c.getAppNamespace()))

	return nil
}

// Disconnect closes the connection to the network
func (c *Client) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return nil
	}

	// Stop peer discovery
	if c.discoveryMgr != nil {
		c.discoveryMgr.Stop()
	}

	// Close pubsub adapter
	if c.pubsub != nil && c.pubsub.adapter != nil {
		if err := c.pubsub.adapter.Close(); err != nil {
			c.logger.Error("Failed to close pubsub adapter", zap.Error(err))
		}
		c.pubsub = nil
	}

	// Close DHT
	if c.dht != nil {
		if err := c.dht.Close(); err != nil {
			c.logger.Error("Failed to close DHT", zap.Error(err))
		}
	}

	// Close LibP2P host
	if c.host != nil {
		if err := c.host.Close(); err != nil {
			c.logger.Error("Failed to close host", zap.Error(err))
		}
	}

	c.connected = false

    c.logger.Info("Client disconnected")

	return nil
}

// Health returns the current health status
func (c *Client) Health() (*HealthStatus, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	status := "healthy"
	if !c.connected {
		status = "unhealthy"
	}

	checks := map[string]string{
		"connection": "ok",
		"database":   "ok",
		"storage":    "ok",
		"pubsub":     "ok",
	}

	if !c.connected {
		checks["connection"] = "disconnected"
	}

	return &HealthStatus{
		Status:       status,
		Checks:       checks,
		LastUpdated:  time.Now(),
		ResponseTime: time.Millisecond * 10, // Simulated
	}, nil
}

// isConnected checks if the client is connected
func (c *Client) isConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// getAppNamespace returns the namespace for this app
func (c *Client) getAppNamespace() string {
	return c.config.AppName
}
