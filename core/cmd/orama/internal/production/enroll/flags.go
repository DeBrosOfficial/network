package enroll

import (
	"fmt"
)

// Flags holds the parsed command-line flags for the enroll command.
type Flags struct {
	NodeIP     string // Public IP of the OramaOS node
	Code       string // Registration code from the node's console
	Token      string // Invite token for cluster joining
	GatewayURL string // Gateway HTTPS URL
	Env        string // Environment name (for display only)
}

// validate checks the required flags.
func (f *Flags) validate() error {
	if f.NodeIP == "" {
		return fmt.Errorf("--node-ip is required")
	}
	if f.Code == "" {
		return fmt.Errorf("--code is required: read it from the node's console")
	}
	if f.Token == "" {
		return fmt.Errorf("--token is required")
	}
	if f.GatewayURL == "" {
		return fmt.Errorf("--gateway is required")
	}
	return nil
}
