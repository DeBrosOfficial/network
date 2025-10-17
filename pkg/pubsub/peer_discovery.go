package pubsub

import (
	"context"
	"encoding/json"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"go.uber.org/zap"
)

// PeerAnnouncement represents a peer announcing its addresses
type PeerAnnouncement struct {
	PeerID    string   `json:"peer_id"`
	Addresses []string `json:"addresses"`
	Timestamp int64    `json:"timestamp"`
}

// PeerDiscoveryService handles active peer discovery via pubsub
type PeerDiscoveryService struct {
	host      host.Host
	manager   *Manager
	logger    *zap.Logger
	ctx       context.Context
	cancel    context.CancelFunc
	topicName string
}

// NewPeerDiscoveryService creates a new peer discovery service
func NewPeerDiscoveryService(host host.Host, manager *Manager, logger *zap.Logger) *PeerDiscoveryService {
	ctx, cancel := context.WithCancel(context.Background())
	return &PeerDiscoveryService{
		host:      host,
		manager:   manager,
		logger:    logger,
		ctx:       ctx,
		cancel:    cancel,
		topicName: "/debros/peer-discovery/v1",
	}
}

// Start begins the peer discovery service
func (pds *PeerDiscoveryService) Start() error {
	// Subscribe to peer discovery topic
	if err := pds.manager.Subscribe(pds.ctx, pds.topicName, pds.handlePeerAnnouncement); err != nil {
		return err
	}

	// Announce our own presence periodically
	go pds.announcePeriodically()

	return nil
}

// Stop stops the peer discovery service
func (pds *PeerDiscoveryService) Stop() error {
	pds.cancel()
	return pds.manager.Unsubscribe(pds.ctx, pds.topicName)
}

// announcePeriodically announces this peer's addresses to the network
func (pds *PeerDiscoveryService) announcePeriodically() {
	// Initial announcement after a short delay to ensure subscriptions are ready
	time.Sleep(2 * time.Second)
	pds.announceOurselves()

	// Then announce periodically
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-pds.ctx.Done():
			return
		case <-ticker.C:
			pds.announceOurselves()
		}
	}
}

// announceOurselves publishes our peer info to the discovery topic
func (pds *PeerDiscoveryService) announceOurselves() {
	// Get our listen addresses
	addrs := pds.host.Addrs()
	addrStrs := make([]string, 0, len(addrs))

	for _, addr := range addrs {
		// Create full multiaddr with peer ID
		fullAddr := addr.Encapsulate(multiaddr.StringCast("/p2p/" + pds.host.ID().String()))
		addrStrs = append(addrStrs, fullAddr.String())
	}

	announcement := PeerAnnouncement{
		PeerID:    pds.host.ID().String(),
		Addresses: addrStrs,
		Timestamp: time.Now().Unix(),
	}

	data, err := json.Marshal(announcement)
	if err != nil {
		pds.logger.Debug("Failed to marshal peer announcement", zap.Error(err))
		return
	}

	if err := pds.manager.Publish(pds.ctx, pds.topicName, data); err != nil {
		pds.logger.Debug("Failed to publish peer announcement", zap.Error(err))
	} else {
		pds.logger.Debug("Announced peer presence",
			zap.String("peer_id", pds.host.ID().String()[:16]+"..."),
			zap.Int("addresses", len(addrStrs)))
	}
}

// handlePeerAnnouncement processes incoming peer announcements
func (pds *PeerDiscoveryService) handlePeerAnnouncement(topic string, data []byte) error {
	var announcement PeerAnnouncement
	if err := json.Unmarshal(data, &announcement); err != nil {
		pds.logger.Debug("Failed to unmarshal peer announcement", zap.Error(err))
		return nil // Don't return error, just skip invalid messages
	}

	// Skip our own announcements
	if announcement.PeerID == pds.host.ID().String() {
		return nil
	}

	// Validate the announcement is recent (within last 5 minutes)
	if time.Now().Unix()-announcement.Timestamp > 300 {
		return nil // Ignore stale announcements
	}

	// Parse peer ID
	peerID, err := peer.Decode(announcement.PeerID)
	if err != nil {
		pds.logger.Debug("Invalid peer ID in announcement", zap.Error(err))
		return nil
	}

	// Skip if it's our own ID (redundant check)
	if peerID == pds.host.ID() {
		return nil
	}

	// Parse and add addresses to peerstore
	var validAddrs []multiaddr.Multiaddr
	for _, addrStr := range announcement.Addresses {
		addr, err := multiaddr.NewMultiaddr(addrStr)
		if err != nil {
			continue
		}
		validAddrs = append(validAddrs, addr)
	}

	if len(validAddrs) == 0 {
		return nil
	}

	// Add peer info to peerstore with a reasonable TTL
	pds.host.Peerstore().AddAddrs(peerID, validAddrs, time.Hour*24)

	pds.logger.Debug("Discovered peer via announcement",
		zap.String("peer_id", peerID.String()[:16]+"..."),
		zap.Int("addresses", len(validAddrs)))

	// Try to connect to the peer if we're not already connected
	if pds.host.Network().Connectedness(peerID) != 1 { // 1 = Connected
		go pds.tryConnectToPeer(peerID, validAddrs)
	}

	return nil
}

// tryConnectToPeer attempts to connect to a discovered peer
func (pds *PeerDiscoveryService) tryConnectToPeer(peerID peer.ID, addrs []multiaddr.Multiaddr) {
	ctx, cancel := context.WithTimeout(pds.ctx, 15*time.Second)
	defer cancel()

	peerInfo := peer.AddrInfo{
		ID:    peerID,
		Addrs: addrs,
	}

	if err := pds.host.Connect(ctx, peerInfo); err != nil {
		pds.logger.Debug("Failed to connect to discovered peer",
			zap.String("peer_id", peerID.String()[:16]+"..."),
			zap.Error(err))
		return
	}

	pds.logger.Info("Successfully connected to discovered peer",
		zap.String("peer_id", peerID.String()[:16]+"..."))
}
