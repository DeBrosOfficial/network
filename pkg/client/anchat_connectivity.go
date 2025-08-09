package client

import (
	"context"
	"time"

	"go.uber.org/zap"
)

// ensureAnchatPeerConnectivity ensures Anchat clients can discover each other through bootstrap
func (c *Client) ensureAnchatPeerConnectivity() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for i := 0; i < 30; i++ { // Run for ~1 minute
		<-ticker.C

		if !c.isConnected() {
			return
		}

		connectedPeers := c.host.Network().Peers()

		if c.dht != nil {
			// Try to find peers through DHT routing table
			routingPeers := c.dht.RoutingTable().ListPeers()

			for _, peerID := range routingPeers {
				if peerID == c.host.ID() {
					continue
				}

				// Check already connected
				alreadyConnected := false
				for _, p := range connectedPeers {
					if p == peerID {
						alreadyConnected = true
						break
					}
				}

				if !alreadyConnected {
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					peerInfo := c.host.Peerstore().PeerInfo(peerID)

					if len(peerInfo.Addrs) == 0 {
						if found, err := c.dht.FindPeer(ctx, peerID); err == nil {
							peerInfo = found
							c.host.Peerstore().AddAddrs(peerInfo.ID, peerInfo.Addrs, time.Hour*24)
						}
					}

					if len(peerInfo.Addrs) > 0 {
						if err := c.host.Connect(ctx, peerInfo); err == nil {
							c.logger.Info("Anchat discovered and connected to peer",
								zap.String("peer", peerID.String()[:8]+"..."))

							if added, err := c.dht.RoutingTable().TryAddPeer(peerID, true, true); err == nil && added {
								c.logger.Debug("Added new peer to DHT routing table",
									zap.String("peer", peerID.String()[:8]+"..."))
							}

							if c.libp2pPS != nil {
								time.Sleep(100 * time.Millisecond)
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

			if len(routingPeers) == 0 {
				for _, id := range connectedPeers {
					if id != c.host.ID() {
						if added, err := c.dht.RoutingTable().TryAddPeer(id, true, true); err == nil && added {
							c.logger.Info("Force-added connected peer to DHT routing table",
								zap.String("peer", id.String()[:8]+"..."))
						}
					}
				}
				c.dht.RefreshRoutingTable()
			}
		}

		// Reconnect to known peers not currently connected
		allKnownPeers := c.host.Peerstore().Peers()
		for _, id := range allKnownPeers {
			if id == c.host.ID() {
				continue
			}
			already := false
			for _, p := range connectedPeers {
				if p == id { already = true; break }
			}
			if !already {
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				pi := c.host.Peerstore().PeerInfo(id)
				if len(pi.Addrs) > 0 {
					if err := c.host.Connect(ctx, pi); err == nil {
						c.logger.Info("Anchat reconnected to known peer",
							zap.String("peer", id.String()[:8]+"..."))
						if c.libp2pPS != nil { time.Sleep(100 * time.Millisecond); _ = c.libp2pPS.ListPeers("") }
					}
				}
				cancel()
			}
		}

		if i%5 == 0 && len(connectedPeers) > 0 {
			c.logger.Info("Anchat peer discovery progress",
				zap.Int("iteration", i+1),
				zap.Int("connected_peers", len(connectedPeers)),
				zap.Int("known_peers", len(allKnownPeers)))
		}
	}
}
