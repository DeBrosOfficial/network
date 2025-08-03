package discovery

import (
	"context"
	"errors"
	"time"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"go.uber.org/zap"
)

// Manager handles peer discovery operations
type Manager struct {
	host   host.Host
	dht    *dht.IpfsDHT
	logger *zap.Logger
	cancel context.CancelFunc
}

// Config contains discovery configuration
type Config struct {
	DiscoveryInterval time.Duration
	MaxConnections    int
}

// NewManager creates a new discovery manager
func NewManager(host host.Host, dht *dht.IpfsDHT, logger *zap.Logger) *Manager {
	return &Manager{
		host:   host,
		dht:    dht,
		logger: logger,
	}
}

// Start begins periodic peer discovery
func (d *Manager) Start(config Config) error {
	ctx, cancel := context.WithCancel(context.Background())
	d.cancel = cancel

	go func() {
		// Do initial discovery immediately
		d.discoverPeers(ctx, config)

		// Continue with periodic discovery
		ticker := time.NewTicker(config.DiscoveryInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				d.discoverPeers(ctx, config)
			}
		}
	}()

	return nil
}

// Stop stops peer discovery
func (d *Manager) Stop() {
	if d.cancel != nil {
		d.cancel()
	}
}

// discoverPeers discovers and connects to new peers
func (d *Manager) discoverPeers(ctx context.Context, config Config) {
	connectedPeers := d.host.Network().Peers()
	initialCount := len(connectedPeers)

	d.logger.Debug("Starting peer discovery",
		zap.Int("current_peers", initialCount))

	// Strategy 1: Use DHT to find peers
	newConnections := d.discoverViaDHT(ctx, config.MaxConnections)

	// Strategy 2: Ask connected peers about their connections
	newConnections += d.discoverViaPeerExchange(ctx, config.MaxConnections)

	finalPeerCount := len(d.host.Network().Peers())

	if newConnections > 0 || finalPeerCount != initialCount {
		d.logger.Debug("Peer discovery completed",
			zap.Int("new_connections", newConnections),
			zap.Int("initial_peers", initialCount),
			zap.Int("final_peers", finalPeerCount))
	}
}

// discoverViaDHT uses the DHT to find random peers
func (d *Manager) discoverViaDHT(ctx context.Context, maxConnections int) int {
	if d.dht == nil {
		return 0
	}

	connected := 0

	// Get peers from routing table
	routingTablePeers := d.dht.RoutingTable().ListPeers()
	d.logger.Debug("DHT routing table has peers", zap.Int("count", len(routingTablePeers)))

	for _, peerID := range routingTablePeers {
		if peerID == d.host.ID() {
			continue
		}

		if connected >= maxConnections {
			break
		}

		// Check if we're already connected
		if d.host.Network().Connectedness(peerID) != network.NotConnected {
			continue
		}

		// Try to connect
		if err := d.connectToPeer(ctx, peerID); err == nil {
			connected++
		}
	}

	return connected
}

// discoverViaPeerExchange asks connected peers about their connections
func (d *Manager) discoverViaPeerExchange(ctx context.Context, maxConnections int) int {
	connected := 0
	connectedPeers := d.host.Network().Peers()

	for _, peerID := range connectedPeers {
		if connected >= maxConnections {
			break
		}

		// Get peer connections (this is a simplified implementation)
		// In a real implementation, you might use a custom protocol
		peerInfo := d.host.Peerstore().PeerInfo(peerID)
		for _, addr := range peerInfo.Addrs {
			if connected >= maxConnections {
				break
			}

			// Extract peer ID from multiaddr and try to connect
			// This is simplified - in practice you'd need proper multiaddr parsing
			_ = addr // Placeholder for actual implementation
		}
	}

	return connected
}

// connectToPeer attempts to connect to a specific peer
func (d *Manager) connectToPeer(ctx context.Context, peerID peer.ID) error {
	// Get peer info from DHT
	peerInfo := d.host.Peerstore().PeerInfo(peerID)
	if len(peerInfo.Addrs) == 0 {
		return errors.New("no addresses for peer")
	}

	// Attempt connection
	if err := d.host.Connect(ctx, peerInfo); err != nil {
		d.logger.Debug("Failed to connect to DHT peer",
			zap.String("peer_id", peerID.String()[:8]+"..."),
			zap.Error(err))
		return err
	}

	d.logger.Debug("Successfully connected to DHT peer",
		zap.String("peer_id", peerID.String()[:8]+"..."))

	return nil
}