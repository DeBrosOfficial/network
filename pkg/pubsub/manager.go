package pubsub

import (
	"context"
	"fmt"
	"sync"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

// Manager handles pub/sub operations
type Manager struct {
	pubsub        *pubsub.PubSub
	topics        map[string]*pubsub.Topic
	subscriptions map[string]*subscription
	namespace     string
	mu            sync.RWMutex
}

// subscription holds subscription data
type subscription struct {
	sub    *pubsub.Subscription
	cancel context.CancelFunc
}

// NewManager creates a new pubsub manager
func NewManager(ps *pubsub.PubSub, namespace string) *Manager {
	return &Manager{
		pubsub:        ps,
		topics:        make(map[string]*pubsub.Topic),
		subscriptions: make(map[string]*subscription),
		namespace:     namespace,
	}
}

// getOrCreateTopic gets an existing topic or creates a new one
func (m *Manager) getOrCreateTopic(topicName string) (*pubsub.Topic, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Return existing topic if available
	if topic, exists := m.topics[topicName]; exists {
		return topic, nil
	}

	// Join the topic - LibP2P allows multiple clients to join the same topic
	topic, err := m.pubsub.Join(topicName)
	if err != nil {
		return nil, fmt.Errorf("failed to join topic: %w", err)
	}

	m.topics[topicName] = topic
	return topic, nil
}

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

// announceTopicInterest helps with peer discovery by announcing interest in a topic
func (m *Manager) announceTopicInterest(topicName string) {
	// Wait a bit for the subscription to be established
	time.Sleep(100 * time.Millisecond)

	// Get the topic
	m.mu.RLock()
	topic, exists := m.topics[topicName]
	m.mu.RUnlock()

	if !exists {
		return
	}

	// For Anchat specifically, be more aggressive about finding peers
	if len(m.namespace) > 6 && m.namespace[:6] == "anchat" {
		go m.aggressiveTopicPeerDiscovery(topicName, topic)
	} else {
		// Start a periodic check to monitor topic peer growth
		go m.monitorTopicPeers(topicName, topic)
	}
}

// aggressiveTopicPeerDiscovery for Anchat - actively seeks topic peers
func (m *Manager) aggressiveTopicPeerDiscovery(topicName string, topic *pubsub.Topic) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for i := 0; i < 30; i++ { // Monitor for 30 seconds
		<-ticker.C
		peers := topic.ListPeers()

		// If we have peers, reduce frequency but keep monitoring
		if len(peers) > 0 {
			// Switch to normal monitoring once we have peers
			go m.monitorTopicPeers(topicName, topic)
			return
		}

		// For Anchat, try to actively discover and connect to peers on this topic
		// This is critical because LibP2P pubsub requires direct connections for message propagation
		m.forceTopicPeerDiscovery(topicName, topic)
	}
}

// enhancedAnchatTopicDiscovery implements enhanced peer discovery specifically for Anchat
func (m *Manager) enhancedAnchatTopicDiscovery(topicName string, topic *pubsub.Topic) {
	// Wait for subscription to be fully established
	time.Sleep(200 * time.Millisecond)

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for i := 0; i < 20; i++ { // Monitor for 20 seconds
		<-ticker.C

		peers := topic.ListPeers()
		if len(peers) > 0 {
			// Success! We found topic peers
			return
		}

		// Try various discovery strategies
		if i%3 == 0 {
			// Strategy: Send discovery heartbeat
			ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			discoveryMsg := []byte("ANCHAT_DISCOVERY_PING")
			topic.Publish(ctx, discoveryMsg)
			cancel()
		}

		// Wait a bit and check again
		time.Sleep(500 * time.Millisecond)
		peers = topic.ListPeers()
		if len(peers) > 0 {
			return
		}
	}
}

// forceTopicPeerDiscovery uses multiple strategies to find and connect to topic peers
func (m *Manager) forceTopicPeerDiscovery(topicName string, topic *pubsub.Topic) {
	// Strategy 1: Check if pubsub knows about any peers for this topic
	peers := topic.ListPeers()
	if len(peers) > 0 {
		return // We already have peers
	}

	// Strategy 2: Try to actively announce our presence and wait for responses
	// Send a ping/heartbeat to the topic to announce our presence
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Create a discovery message to announce our presence on this topic
	discoveryMsg := []byte("ANCHAT_PEER_DISCOVERY")
	topic.Publish(ctx, discoveryMsg)

	// Strategy 3: Wait briefly and check again
	time.Sleep(500 * time.Millisecond)
	_ = topic.ListPeers() // Check again but we don't need to use the result

	// Note: In LibP2P, topics don't automatically form connections between subscribers
	// The underlying network layer needs to ensure peers are connected first
	// This is why our enhanced client peer discovery is crucial
}

// monitorTopicPeers periodically checks topic peer connectivity
func (m *Manager) monitorTopicPeers(topicName string, topic *pubsub.Topic) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for i := 0; i < 6; i++ { // Monitor for 30 seconds
		<-ticker.C
		peers := topic.ListPeers()

		// If we have peers, we're good
		if len(peers) > 0 {
			return
		}
	}
}
