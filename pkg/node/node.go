package node

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"

	noise "github.com/libp2p/go-libp2p/p2p/security/noise"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	"github.com/multiformats/go-multiaddr"
	"go.uber.org/zap"

	"git.debros.io/DeBros/network/pkg/anyoneproxy"
	"git.debros.io/DeBros/network/pkg/config"
	"git.debros.io/DeBros/network/pkg/database"
	"git.debros.io/DeBros/network/pkg/logging"
	"git.debros.io/DeBros/network/pkg/storage"
)

// Node represents a network node with RQLite database
type Node struct {
	config *config.Config
	logger *logging.ColoredLogger
	host   host.Host

	rqliteManager  *database.RQLiteManager
	rqliteAdapter  *database.RQLiteAdapter
	storageService *storage.Service

	// Peer discovery
	discoveryCancel context.CancelFunc
}

// NewNode creates a new network node
func NewNode(cfg *config.Config) (*Node, error) {
	// Create colored logger
	logger, err := logging.NewColoredLogger(logging.ComponentNode, true)
	if err != nil {
		return nil, fmt.Errorf("failed to create logger: %w", err)
	}

	return &Node{
		config: cfg,
		logger: logger,
	}, nil
}

// startRQLite initializes and starts the RQLite database
func (n *Node) startRQLite(ctx context.Context) error {
	n.logger.Info("Starting RQLite database")

	// Create RQLite manager
	n.rqliteManager = database.NewRQLiteManager(&n.config.Database, n.config.Node.DataDir, n.logger.Logger)

	// Start RQLite
	if err := n.rqliteManager.Start(ctx); err != nil {
		return err
	}

	// Create adapter for sql.DB compatibility
	adapter, err := database.NewRQLiteAdapter(n.rqliteManager)
	if err != nil {
		return fmt.Errorf("failed to create RQLite adapter: %w", err)
	}
	n.rqliteAdapter = adapter

	return nil
}

// connectToBootstrapPeer connects to a single bootstrap peer
func (n *Node) connectToBootstrapPeer(ctx context.Context, addr string) error {
	ma, err := multiaddr.NewMultiaddr(addr)
	if err != nil {
		return fmt.Errorf("invalid multiaddr: %w", err)
	}

	// Extract peer info from multiaddr
	peerInfo, err := peer.AddrInfoFromP2pAddr(ma)
	if err != nil {
		return fmt.Errorf("failed to extract peer info: %w", err)
	}

	// Log resolved peer info prior to connect
	n.logger.ComponentDebug(logging.ComponentNode, "Resolved bootstrap peer",
		zap.String("peer_id", peerInfo.ID.String()),
		zap.String("addr", addr),
		zap.Int("addr_count", len(peerInfo.Addrs)),
	)

	// Connect to the peer
	if err := n.host.Connect(ctx, *peerInfo); err != nil {
		return fmt.Errorf("failed to connect to peer: %w", err)
	}

	n.logger.Info("Connected to bootstrap peer",
		zap.String("peer", peerInfo.ID.String()),
		zap.String("addr", addr))

	return nil
}

// connectToBootstrapPeers connects to configured LibP2P bootstrap peers
func (n *Node) connectToBootstrapPeers(ctx context.Context) error {
	if len(n.config.Discovery.BootstrapPeers) == 0 {
		n.logger.ComponentDebug(logging.ComponentNode, "No bootstrap peers configured")
		return nil
	}

	// Use passed context with a reasonable timeout for bootstrap connections
	connectCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	for _, bootstrapAddr := range n.config.Discovery.BootstrapPeers {
		if err := n.connectToBootstrapPeer(connectCtx, bootstrapAddr); err != nil {
			n.logger.ComponentWarn(logging.ComponentNode, "Failed to connect to bootstrap peer",
				zap.String("addr", bootstrapAddr),
				zap.Error(err))
			continue
		}
	}

	return nil
}

// startLibP2P initializes the LibP2P host
func (n *Node) startLibP2P() error {
	n.logger.ComponentInfo(logging.ComponentLibP2P, "Starting LibP2P host")

	// Get listen addresses
	listenAddrs, err := n.config.ParseMultiaddrs()
	if err != nil {
		return fmt.Errorf("failed to parse listen addresses: %w", err)
	}

	// Load or create persistent identity
	identity, err := n.loadOrCreateIdentity()
	if err != nil {
		return fmt.Errorf("failed to load identity: %w", err)
	}

	// Log Anyone proxy status before constructing host
	n.logger.ComponentInfo(logging.ComponentLibP2P, "Anyone proxy status",
		zap.Bool("proxy_enabled", anyoneproxy.Enabled()),
		zap.String("proxy_addr", anyoneproxy.Address()),
		zap.Bool("proxy_running", anyoneproxy.Running()),
	)

	if anyoneproxy.Enabled() && !anyoneproxy.Running() {
		n.logger.Warn("Anyone proxy is enabled but not reachable",
			zap.String("addr", anyoneproxy.Address()))
	}

	// Create LibP2P host with persistent identity
	// Build options allowing conditional proxying via Anyone SOCKS5
	var opts []libp2p.Option
	opts = append(opts,
		libp2p.Identity(identity),
		libp2p.ListenAddrs(listenAddrs...),
		libp2p.Security(noise.ID, noise.New),
		libp2p.DefaultMuxers,
	)

	// TCP transport with optional SOCKS5 dialer override
	if anyoneproxy.Enabled() {
		opts = append(opts, libp2p.Transport(tcp.NewTCPTransport, tcp.WithDialerForAddr(anyoneproxy.DialerForAddr())))
	} else {
		opts = append(opts, libp2p.Transport(tcp.NewTCPTransport))
	}

	h, err := libp2p.New(opts...)
	if err != nil {
		return err
	}

	n.host = h

	// Log configured bootstrap peers
	if len(n.config.Discovery.BootstrapPeers) > 0 {
		n.logger.ComponentInfo(logging.ComponentNode, "Configured bootstrap peers",
			zap.Strings("peers", n.config.Discovery.BootstrapPeers))
	} else {
		n.logger.ComponentDebug(logging.ComponentNode, "No bootstrap peers configured")
	}

	// Connect to LibP2P bootstrap peers if configured
	if err := n.connectToBootstrapPeers(context.Background()); err != nil {
		n.logger.ComponentWarn(logging.ComponentNode, "Failed to connect to bootstrap peers", zap.Error(err))
		// Don't fail - continue without bootstrap connections
	}

	// Background reconnect loop: keep trying to connect to bootstrap peers for a short window
	// This helps when nodes are started slightly out-of-order in dev.
	if len(n.config.Discovery.BootstrapPeers) > 0 {
		go func() {
			for i := 0; i < 12; i++ { // ~60s total
				if n.host == nil {
					return
				}
				// If we already have peers, stop retrying
				if len(n.host.Network().Peers()) > 0 {
					return
				}
				if err := n.connectToBootstrapPeers(context.Background()); err == nil {
					n.logger.ComponentDebug(logging.ComponentNode, "Bootstrap reconnect attempt completed")
				}
				time.Sleep(5 * time.Second)
			}
		}()
	}

	// Add bootstrap peers to peerstore for peer exchange
	if len(n.config.Discovery.BootstrapPeers) > 0 {
		n.logger.ComponentInfo(logging.ComponentNode, "Adding bootstrap peers to peerstore")
		for _, bootstrapAddr := range n.config.Discovery.BootstrapPeers {
			if ma, err := multiaddr.NewMultiaddr(bootstrapAddr); err == nil {
				if peerInfo, err := peer.AddrInfoFromP2pAddr(ma); err == nil {
					// Add to peerstore with longer TTL for peer exchange
					n.host.Peerstore().AddAddrs(peerInfo.ID, peerInfo.Addrs, time.Hour*24)
					n.logger.ComponentDebug(logging.ComponentNode, "Added bootstrap peer to peerstore",
						zap.String("peer", peerInfo.ID.String()))
				}
			}
		}
	}

	// DHT and routing table logic removed - using simplified peer exchange instead
	n.logger.ComponentInfo(logging.ComponentNode, "LibP2P host started successfully - using bootstrap + peer exchange discovery")

	// Start peer discovery and monitoring
	n.startPeerDiscovery()
	n.startConnectionMonitoring()

	n.logger.ComponentInfo(logging.ComponentLibP2P, "LibP2P host started with DHT enabled",
		zap.String("peer_id", h.ID().String()))

	return nil
}

// startStorageService initializes the storage service
func (n *Node) startStorageService() error {
	n.logger.ComponentInfo(logging.ComponentStorage, "Starting storage service")

	// Create storage service using the RQLite SQL adapter
	service, err := storage.NewService(n.rqliteAdapter.GetSQLDB(), n.logger.Logger)
	if err != nil {
		return err
	}

	n.storageService = service

	// Set up stream handler for storage protocol
	n.host.SetStreamHandler("/network/storage/1.0.0", n.storageService.HandleStorageStream)

	return nil
}

// loadOrCreateIdentity loads an existing identity or creates a new one
func (n *Node) loadOrCreateIdentity() (crypto.PrivKey, error) {
	identityFile := filepath.Join(n.config.Node.DataDir, "identity.key")

	// Try to load existing identity
	if _, err := os.Stat(identityFile); err == nil {
		data, err := os.ReadFile(identityFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read identity file: %w", err)
		}

		priv, err := crypto.UnmarshalPrivateKey(data)
		if err != nil {
			n.logger.Warn("Failed to unmarshal existing identity, creating new one", zap.Error(err))
		} else {
			n.logger.ComponentInfo(logging.ComponentNode, "Loaded existing identity", zap.String("file", identityFile))
			return priv, nil
		}
	}

	// Create new identity
	n.logger.Info("Creating new identity", zap.String("file", identityFile))
	priv, _, err := crypto.GenerateKeyPairWithReader(crypto.Ed25519, 2048, rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate key pair: %w", err)
	}

	// Save identity
	data, err := crypto.MarshalPrivateKey(priv)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal private key: %w", err)
	}

	if err := os.WriteFile(identityFile, data, 0600); err != nil {
		return nil, fmt.Errorf("failed to save identity: %w", err)
	}

	n.logger.Info("Identity saved", zap.String("file", identityFile))
	return priv, nil
}

// GetPeerID returns the peer ID of this node
func (n *Node) GetPeerID() string {
	if n.host == nil {
		return ""
	}
	return n.host.ID().String()
}

// startPeerDiscovery starts periodic peer discovery for the node
func (n *Node) startPeerDiscovery() {
	// Create a cancellation context for discovery
	ctx, cancel := context.WithCancel(context.Background())
	n.discoveryCancel = cancel

	// Start bootstrap peer connections immediately
	go func() {
		n.connectToBootstrapPeers(ctx)

		// Periodic peer discovery using interval from config
		ticker := time.NewTicker(n.config.Discovery.DiscoveryInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				n.discoverPeers(ctx)
			}
		}
	}()
}

// discoverPeers discovers and connects to new peers using peer exchange
func (n *Node) discoverPeers(ctx context.Context) {
	if n.host == nil {
		return
	}

	connectedPeers := n.host.Network().Peers()
	initialCount := len(connectedPeers)

	if initialCount == 0 {
		// No peers connected, try bootstrap peers again
		n.logger.ComponentInfo(logging.ComponentNode, "No peers connected, retrying bootstrap peers")
		n.connectToBootstrapPeers(ctx)
		return
	}

	n.logger.ComponentDebug(logging.ComponentNode, "Discovering peers via peer exchange",
		zap.Int("current_peers", initialCount))

	// Strategy: Use peer exchange through libp2p's identify protocol
	// LibP2P automatically exchanges peer information when peers connect
	// We just need to try connecting to peers in our peerstore

	newConnections := n.discoverViaPeerExchange(ctx)

	finalPeerCount := len(n.host.Network().Peers())

	if newConnections > 0 {
		n.logger.ComponentInfo(logging.ComponentNode, "Peer discovery completed",
			zap.Int("new_connections", newConnections),
			zap.Int("initial_peers", initialCount),
			zap.Int("final_peers", finalPeerCount))
	}
}

// discoverViaPeerExchange discovers new peers using peer exchange (identify protocol)
func (n *Node) discoverViaPeerExchange(ctx context.Context) int {
	connected := 0
	maxConnections := 3 // Conservative limit to avoid overwhelming proxy

	// Get all peers from peerstore (includes peers discovered through identify protocol)
	allKnownPeers := n.host.Peerstore().Peers()

	for _, knownPeer := range allKnownPeers {
		if knownPeer == n.host.ID() {
			continue
		}

		// Skip if already connected
		if n.host.Network().Connectedness(knownPeer) == network.Connected {
			continue
		}

		// Get addresses for this peer
		addrs := n.host.Peerstore().Addrs(knownPeer)
		if len(addrs) == 0 {
			continue
		}

		// Filter to only standard P2P ports (avoid ephemeral client ports)
		var validAddrs []multiaddr.Multiaddr
		for _, addr := range addrs {
			addrStr := addr.String()
			// Keep addresses with standard P2P ports (4000-4999 range)
			if strings.Contains(addrStr, ":400") {
				validAddrs = append(validAddrs, addr)
			}
		}

		if len(validAddrs) == 0 {
			continue
		}

		// Try to connect with shorter timeout (proxy connections are slower)
		connectCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		peerInfo := peer.AddrInfo{ID: knownPeer, Addrs: validAddrs}

		if err := n.host.Connect(connectCtx, peerInfo); err != nil {
			cancel()
			n.logger.ComponentDebug(logging.ComponentNode, "Failed to connect to peer via exchange",
				zap.String("peer", knownPeer.String()),
				zap.Error(err))
			continue
		}
		cancel()

		n.logger.ComponentInfo(logging.ComponentNode, "Connected to new peer via peer exchange",
			zap.String("peer", knownPeer.String()))
		connected++

		if connected >= maxConnections {
			break
		}
	}

	return connected
}

// startConnectionMonitoring monitors connection health and logs status
func (n *Node) startConnectionMonitoring() {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if n.host == nil {
					return
				}

				connectedPeers := n.host.Network().Peers()
				if len(connectedPeers) == 0 {
					n.logger.Debug("Node has no connected peers - seeking connections",
						zap.String("node_id", n.host.ID().String()))
				}
			}
		}
	}()
}

// stopPeerDiscovery stops peer discovery
func (n *Node) stopPeerDiscovery() {
	if n.discoveryCancel != nil {
		n.discoveryCancel()
		n.discoveryCancel = nil
	}
	n.logger.ComponentInfo(logging.ComponentNode, "Peer discovery stopped")
}

// getListenAddresses returns the current listen addresses as strings
// Stop stops the node and all its services
func (n *Node) Stop() error {
	n.logger.ComponentInfo(logging.ComponentNode, "Stopping network node")

	// Stop peer discovery
	n.stopPeerDiscovery()

	// Stop storage service
	if n.storageService != nil {
		n.storageService.Close()
	}

	// Stop DHT
	// DHT removed - using simplified bootstrap + peer exchange discovery

	// Stop LibP2P host
	if n.host != nil {
		n.host.Close()
	}

	// Stop RQLite
	if n.rqliteAdapter != nil {
		n.rqliteAdapter.Close()
	}
	if n.rqliteManager != nil {
		_ = n.rqliteManager.Stop()
	}

	n.logger.ComponentInfo(logging.ComponentNode, "Network node stopped")
	return nil
}

// Starts the network node
func (n *Node) Start(ctx context.Context) error {
	n.logger.Info("Starting network node", zap.String("data_dir", n.config.Node.DataDir))

	// Create data directory
	if err := os.MkdirAll(n.config.Node.DataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// Start RQLite
	if err := n.startRQLite(ctx); err != nil {
		return fmt.Errorf("failed to start RQLite: %w", err)
	}

	// Start LibP2P host
	if err := n.startLibP2P(); err != nil {
		return fmt.Errorf("failed to start LibP2P: %w", err)
	}

	// Start storage service
	if err := n.startStorageService(); err != nil {
		return fmt.Errorf("failed to start storage service: %w", err)
	}

	// Get listen addresses for logging
	var listenAddrs []string
	for _, addr := range n.host.Addrs() {
		listenAddrs = append(listenAddrs, addr.String())
	}

	n.logger.ComponentInfo(logging.ComponentNode, "Network node started successfully",
		zap.String("peer_id", n.host.ID().String()),
		zap.Strings("listen_addrs", listenAddrs),
	)

	return nil
}
