package client

import (
	"context"

	"git.debros.io/DeBros/network/pkg/pubsub"
	"git.debros.io/DeBros/network/pkg/storage"
)

// WithNamespace applies both storage and pubsub namespace overrides to the context.
// It is a convenience helper for client callers to ensure both subsystems receive
// the same, consistent namespace override.
func WithNamespace(ctx context.Context, ns string) context.Context {
	ctx = storage.WithNamespace(ctx, ns)
	ctx = pubsub.WithNamespace(ctx, ns)
	return ctx
}
