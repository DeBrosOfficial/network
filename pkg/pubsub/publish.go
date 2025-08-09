package pubsub

import (
	"context"
	"fmt"
)

// Publish publishes a message to a topic
func (m *Manager) Publish(ctx context.Context, topic string, data []byte) error {
	if m.pubsub == nil {
		return fmt.Errorf("pubsub not initialized")
	}

	namespacedTopic := fmt.Sprintf("%s.%s", m.namespace, topic)

	// Get or create topic
	libp2pTopic, err := m.getOrCreateTopic(namespacedTopic)
	if err != nil {
		return fmt.Errorf("failed to get topic for publishing: %w", err)
	}

	// Publish message
	if err := libp2pTopic.Publish(ctx, data); err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}

	return nil
}
