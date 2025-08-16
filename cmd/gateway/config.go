package main

import (
	"flag"
	"os"
	"strings"

	"git.debros.io/DeBros/network/pkg/gateway"
	"git.debros.io/DeBros/network/pkg/logging"
	"go.uber.org/zap"
)

// For transition, alias main.GatewayConfig to pkg/gateway.Config
// server.go will be removed; this keeps compatibility until then.
type GatewayConfig = gateway.Config

func getEnvDefault(key, def string) string {
	if v := os.Getenv(key); strings.TrimSpace(v) != "" {
		return v
	}
	return def
}

func getEnvBoolDefault(key string, def bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	switch strings.ToLower(v) {
	case "1", "true", "t", "yes", "y", "on":
		return true
	case "0", "false", "f", "no", "n", "off":
		return false
	default:
		return def
	}
}

// parseGatewayConfig parses flags and environment variables into GatewayConfig.
// Priority: flags > env > defaults.
func parseGatewayConfig(logger *logging.ColoredLogger) *gateway.Config {
	addr := flag.String("addr", getEnvDefault("GATEWAY_ADDR", ":8080"), "HTTP listen address (e.g., :8080)")
	ns := flag.String("namespace", getEnvDefault("GATEWAY_NAMESPACE", "default"), "Client namespace for scoping resources")
	peers := flag.String("bootstrap-peers", getEnvDefault("GATEWAY_BOOTSTRAP_PEERS", ""), "Comma-separated bootstrap peers for network client")
	requireAuth := flag.Bool("require-auth", getEnvBoolDefault("GATEWAY_REQUIRE_AUTH", false), "Require API key authentication for requests")
	apiKeysStr := flag.String("api-keys", getEnvDefault("GATEWAY_API_KEYS", ""), "Comma-separated API keys, optionally as key:namespace")

	// Do not call flag.Parse() elsewhere to avoid double-parsing
	flag.Parse()

	var bootstrap []string
	if p := strings.TrimSpace(*peers); p != "" {
		parts := strings.Split(p, ",")
		for _, part := range parts {
			val := strings.TrimSpace(part)
			if val != "" {
				bootstrap = append(bootstrap, val)
			}
		}
	}

	apiKeys := make(map[string]string)
	if s := strings.TrimSpace(*apiKeysStr); s != "" {
		tokens := strings.Split(s, ",")
		for _, tok := range tokens {
			tok = strings.TrimSpace(tok)
			if tok == "" {
				continue
			}
			key := tok
			nsOverride := ""
			if i := strings.Index(tok, ":"); i != -1 {
				key = strings.TrimSpace(tok[:i])
				nsOverride = strings.TrimSpace(tok[i+1:])
			}
			if key != "" {
				apiKeys[key] = nsOverride
			}
		}
	}

	logger.ComponentInfo(logging.ComponentGeneral, "Loaded gateway configuration",
		zap.String("addr", *addr),
		zap.String("namespace", *ns),
		zap.Int("bootstrap_peer_count", len(bootstrap)),
		zap.Bool("require_auth", *requireAuth),
		zap.Int("api_key_count", len(apiKeys)),
	)

	return &gateway.Config{
		ListenAddr:      *addr,
		ClientNamespace: *ns,
		BootstrapPeers:  bootstrap,
		RequireAuth:     *requireAuth,
		APIKeys:         apiKeys,
	}
}
