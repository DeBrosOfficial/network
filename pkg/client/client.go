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
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"

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

// pubSubBridge bridges between our PubSubClient interface and the pubsub package
type pubSubBridge struct {
	adapter *pubsub.ClientAdapter
}

func (p *pubSubBridge) Subscribe(ctx context.Context, topic string, handler MessageHandler) error {
	// Convert our MessageHandler to the pubsub package MessageHandler
	pubsubHandler := func(topic string, data []byte) error {
		return handler(topic, data)
	}
	return p.adapter.Subscribe(ctx, topic, pubsubHandler)
}

func (p *pubSubBridge) Publish(ctx context.Context, topic string, data []byte) error {
	return p.adapter.Publish(ctx, topic, data)
}

func (p *pubSubBridge) Unsubscribe(ctx context.Context, topic string) error {
	return p.adapter.Unsubscribe(ctx, topic)
}

func (p *pubSubBridge) ListTopics(ctx context.Context) ([]string, error) {
	return p.adapter.ListTopics(ctx)
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

	// For Anchat clients, ensure we connect to all other clients through bootstrap
	if c.config.AppName == "anchat" {
		// Start mDNS discovery for local network peer discovery
		go c.startMDNSDiscovery()
		go c.ensureAnchatPeerConnectivity()
	} else {
		// Start aggressive peer discovery for other apps
		go c.startAggressivePeerDiscovery()
	}

	// Start connection monitoring
	c.startConnectionMonitoring()

	c.connected = true

	return nil
}

// connectToBootstrap connects to a bootstrap peer
func (c *Client) connectToBootstrap(ctx context.Context, addr string) error {
	ma, err := multiaddr.NewMultiaddr(addr)
	if err != nil {
		return fmt.Errorf("invalid multiaddr: %w", err)
	}

	// Try to extract peer info if it's a full multiaddr with peer ID
	peerInfo, err := peer.AddrInfoFromP2pAddr(ma)
	if err != nil {
		// If there's no peer ID, try to discover the peer at this address
		return c.connectToAddress(ctx, ma)
	}

	if err := c.host.Connect(ctx, *peerInfo); err != nil {
		return fmt.Errorf("failed to connect to peer: %w", err)
	}

	c.logger.Debug("Connected to bootstrap peer",
		zap.String("peer", peerInfo.ID.String()),
		zap.String("addr", addr))

	return nil
}

// connectToAddress attempts to discover and connect to a peer at the given address
func (c *Client) connectToAddress(ctx context.Context, ma multiaddr.Multiaddr) error {
	// For the simple case, we'll just warn and continue
	// In a production environment, you'd implement proper peer discovery
	// using mDNS, DHT, or other mechanisms

	c.logger.Warn("No peer ID provided in address, skipping bootstrap connection",
		zap.String("addr", ma.String()),
		zap.String("suggestion", "Use full multiaddr with peer ID like: /ip4/127.0.0.1/tcp/4001/p2p/<peer-id>"))

	return nil // Don't fail - let the client continue without bootstrap
} // Disconnect closes the connection to the network
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

// startConnectionMonitoring monitors connection health and logs status
func (c *Client) startConnectionMonitoring() {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			if !c.isConnected() {
				return
			}

			// Remove connection status logging for cleaner output
			// connectedPeers := c.host.Network().Peers()
			// Only log if there are connection issues
			_ = c.host.Network().Peers()
		}
	}()
}

// ensureAnchatPeerConnectivity ensures Anchat clients can discover each other through bootstrap
func (c *Client) ensureAnchatPeerConnectivity() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for i := 0; i < 30; i++ { // Run for 1 minute
		<-ticker.C

		if !c.isConnected() {
			return
		}

		// Get current connected peers
		connectedPeers := c.host.Network().Peers()

		// For Anchat, we need to be very aggressive about finding other clients
		// The key insight: we need to ask our connected peers (like bootstrap) for their peers

		if c.dht != nil {
			// Try to find peers through DHT routing table
			routingPeers := c.dht.RoutingTable().ListPeers()

			for _, peerID := range routingPeers {
				if peerID == c.host.ID() {
					continue
				}

				// Check if we're already connected to this peer
				alreadyConnected := false
				for _, alreadyConnectedPeer := range connectedPeers {
					if alreadyConnectedPeer == peerID {
						alreadyConnected = true
						break
					}
				}

				if !alreadyConnected {
					// Try to connect to this peer
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					peerInfo := c.host.Peerstore().PeerInfo(peerID)

					// If we don't have addresses, try to find them through the DHT
					if len(peerInfo.Addrs) == 0 {
						if foundPeerInfo, err := c.dht.FindPeer(ctx, peerID); err == nil {
							peerInfo = foundPeerInfo
							// Add to peerstore for future use
							c.host.Peerstore().AddAddrs(peerInfo.ID, peerInfo.Addrs, time.Hour*24)
						}
					}

					if len(peerInfo.Addrs) > 0 {
						err := c.host.Connect(ctx, peerInfo)
						if err == nil {
							c.logger.Info("Anchat discovered and connected to peer",
								zap.String("peer", peerID.String()[:8]+"..."))

							// Add newly connected peer to DHT routing table
							if added, addErr := c.dht.RoutingTable().TryAddPeer(peerID, true, true); addErr == nil && added {
								c.logger.Debug("Added new peer to DHT routing table",
									zap.String("peer", peerID.String()[:8]+"..."))
							}

							// Force pubsub to recognize the new peer and form mesh connections
							if c.libp2pPS != nil {
								// Wait a moment for connection to stabilize
								time.Sleep(100 * time.Millisecond)
								// List peers to trigger mesh formation
								_ = c.libp2pPS.ListPeers("")
							}
						} else {
							c.logger.Debug("Failed to connect to discovered peer",
								zap.String("peer", peerID.String()[:8]+"..."),
								zap.Error(err))
						}
					}
					cancel()
				}
			}

			// If DHT routing table is still empty, try to force populate it
			if len(routingPeers) == 0 {
				// Try to add all connected peers to DHT routing table
				for _, connectedPeerID := range connectedPeers {
					if connectedPeerID != c.host.ID() {
						if added, err := c.dht.RoutingTable().TryAddPeer(connectedPeerID, true, true); err == nil && added {
							c.logger.Info("Force-added connected peer to DHT routing table",
								zap.String("peer", connectedPeerID.String()[:8]+"..."))
						}
					}
				}

				// Force DHT refresh
				c.dht.RefreshRoutingTable()
			}
		}

		// Also try to connect to any peers we might have in our peerstore but aren't connected to
		allKnownPeers := c.host.Peerstore().Peers()
		for _, knownPeerID := range allKnownPeers {
			if knownPeerID == c.host.ID() {
				continue
			}

			// Check if we're already connected
			alreadyConnected := false
			for _, connectedPeer := range connectedPeers {
				if connectedPeer == knownPeerID {
					alreadyConnected = true
					break
				}
			}

			if !alreadyConnected {
				// Try to connect to this known peer
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				peerInfo := c.host.Peerstore().PeerInfo(knownPeerID)
				if len(peerInfo.Addrs) > 0 {
					err := c.host.Connect(ctx, peerInfo)
					if err == nil {
						c.logger.Info("Anchat reconnected to known peer",
							zap.String("peer", knownPeerID.String()[:8]+"..."))

						// Force pubsub mesh formation
						if c.libp2pPS != nil {
							time.Sleep(100 * time.Millisecond)
							_ = c.libp2pPS.ListPeers("")
						}
					}
				}
				cancel()
			}
		}

		// Log status every 5 iterations (10 seconds)
		if i%5 == 0 && len(connectedPeers) > 0 {
			c.logger.Info("Anchat peer discovery progress",
				zap.Int("iteration", i+1),
				zap.Int("connected_peers", len(connectedPeers)),
				zap.Int("known_peers", len(allKnownPeers)))
		}
	}
} // startAggressivePeerDiscovery implements aggressive peer discovery for non-Anchat apps
func (c *Client) startAggressivePeerDiscovery() {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for i := 0; i < 20; i++ { // Run for 1 minute
		<-ticker.C

		if !c.isConnected() {
			return
		}

		// Get current connected peers
		connectedPeers := c.host.Network().Peers()

		// Try to discover more peers through the DHT
		if c.dht != nil {
			// Get peers from the DHT routing table
			routingPeers := c.dht.RoutingTable().ListPeers()

			for _, peerID := range routingPeers {
				if peerID == c.host.ID() {
					continue
				}

				// Check if we're already connected
				alreadyConnected := false
				for _, connectedPeer := range connectedPeers {
					if connectedPeer == peerID {
						alreadyConnected = true
						break
					}
				}

				if !alreadyConnected {
					// Try to connect
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					peerInfo := c.host.Peerstore().PeerInfo(peerID)
					if len(peerInfo.Addrs) > 0 {
						err := c.host.Connect(ctx, peerInfo)
						if err == nil {
							c.logger.Debug("Connected to discovered peer",
								zap.String("peer", peerID.String()[:8]+"..."))
						}
					}
					cancel()
				}
			}
		}

		// Log current status every 10 iterations (30 seconds)
		if i%10 == 0 {
			c.logger.Debug("Peer discovery status",
				zap.Int("iteration", i+1),
				zap.Int("connected_peers", len(connectedPeers)))
		}
	}
}

// startMDNSDiscovery enables mDNS peer discovery for local network
func (c *Client) startMDNSDiscovery() {
	// Setup mDNS discovery service for Anchat
	mdnsService := mdns.NewMdnsService(c.host, "anchat-p2p", &discoveryNotifee{
		client: c,
		logger: c.logger,
	})

	if err := mdnsService.Start(); err != nil {
		c.logger.Warn("Failed to start mDNS discovery", zap.Error(err))
		return
	}

	c.logger.Info("Started mDNS discovery for Anchat")
}

// discoveryNotifee handles mDNS peer discovery notifications
type discoveryNotifee struct {
	client *Client
	logger *zap.Logger
}

func (n *discoveryNotifee) HandlePeerFound(pi peer.AddrInfo) {
	n.logger.Info("mDNS discovered Anchat peer",
		zap.String("peer", pi.ID.String()[:8]+"..."),
		zap.Int("addrs", len(pi.Addrs)))

	// Connect to the discovered peer
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := n.client.host.Connect(ctx, pi); err != nil {
		n.logger.Debug("Failed to connect to mDNS discovered peer",
			zap.String("peer", pi.ID.String()[:8]+"..."),
			zap.Error(err))
	} else {
		n.logger.Info("Successfully connected to mDNS discovered peer",
			zap.String("peer", pi.ID.String()[:8]+"..."))

		// Force pubsub to recognize the new peer
		if n.client.libp2pPS != nil {
			_ = n.client.libp2pPS.ListPeers("")
		}
	}
}
