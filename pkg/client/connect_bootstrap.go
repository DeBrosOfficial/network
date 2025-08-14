package client

import (
	"context"
	"fmt"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"go.uber.org/zap"
)

// connectToBootstrap connects to a bootstrap peer
func (c *Client) connectToBootstrap(ctx context.Context, addr string) error {
	ma, err := multiaddr.NewMultiaddr(addr)
	if err != nil {
		return fmt.Errorf("invalid multiaddr: %w", err)
	}

	// Try to extract peer info if it's a full multiaddr with peer ID
	peerInfo, err := peer.AddrInfoFromP2pAddr(ma)
	if err != nil {
		// If there's no peer ID, try to discover the peer at this address
		return c.connectToAddress(ctx, ma)
	}

	if err := c.host.Connect(ctx, *peerInfo); err != nil {
		return fmt.Errorf("failed to connect to peer: %w", err)
	}

	c.logger.Debug("Connected to bootstrap peer",
		zap.String("peer", peerInfo.ID.String()),
		zap.String("addr", addr))

	return nil
}

// connectToAddress attempts to discover and connect to a peer at the given address
func (c *Client) connectToAddress(ctx context.Context, ma multiaddr.Multiaddr) error {
	// For the simple case, we'll just warn and continue
	// In a production environment, you'd implement proper peer discovery

	c.logger.Warn("No peer ID provided in address, skipping bootstrap connection",
		zap.String("addr", ma.String()),
		zap.String("suggestion", "Use full multiaddr with peer ID like: /ip4/127.0.0.1/tcp/4001/p2p/<peer-id>"))

	return nil // Don't fail - let the client continue without bootstrap
}
