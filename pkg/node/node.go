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
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	noise "github.com/libp2p/go-libp2p/p2p/security/noise"
	libp2pquic "github.com/libp2p/go-libp2p/p2p/transport/quic"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	"github.com/multiformats/go-multiaddr"
	"go.uber.org/zap"

	"git.debros.io/DeBros/network/pkg/config"
	"git.debros.io/DeBros/network/pkg/database"
	"git.debros.io/DeBros/network/pkg/logging"
	"git.debros.io/DeBros/network/pkg/storage"
)

// Node represents a network node with RQLite database
type Node struct {
	config         *config.Config
	logger         *logging.ColoredLogger
	host           host.Host
	dht            *dht.IpfsDHT
	rqliteManager  *database.RQLiteManager
	rqliteAdapter  *database.RQLiteAdapter
	storageService *storage.Service

	// Peer discovery
	discoveryCancel context.CancelFunc
}

// NewNode creates a new network node
func NewNode(cfg *config.Config) (*Node, error) {
	// Create colored logger
	logger, err := logging.NewDefaultLogger(logging.ComponentNode)
	if err != nil {
		return nil, fmt.Errorf("failed to create logger: %w", err)
	}

	return &Node{
		config: cfg,
		logger: logger,
	}, nil
}

// Start starts the network node
func (n *Node) Start(ctx context.Context) error {
	n.logger.ComponentInfo(logging.ComponentNode, "Starting network node",
		zap.String("data_dir", n.config.Node.DataDir),
		zap.String("type", "bootstrap"),
	)

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

// startRQLite initializes and starts the RQLite database
func (n *Node) startRQLite(ctx context.Context) error {
	n.logger.ComponentInfo(logging.ComponentDatabase, "Starting RQLite database")

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

	// Create LibP2P host with persistent identity
	h, err := libp2p.New(
		libp2p.Identity(identity),
		libp2p.ListenAddrs(listenAddrs...),
		libp2p.Security(noise.ID, noise.New),
		libp2p.Transport(tcp.NewTCPTransport),
		libp2p.Transport(libp2pquic.NewTransport),
		libp2p.DefaultMuxers,
	)
	if err != nil {
		return err
	}

	n.host = h

	// Create DHT for peer discovery - Use server mode for better peer discovery
	kademliaDHT, err := dht.New(context.Background(), h, dht.Mode(dht.ModeServer))
	if err != nil {
		return fmt.Errorf("failed to create DHT: %w", err)
	}
	n.dht = kademliaDHT

	// Connect to LibP2P bootstrap peers if configured
	if err := n.connectToBootstrapPeers(); err != nil {
		n.logger.Warn("Failed to connect to bootstrap peers", zap.Error(err))
		// Don't fail - continue without bootstrap connections
	}

	// Add bootstrap peers to DHT routing table BEFORE bootstrapping
	if len(n.config.Discovery.BootstrapPeers) > 0 {
		n.logger.Info("Adding bootstrap peers to DHT routing table")
		for _, bootstrapAddr := range n.config.Discovery.BootstrapPeers {
			if ma, err := multiaddr.NewMultiaddr(bootstrapAddr); err == nil {
				if peerInfo, err := peer.AddrInfoFromP2pAddr(ma); err == nil {
					// Add to peerstore with longer TTL
					n.host.Peerstore().AddAddrs(peerInfo.ID, peerInfo.Addrs, time.Hour*24)

					// Force add to DHT routing table
					added, err := n.dht.RoutingTable().TryAddPeer(peerInfo.ID, true, true)
					if err != nil {
						n.logger.Debug("Failed to add bootstrap peer to DHT routing table",
							zap.String("peer", peerInfo.ID.String()),
							zap.Error(err))
					} else if added {
						n.logger.Info("Successfully added bootstrap peer to DHT routing table",
							zap.String("peer", peerInfo.ID.String()))
					}
				}
			}
		}
	}

	// Bootstrap the DHT AFTER connecting to bootstrap peers and adding them to routing table
	if err = kademliaDHT.Bootstrap(context.Background()); err != nil {
		n.logger.Warn("Failed to bootstrap DHT", zap.Error(err))
		// Don't fail - continue without DHT
	} else {
		n.logger.ComponentInfo(logging.ComponentDHT, "DHT bootstrap initiated successfully")
	}

	// Give DHT a moment to initialize, then add connected peers to routing table
	go func() {
		time.Sleep(2 * time.Second)
		connectedPeers := n.host.Network().Peers()
		for _, peerID := range connectedPeers {
			if peerID != n.host.ID() {
				addrs := n.host.Peerstore().Addrs(peerID)
				if len(addrs) > 0 {
					n.host.Peerstore().AddAddrs(peerID, addrs, time.Hour*24)
					n.logger.Info("Added connected peer to DHT peerstore",
						zap.String("peer", peerID.String()))

					// Try to add this peer to DHT routing table explicitly
					if n.dht != nil {
						added, err := n.dht.RoutingTable().TryAddPeer(peerID, true, true)
						if err != nil {
							n.logger.Debug("Failed to add peer to DHT routing table",
								zap.String("peer", peerID.String()),
								zap.Error(err))
						} else if added {
							n.logger.Info("Successfully added peer to DHT routing table",
								zap.String("peer", peerID.String()))
						} else {
							n.logger.Debug("Peer already in DHT routing table or rejected",
								zap.String("peer", peerID.String()))
						}
					}
				}
			}
		}

		// Force multiple DHT refresh attempts to populate routing table
		if n.dht != nil {
			n.logger.Info("Forcing DHT refresh to discover peers")
			for i := 0; i < 3; i++ {
				time.Sleep(1 * time.Second)
				n.dht.RefreshRoutingTable()

				// Check if routing table is populated
				routingPeers := n.dht.RoutingTable().ListPeers()
				n.logger.Info("DHT routing table status after refresh",
					zap.Int("attempt", i+1),
					zap.Int("peers_in_table", len(routingPeers)))

				if len(routingPeers) > 0 {
					break // Success!
				}
			}
		}
	}()

	// Start peer discovery and monitoring
	n.startPeerDiscovery()
	n.startConnectionMonitoring()

	n.logger.ComponentInfo(logging.ComponentLibP2P, "LibP2P host started with DHT enabled",
		zap.String("peer_id", h.ID().String()))

	return nil
}

// connectToBootstrapPeers connects to configured LibP2P bootstrap peers
func (n *Node) connectToBootstrapPeers() error {
	if len(n.config.Discovery.BootstrapPeers) == 0 {
		n.logger.ComponentDebug(logging.ComponentDHT, "No bootstrap peers configured")
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for _, bootstrapAddr := range n.config.Discovery.BootstrapPeers {
		if err := n.connectToBootstrapPeer(ctx, bootstrapAddr); err != nil {
			n.logger.Warn("Failed to connect to bootstrap peer",
				zap.String("addr", bootstrapAddr),
				zap.Error(err))
			continue
		}
	}

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

	// Connect to the peer
	if err := n.host.Connect(ctx, *peerInfo); err != nil {
		return fmt.Errorf("failed to connect to peer: %w", err)
	}

	n.logger.Info("Connected to bootstrap peer",
		zap.String("peer", peerInfo.ID.String()),
		zap.String("addr", addr))

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
	if n.dht != nil {
		n.dht.Close()
	}

	// Stop LibP2P host
	if n.host != nil {
		n.host.Close()
	}

	// Stop RQLite
	if n.rqliteAdapter != nil {
		n.rqliteAdapter.Close()
	}

	n.logger.ComponentInfo(logging.ComponentNode, "Network node stopped")
	return nil
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

	// Start discovery in a goroutine
	go func() {
		// Do initial discovery immediately (no delay for faster discovery)
		n.discoverPeers(ctx)

		// Start with frequent discovery for the first minute
		rapidTicker := time.NewTicker(10 * time.Second)
		rapidAttempts := 0
		maxRapidAttempts := 6 // 6 attempts * 10 seconds = 1 minute

		for {
			select {
			case <-ctx.Done():
				rapidTicker.Stop()
				return
			case <-rapidTicker.C:
				n.discoverPeers(ctx)
				rapidAttempts++

				// After rapid attempts, switch to slower periodic discovery
				if rapidAttempts >= maxRapidAttempts {
					rapidTicker.Stop()

					// Continue with slower periodic discovery every 15 seconds
					slowTicker := time.NewTicker(15 * time.Second)
					defer slowTicker.Stop()

					for {
						select {
						case <-ctx.Done():
							return
						case <-slowTicker.C:
							n.discoverPeers(ctx)
						}
					}
				}
			}
		}
	}()
}

// discoverPeers discovers and connects to new peers
func (n *Node) discoverPeers(ctx context.Context) {
	if n.host == nil || n.dht == nil {
		return
	}

	connectedPeers := n.host.Network().Peers()
	initialCount := len(connectedPeers)

	n.logger.Debug("Node peer discovery",
		zap.Int("current_peers", initialCount))

	// Strategy 1: Use DHT to find new peers
	newConnections := n.discoverViaDHT(ctx)

	// Strategy 2: Search for random peers using DHT FindPeer

	finalPeerCount := len(n.host.Network().Peers())

	if newConnections > 0 || finalPeerCount != initialCount {
		n.logger.Debug("Node peer discovery completed",
			zap.Int("new_connections", newConnections),
			zap.Int("initial_peers", initialCount),
			zap.Int("final_peers", finalPeerCount))
	}
}

// discoverViaDHT uses the DHT to find and connect to new peers
func (n *Node) discoverViaDHT(ctx context.Context) int {
	if n.dht == nil {
		return 0
	}

	connected := 0
	maxConnections := 5

	// Get peers from routing table
	routingTablePeers := n.dht.RoutingTable().ListPeers()
	n.logger.ComponentDebug(logging.ComponentDHT, "Node DHT routing table has peers", zap.Int("count", len(routingTablePeers)))

	// Strategy 1: Connect to peers in DHT routing table
	for _, peerID := range routingTablePeers {
		if peerID == n.host.ID() {
			continue
		}

		// Check if already connected
		if n.host.Network().Connectedness(peerID) == 1 {
			continue
		}

		// Get addresses for this peer
		addrs := n.host.Peerstore().Addrs(peerID)
		if len(addrs) == 0 {
			continue
		}

		// Try to connect
		connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		peerInfo := peer.AddrInfo{ID: peerID, Addrs: addrs}

		if err := n.host.Connect(connectCtx, peerInfo); err != nil {
			cancel()
			n.logger.Debug("Failed to connect to DHT peer",
				zap.String("peer", peerID.String()),
				zap.Error(err))
			continue
		}
		cancel()

		n.logger.Debug("Node connected to new peer via DHT",
			zap.String("peer", peerID.String()))
		connected++

		if connected >= maxConnections {
			break
		}
	}

	// Strategy 2: Use peer exchange - check what peers our connected peers know about
	connectedPeers := n.host.Network().Peers()
	for _, connectedPeer := range connectedPeers {
		if connectedPeer == n.host.ID() {
			continue
		}

		// Get all peers from peerstore (this includes peers that connected peers might know about)
		allKnownPeers := n.host.Peerstore().Peers()

		for _, knownPeer := range allKnownPeers {
			if knownPeer == n.host.ID() || knownPeer == connectedPeer {
				continue
			}

			// Skip if already connected
			if n.host.Network().Connectedness(knownPeer) == 1 {
				continue
			}

			// Get addresses for this peer
			addrs := n.host.Peerstore().Addrs(knownPeer)
			if len(addrs) == 0 {
				continue
			}

			// Filter addresses to only include listening ports (not ephemeral client ports)
			var validAddrs []multiaddr.Multiaddr
			for _, addr := range addrs {
				addrStr := addr.String()
				// Skip ephemeral ports (typically above 49152) and keep standard ports
				if !strings.Contains(addrStr, ":53") && // Skip ephemeral ports starting with 53
					!strings.Contains(addrStr, ":54") && // Skip ephemeral ports starting with 54
					!strings.Contains(addrStr, ":55") && // Skip ephemeral ports starting with 55
					!strings.Contains(addrStr, ":56") && // Skip ephemeral ports starting with 56
					!strings.Contains(addrStr, ":57") && // Skip ephemeral ports starting with 57
					!strings.Contains(addrStr, ":58") && // Skip ephemeral ports starting with 58
					!strings.Contains(addrStr, ":59") && // Skip ephemeral ports starting with 59
					!strings.Contains(addrStr, ":6") && // Skip ephemeral ports starting with 6
					(strings.Contains(addrStr, ":400") || // Include 4000-4999 range
						strings.Contains(addrStr, ":401") ||
						strings.Contains(addrStr, ":402") ||
						strings.Contains(addrStr, ":403")) {
					validAddrs = append(validAddrs, addr)
				}
			}

			if len(validAddrs) == 0 {
				continue
			}

			// Try to connect using only valid addresses
			connectCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			peerInfo := peer.AddrInfo{ID: knownPeer, Addrs: validAddrs}

			if err := n.host.Connect(connectCtx, peerInfo); err != nil {
				cancel()
				n.logger.Debug("Failed to connect to peerstore peer",
					zap.String("peer", knownPeer.String()),
					zap.Error(err))
				continue
			}
			cancel()

			n.logger.Debug("Node connected to new peer via peerstore",
				zap.String("peer", knownPeer.String()))
			connected++

			if connected >= maxConnections {
				return connected
			}
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
}
