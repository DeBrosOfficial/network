package client

import (
	"context"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	"go.uber.org/zap"
)

// startMDNSDiscovery enables mDNS peer discovery for local network
func (c *Client) startMDNSDiscovery() {
	mdnsService := mdns.NewMdnsService(c.host, "anchat-p2p", &discoveryNotifee{ client: c, logger: c.logger })
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
	n.logger.Info("mDNS discovered Anchat peer", zap.String("peer", pi.ID.String()[:8]+"..."), zap.Int("addrs", len(pi.Addrs)))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := n.client.host.Connect(ctx, pi); err != nil {
		n.logger.Debug("Failed to connect to mDNS discovered peer", zap.String("peer", pi.ID.String()[:8]+"..."), zap.Error(err))
	} else {
		n.logger.Info("Successfully connected to mDNS discovered peer", zap.String("peer", pi.ID.String()[:8]+"..."))
		if n.client.libp2pPS != nil { _ = n.client.libp2pPS.ListPeers("") }
	}
}
