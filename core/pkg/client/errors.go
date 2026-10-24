package client

import "errors"

// Common client errors
var (
	// ErrNotConnected indicates the client is not connected to the network
	ErrNotConnected = errors.New("client not connected")

	// ErrAuthRequired indicates authentication is required for the operation
	ErrAuthRequired = errors.New("authentication required")

	// ErrNoHost indicates no LibP2P host is available
	ErrNoHost = errors.New("no host available")

	// ErrInvalidPeer is a peer address or peer ID the caller got wrong: it is
	// not a multiaddr, names no peer or no address to dial, or is not a peer
	// ID. The wrapped text says which.
	ErrInvalidPeer = errors.New("invalid peer")
)
