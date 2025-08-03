package pubsub

// MessageHandler represents a message handler function signature
// This matches the client.MessageHandler type to avoid circular imports
type MessageHandler func(topic string, data []byte) error