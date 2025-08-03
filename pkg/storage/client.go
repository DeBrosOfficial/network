package storage

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"go.uber.org/zap"
)

// Client provides distributed storage client functionality
type Client struct {
	host      host.Host
	logger    *zap.Logger
	namespace string
}

// NewClient creates a new storage client
func NewClient(h host.Host, namespace string, logger *zap.Logger) *Client {
	return &Client{
		host:      h,
		logger:    logger,
		namespace: namespace,
	}
}

// Put stores a key-value pair in the distributed storage
func (c *Client) Put(ctx context.Context, key string, value []byte) error {
	request := &StorageRequest{
		Type:      MessageTypePut,
		Key:       key,
		Value:     value,
		Namespace: c.namespace,
	}

	return c.sendRequest(ctx, request)
}

// Get retrieves a value by key from the distributed storage
func (c *Client) Get(ctx context.Context, key string) ([]byte, error) {
	request := &StorageRequest{
		Type:      MessageTypeGet,
		Key:       key,
		Namespace: c.namespace,
	}

	response, err := c.sendRequestWithResponse(ctx, request)
	if err != nil {
		return nil, err
	}

	if !response.Success {
		return nil, fmt.Errorf(response.Error)
	}

	return response.Value, nil
}

// Delete removes a key from the distributed storage
func (c *Client) Delete(ctx context.Context, key string) error {
	request := &StorageRequest{
		Type:      MessageTypeDelete,
		Key:       key,
		Namespace: c.namespace,
	}

	return c.sendRequest(ctx, request)
}

// List returns keys with a given prefix
func (c *Client) List(ctx context.Context, prefix string, limit int) ([]string, error) {
	request := &StorageRequest{
		Type:      MessageTypeList,
		Prefix:    prefix,
		Limit:     limit,
		Namespace: c.namespace,
	}

	response, err := c.sendRequestWithResponse(ctx, request)
	if err != nil {
		return nil, err
	}

	if !response.Success {
		return nil, fmt.Errorf(response.Error)
	}

	return response.Keys, nil
}

// Exists checks if a key exists in the distributed storage
func (c *Client) Exists(ctx context.Context, key string) (bool, error) {
	request := &StorageRequest{
		Type:      MessageTypeExists,
		Key:       key,
		Namespace: c.namespace,
	}

	response, err := c.sendRequestWithResponse(ctx, request)
	if err != nil {
		return false, err
	}

	if !response.Success {
		return false, fmt.Errorf(response.Error)
	}

	return response.Exists, nil
}

// sendRequest sends a request without expecting a response
func (c *Client) sendRequest(ctx context.Context, request *StorageRequest) error {
	_, err := c.sendRequestWithResponse(ctx, request)
	return err
}

// sendRequestWithResponse sends a request and waits for a response
func (c *Client) sendRequestWithResponse(ctx context.Context, request *StorageRequest) (*StorageResponse, error) {
	// Get connected peers
	peers := c.host.Network().Peers()
	if len(peers) == 0 {
		return nil, fmt.Errorf("no peers connected")
	}

	// Try to send to the first available peer
	// In a production system, you might want to implement peer selection logic
	for _, peerID := range peers {
		response, err := c.sendToPeer(ctx, peerID, request)
		if err != nil {
			c.logger.Debug("Failed to send to peer",
				zap.String("peer", peerID.String()),
				zap.Error(err))
			continue
		}
		return response, nil
	}

	return nil, fmt.Errorf("failed to send request to any peer")
}

// sendToPeer sends a request to a specific peer
func (c *Client) sendToPeer(ctx context.Context, peerID peer.ID, request *StorageRequest) (*StorageResponse, error) {
	// Create a new stream to the peer
	stream, err := c.host.NewStream(ctx, peerID, protocol.ID(StorageProtocolID))
	if err != nil {
		return nil, fmt.Errorf("failed to create stream: %w", err)
	}
	defer stream.Close()

	// Set deadline for the operation
	deadline, ok := ctx.Deadline()
	if ok {
		stream.SetDeadline(deadline)
	} else {
		stream.SetDeadline(time.Now().Add(30 * time.Second))
	}

	// Marshal and send request
	requestData, err := request.Marshal()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	if _, err := stream.Write(requestData); err != nil {
		return nil, fmt.Errorf("failed to write request: %w", err)
	}

	// Close write side to signal end of request
	if err := stream.CloseWrite(); err != nil {
		return nil, fmt.Errorf("failed to close write: %w", err)
	}

	// Read response
	responseData, err := io.ReadAll(stream)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Unmarshal response
	var response StorageResponse
	if err := response.Unmarshal(responseData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &response, nil
}
