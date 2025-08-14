package discovery

import (
	"context"
	"errors"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"go.uber.org/zap"
)

// Manager handles peer discovery operations without a DHT dependency.
// Note: The constructor intentionally accepts a second parameter of type
// interface{} to remain source-compatible with previous call sites that
// passed a DHT instance. The value is ignored.
type Manager struct {
	host   host.Host
	logger *zap.Logger
	cancel context.CancelFunc
}

// Config contains discovery configuration
type Config struct {
	DiscoveryInterval time.Duration
	MaxConnections    int
}

// NewManager creates a new discovery manager.
//
// The second parameter is intentionally typed as interface{} so callers that
// previously passed a DHT instance can continue to do so; the value is ignored.
func NewManager(h host.Host, _ interface{}, logger *zap.Logger) *Manager {
	return &Manager{
		host:   h,
		logger: logger,
	}
}

// NewManagerSimple creates a manager with a cleaner signature (host + logger).
func NewManagerSimple(h host.Host, logger *zap.Logger) *Manager {
	return NewManager(h, nil, logger)
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

// discoverPeers discovers and connects to new peers using non-DHT strategies:
//   - Peerstore entries (bootstrap peers added to peerstore by the caller)
//   - Peer exchange: query currently connected peers' peerstore entries
func (d *Manager) discoverPeers(ctx context.Context, config Config) {
	connectedPeers := d.host.Network().Peers()
	initialCount := len(connectedPeers)

	d.logger.Debug("Starting peer discovery",
		zap.Int("current_peers", initialCount))

	newConnections := 0

	// Strategy 1: Try to connect to peers learned from the host's peerstore
	newConnections += d.discoverViaPeerstore(ctx, config.MaxConnections-newConnections)

	// Strategy 2: Ask connected peers about their connections (peer exchange)
	if newConnections < config.MaxConnections {
		newConnections += d.discoverViaPeerExchange(ctx, config.MaxConnections-newConnections)
	}

	finalPeerCount := len(d.host.Network().Peers())

	if newConnections > 0 || finalPeerCount != initialCount {
		d.logger.Debug("Peer discovery completed",
			zap.Int("new_connections", newConnections),
			zap.Int("initial_peers", initialCount),
			zap.Int("final_peers", finalPeerCount))
	}
}

// discoverViaPeerstore attempts to connect to peers found in the host's peerstore.
// This is useful for bootstrap peers that have been pre-populated into the peerstore.
func (d *Manager) discoverViaPeerstore(ctx context.Context, maxConnections int) int {
	if maxConnections <= 0 {
		return 0
	}

	connected := 0

	// Iterate over peerstore known peers
	peers := d.host.Peerstore().Peers()
	d.logger.Debug("Peerstore contains peers", zap.Int("count", len(peers)))

	for _, pid := range peers {
		if connected >= maxConnections {
			break
		}
		// Skip self
		if pid == d.host.ID() {
			continue
		}
		// Skip already connected peers
		if d.host.Network().Connectedness(pid) != network.NotConnected {
			continue
		}

		// Try to connect
		if err := d.connectToPeer(ctx, pid); err == nil {
			connected++
		}
	}

	return connected
}

// discoverViaPeerExchange asks currently connected peers for addresses of other peers
// by inspecting their peerstore entries. This is a lightweight peer-exchange approach.
func (d *Manager) discoverViaPeerExchange(ctx context.Context, maxConnections int) int {
	if maxConnections <= 0 {
		return 0
	}

	connected := 0
	connectedPeers := d.host.Network().Peers()

	for _, peerID := range connectedPeers {
		if connected >= maxConnections {
			break
		}

		peerInfo := d.host.Peerstore().PeerInfo(peerID)
		for _, addr := range peerInfo.Addrs {
			if connected >= maxConnections {
				break
			}
			// Attempt to extract peer ID from addr is not done here; we rely on peerstore entries.
			// If an address belongs to a known peer (already in peerstore), connect via that peer id.
			// No-op placeholder: actual exchange protocols would be required for richer discovery.
			_ = addr
		}
	}

	// The above is intentionally conservative (no active probing) because without an application-level
	// peer-exchange protocol we cannot reliably learn new peer IDs from peers' addresses.
	// Most useful discovery will come from bootstrap peers added to the peerstore by the caller.

	return connected
}

// connectToPeer attempts to connect to a specific peer using its peerstore info.
func (d *Manager) connectToPeer(ctx context.Context, peerID peer.ID) error {
	peerInfo := d.host.Peerstore().PeerInfo(peerID)
	if len(peerInfo.Addrs) == 0 {
		return errors.New("no addresses for peer")
	}

	// Attempt connection
	if err := d.host.Connect(ctx, peerInfo); err != nil {
		d.logger.Debug("Failed to connect to peer",
			zap.String("peer_id", peerID.String()[:8]+"..."),
			zap.Error(err))
		return err
	}

	d.logger.Debug("Successfully connected to peer",
		zap.String("peer_id", peerID.String()[:8]+"..."))

	return nil
}
