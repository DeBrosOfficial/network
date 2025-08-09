package pubsub

import (
	"context"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

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
