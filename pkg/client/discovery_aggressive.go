package client

import (
	"context"
	"time"

	"go.uber.org/zap"
)

// startAggressivePeerDiscovery implements aggressive peer discovery for non-Anchat apps
func (c *Client) startAggressivePeerDiscovery() {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for i := 0; i < 20; i++ { // ~1 minute
		<-ticker.C
		if !c.isConnected() { return }

		connectedPeers := c.host.Network().Peers()
		if c.dht != nil {
			routingPeers := c.dht.RoutingTable().ListPeers()
			for _, pid := range routingPeers {
				if pid == c.host.ID() { continue }
				already := false
				for _, cp := range connectedPeers { if cp == pid { already = true; break } }
				if !already {
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					pi := c.host.Peerstore().PeerInfo(pid)
					if len(pi.Addrs) > 0 {
						if err := c.host.Connect(ctx, pi); err == nil {
							c.logger.Debug("Connected to discovered peer", zap.String("peer", pid.String()[:8]+"..."))
						}
					}
					cancel()
				}
			}
		}
		if i%10 == 0 {
			c.logger.Debug("Peer discovery status", zap.Int("iteration", i+1), zap.Int("connected_peers", len(connectedPeers)))
		}
	}
}
