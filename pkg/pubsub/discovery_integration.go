package pubsub

import (
	"context"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

// announceTopicInterest helps with peer discovery by announcing interest in a topic.
// It starts lightweight monitoring and performs a single proactive announcement to
// encourage peers to respond. This implementation is application-agnostic.
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

	// Start periodic monitoring for topic peers
	go m.monitorTopicPeers(topicName, topic)

	// Perform a single proactive announcement to the topic to encourage peers to respond
	go m.forceTopicPeerDiscovery(topicName, topic)
}

// forceTopicPeerDiscovery uses a simple strategy to announce presence on the topic.
// It publishes a lightweight discovery ping and returns quickly.
func (m *Manager) forceTopicPeerDiscovery(topicName string, topic *pubsub.Topic) {
	// If pubsub already reports peers for this topic, do nothing.
	peers := topic.ListPeers()
	if len(peers) > 0 {
		return
	}

	// Send a short-lived discovery ping to the topic to announce presence.
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	discoveryMsg := []byte("PEER_DISCOVERY_PING")
	_ = topic.Publish(ctx, discoveryMsg)

	// Wait briefly to allow peers to respond via pubsub peer exchange
	time.Sleep(300 * time.Millisecond)
}

// monitorTopicPeers periodically checks topic peer connectivity and stops once peers are found.
func (m *Manager) monitorTopicPeers(topicName string, topic *pubsub.Topic) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for i := 0; i < 6; i++ { // Monitor for ~30 seconds
		<-ticker.C
		peers := topic.ListPeers()

		// If we have peers, stop monitoring
		if len(peers) > 0 {
			return
		}
	}
}
