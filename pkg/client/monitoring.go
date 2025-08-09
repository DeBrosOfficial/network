package client

import "time"

// startConnectionMonitoring monitors connection health and logs status
func (c *Client) startConnectionMonitoring() {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			if !c.isConnected() {
				return
			}

			// Only touch network to detect issues; avoid noisy logs
			_ = c.host.Network().Peers()
		}
	}()
}
