package client

import (
	"context"

	"git.debros.io/DeBros/network/pkg/pubsub"
)

// pubSubBridge bridges between our PubSubClient interface and the pubsub package
type pubSubBridge struct {
	adapter *pubsub.ClientAdapter
}

func (p *pubSubBridge) Subscribe(ctx context.Context, topic string, handler MessageHandler) error {
	// Convert our MessageHandler to the pubsub package MessageHandler
	pubsubHandler := func(topic string, data []byte) error {
		return handler(topic, data)
	}
	return p.adapter.Subscribe(ctx, topic, pubsubHandler)
}

func (p *pubSubBridge) Publish(ctx context.Context, topic string, data []byte) error {
	return p.adapter.Publish(ctx, topic, data)
}

func (p *pubSubBridge) Unsubscribe(ctx context.Context, topic string) error {
	return p.adapter.Unsubscribe(ctx, topic)
}

func (p *pubSubBridge) ListTopics(ctx context.Context) ([]string, error) {
	return p.adapter.ListTopics(ctx)
}
