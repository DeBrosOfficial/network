package client

import (
	"time"

	"go.uber.org/zap"
)

// startConnectionMonitoring starts minimal connection monitoring for the lightweight client.
// Unlike nodes which need extensive monitoring, clients only need basic health checks.
func (c *Client) startConnectionMonitoring() {
	go func() {
		ticker := time.NewTicker(60 * time.Second) // Less frequent than nodes (60s vs 30s)
		defer ticker.Stop()

		var lastPeerCount int
		firstCheck := true

		for range ticker.C {
			if !c.isConnected() {
				c.logger.Debug("Connection monitoring stopped: client disconnected")
				return
			}

			if c.host == nil {
				return
			}

			// Get current peer count
			peers := c.host.Network().Peers()
			currentPeerCount := len(peers)

			// Only log if peer count changed or on first check
			if firstCheck || currentPeerCount != lastPeerCount {
				if currentPeerCount == 0 {
					c.logger.Warn("Client has no connected peers",
						zap.String("client_id", c.host.ID().String()))
				} else if currentPeerCount < lastPeerCount {
					c.logger.Info("Client lost peers",
						zap.Int("current_peers", currentPeerCount),
						zap.Int("previous_peers", lastPeerCount))
				} else if currentPeerCount > lastPeerCount && !firstCheck {
					c.logger.Debug("Client gained peers",
						zap.Int("current_peers", currentPeerCount),
						zap.Int("previous_peers", lastPeerCount))
				}

				lastPeerCount = currentPeerCount
				firstCheck = false
			}

			// Log detailed peer info at debug level occasionally (every 5 minutes)
			if time.Now().Unix()%300 == 0 && currentPeerCount > 0 {
				peerIDs := make([]string, 0, currentPeerCount)
				for _, p := range peers {
					peerIDs = append(peerIDs, p.String())
				}
				c.logger.Debug("Client peer status",
					zap.Int("peer_count", currentPeerCount),
					zap.Strings("peer_ids", peerIDs))
			}
		}
	}()

	c.logger.Debug("Lightweight connection monitoring started")
}
