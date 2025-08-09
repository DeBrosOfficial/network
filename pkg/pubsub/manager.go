package pubsub

import (
    "sync"

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
    cancel func()
}

// NewManager creates a new pubsub manager
func NewManager(ps *pubsub.PubSub, namespace string) *Manager {
    return &Manager {
        pubsub:        ps,
        topics:        make(map[string]*pubsub.Topic),
        subscriptions: make(map[string]*subscription),
        namespace:     namespace,
    }
}
