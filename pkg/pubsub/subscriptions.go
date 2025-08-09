package pubsub

import (
	"context"
	"fmt"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

// Subscribe subscribes to a topic
func (m *Manager) Subscribe(ctx context.Context, topic string, handler MessageHandler) error {
	if m.pubsub == nil {
		return fmt.Errorf("pubsub not initialized")
	}

	namespacedTopic := fmt.Sprintf("%s.%s", m.namespace, topic)

	// Check if already subscribed
	m.mu.Lock()
	if _, exists := m.subscriptions[namespacedTopic]; exists {
		m.mu.Unlock()
		// Already subscribed - this is normal for LibP2P pubsub
		return nil
	}
	m.mu.Unlock()

	// Get or create topic
	libp2pTopic, err := m.getOrCreateTopic(namespacedTopic)
	if err != nil {
		return fmt.Errorf("failed to get topic: %w", err)
	}

	// Subscribe to topic
	sub, err := libp2pTopic.Subscribe()
	if err != nil {
		return fmt.Errorf("failed to subscribe to topic: %w", err)
	}

	// Create cancellable context for this subscription
	subCtx, cancel := context.WithCancel(context.Background())

	// Store subscription
	m.mu.Lock()
	m.subscriptions[namespacedTopic] = &subscription{
		sub:    sub,
		cancel: cancel,
	}
	m.mu.Unlock()

	// Start message handler goroutine
	go func() {
		defer func() {
			sub.Cancel()
		}()

		for {
			select {
			case <-subCtx.Done():
				return
			default:
				msg, err := sub.Next(subCtx)
				if err != nil {
					if subCtx.Err() != nil {
						return // Context cancelled
					}
					continue
				}

				// Call the handler
				if err := handler(topic, msg.Data); err != nil {
					// Log error but continue processing
					continue
				}
			}
		}
	}()

	// Force peer discovery for this topic
	go m.announceTopicInterest(namespacedTopic)

	// For Anchat, also try to actively find topic peers through the libp2p pubsub system
	if len(m.namespace) > 6 && m.namespace[:6] == "anchat" {
		go m.enhancedAnchatTopicDiscovery(namespacedTopic, libp2pTopic)
	}

	return nil
}

// Unsubscribe unsubscribes from a topic
func (m *Manager) Unsubscribe(ctx context.Context, topic string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	namespacedTopic := fmt.Sprintf("%s.%s", m.namespace, topic)

	if subscription, exists := m.subscriptions[namespacedTopic]; exists {
		// Cancel the subscription context to stop the message handler goroutine
		subscription.cancel()
		delete(m.subscriptions, namespacedTopic)
	}

	return nil
}

// ListTopics returns all subscribed topics
func (m *Manager) ListTopics(ctx context.Context) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var topics []string
	prefix := m.namespace + "."

	for topic := range m.subscriptions {
		if len(topic) > len(prefix) && topic[:len(prefix)] == prefix {
			originalTopic := topic[len(prefix):]
			topics = append(topics, originalTopic)
		}
	}

	return topics, nil
}

// Close closes all subscriptions and topics
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Cancel all subscriptions
	for _, sub := range m.subscriptions {
		sub.cancel()
	}
	m.subscriptions = make(map[string]*subscription)

	// Close all topics
	for _, topic := range m.topics {
		topic.Close()
	}
	m.topics = make(map[string]*pubsub.Topic)

	return nil
}
