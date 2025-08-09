package storage

import "go.uber.org/zap"

// newStorageLogger creates a zap.Logger for storage components.
// Callers can pass quiet=true to reduce log verbosity.
func newStorageLogger(quiet bool) (*zap.Logger, error) {
	if quiet {
		cfg := zap.NewProductionConfig()
		cfg.Level = zap.NewAtomicLevelAt(zap.WarnLevel)
		cfg.DisableCaller = true
		cfg.DisableStacktrace = true
		return cfg.Build()
	}
	return zap.NewDevelopment()
}
