package client

import (
	"fmt"
	"time"

	"github.com/DeBrosOfficial/network/pkg/config"
)

// DefaultClientConfig returns a default client configuration
func DefaultClientConfig(appName string) *ClientConfig {
	defaultCfg := config.DefaultConfig()

	return &ClientConfig{
		AppName:           appName,
		DatabaseName:      fmt.Sprintf("%s_db", appName),
		BootstrapPeers:    defaultCfg.Discovery.BootstrapPeers,
		DatabaseEndpoints: []string{},
		ConnectTimeout:    30 * time.Second,
		RetryAttempts:     3,
		RetryDelay:        5 * time.Second,
		QuietMode:         false,
		APIKey:            "",
		JWT:               "",
	}
}

// ValidateClientConfig validates a client configuration
func ValidateClientConfig(cfg *ClientConfig) error {
	if len(cfg.BootstrapPeers) == 0 {
		return fmt.Errorf("at least one bootstrap peer is required")
	}

	if cfg.AppName == "" {
		return fmt.Errorf("app name is required")
	}

	return nil
}
