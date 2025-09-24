package main

import (
	"flag"
	"os"
	"strings"

	"github.com/DeBrosOfficial/network/pkg/gateway"
	"github.com/DeBrosOfficial/network/pkg/logging"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
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

// parseGatewayConfig loads optional configs/gateway.yaml then applies env and flags.
// Priority: flags > env > yaml > defaults.
func parseGatewayConfig(logger *logging.ColoredLogger) *gateway.Config {
	// Base defaults
	cfg := &gateway.Config{
		ListenAddr:      ":6001",
		ClientNamespace: "default",
		BootstrapPeers:  nil,
	}

	// 1) YAML (optional)
	{
		type yamlCfg struct {
			ListenAddr      string   `yaml:"listen_addr"`
			ClientNamespace string   `yaml:"client_namespace"`
			BootstrapPeers  []string `yaml:"bootstrap_peers"`
		}
		const path = "configs/gateway.yaml"
		if data, err := os.ReadFile(path); err == nil {
			var y yamlCfg
			if err := yaml.Unmarshal(data, &y); err != nil {
				logger.ComponentWarn(logging.ComponentGeneral, "failed to parse configs/gateway.yaml; ignoring", zap.Error(err))
			} else {
				if v := strings.TrimSpace(y.ListenAddr); v != "" {
					cfg.ListenAddr = v
				}
				if v := strings.TrimSpace(y.ClientNamespace); v != "" {
					cfg.ClientNamespace = v
				}
				if len(y.BootstrapPeers) > 0 {
					var bp []string
					for _, p := range y.BootstrapPeers {
						p = strings.TrimSpace(p)
						if p != "" {
							bp = append(bp, p)
						}
					}
					if len(bp) > 0 {
						cfg.BootstrapPeers = bp
					}
				}
			}
		}
	}

	// 2) Env overrides
	if v := strings.TrimSpace(os.Getenv("GATEWAY_ADDR")); v != "" {
		cfg.ListenAddr = v
	}
	if v := strings.TrimSpace(os.Getenv("GATEWAY_NAMESPACE")); v != "" {
		cfg.ClientNamespace = v
	}
	if v := strings.TrimSpace(os.Getenv("GATEWAY_BOOTSTRAP_PEERS")); v != "" {
		parts := strings.Split(v, ",")
		var bp []string
		for _, part := range parts {
			s := strings.TrimSpace(part)
			if s != "" {
				bp = append(bp, s)
			}
		}
		cfg.BootstrapPeers = bp
	}
	if v := strings.TrimSpace(os.Getenv("RQLITE_DSN")); v != "" {
		cfg.RQLiteDSN = v
	}

	// 3) Flags (override env)
	addr := flag.String("addr", "", "HTTP listen address (e.g., :6001)")
	ns := flag.String("namespace", "", "Client namespace for scoping resources")
	peers := flag.String("bootstrap-peers", "", "Comma-separated bootstrap peers for network client")
	rqliteDSN := flag.String("rqlite-dsn", "", "RQLite database DSN (e.g., http://localhost:5001)")

	// Do not call flag.Parse() elsewhere to avoid double-parsing
	flag.Parse()

	if a := strings.TrimSpace(*addr); a != "" {
		cfg.ListenAddr = a
	}
	if n := strings.TrimSpace(*ns); n != "" {
		cfg.ClientNamespace = n
	}
	if p := strings.TrimSpace(*peers); p != "" {
		parts := strings.Split(p, ",")
		var bp []string
		for _, part := range parts {
			s := strings.TrimSpace(part)
			if s != "" {
				bp = append(bp, s)
			}
		}
		cfg.BootstrapPeers = bp
	}
	if r := strings.TrimSpace(*rqliteDSN); r != "" {
		cfg.RQLiteDSN = r
	}

	logger.ComponentInfo(logging.ComponentGeneral, "Loaded gateway configuration",
		zap.String("addr", cfg.ListenAddr),
		zap.String("namespace", cfg.ClientNamespace),
		zap.Int("bootstrap_peer_count", len(cfg.BootstrapPeers)),
	)

	return cfg
}
