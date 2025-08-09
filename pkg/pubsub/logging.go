package pubsub

import "go.uber.org/zap"

// newPubSubLogger creates a zap.Logger for pubsub components.
// Quiet mode can be handled by callers by using production config externally;
// here we default to development logger for richer diagnostics during dev.
func newPubSubLogger(quiet bool) (*zap.Logger, error) {
	if quiet {
		cfg := zap.NewProductionConfig()
		cfg.Level = zap.NewAtomicLevelAt(zap.WarnLevel)
		cfg.DisableCaller = true
		cfg.DisableStacktrace = true
		return cfg.Build()
	}
	return zap.NewDevelopment()
}
