package client

import (
	"os"
	"strconv"
	"strings"

	"git.debros.io/DeBros/network/pkg/constants"
	"github.com/multiformats/go-multiaddr"
)

// DefaultBootstrapPeers returns the library's default bootstrap peer multiaddrs.
func DefaultBootstrapPeers() []string {
	// Development local-only override
	if truthy(os.Getenv("NETWORK_DEV_LOCAL")) {
		if ma := os.Getenv("LOCAL_BOOTSTRAP_MULTIADDR"); ma != "" {
			return []string{ma}
		}
		// Fallback to localhost transport without peer ID (connect will warn and skip)
		return []string{"/ip4/127.0.0.1/tcp/4001"}
	}
	peers := constants.GetBootstrapPeers()
	out := make([]string, len(peers))
	copy(out, peers)
	return out
}

// truthy reports if s is a common truthy string.
func truthy(s string) bool {
	switch s {
	case "1", "true", "TRUE", "True", "yes", "YES", "on", "ON":
		return true
	default:
		return false
	}
}

// DefaultDatabaseEndpoints returns default DB HTTP endpoints derived from default bootstrap peers.
// Port defaults to RQLite HTTP 5001, or RQLITE_PORT if set.
func DefaultDatabaseEndpoints() []string {
	port := 5001
	if v := os.Getenv("RQLITE_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			port = n
		}
	}

	// Development local-only override
	if truthy(os.Getenv("NETWORK_DEV_LOCAL")) {
		return []string{"http://127.0.0.1:" + strconv.Itoa(port)}
	}

	peers := DefaultBootstrapPeers()
	if len(peers) == 0 {
		return []string{"http://localhost:" + strconv.Itoa(port)}
	}

	endpoints := make([]string, 0, len(peers))
	for _, s := range peers {
		ma, err := multiaddr.NewMultiaddr(s)
		if err != nil {
			continue
		}
		endpoints = append(endpoints, endpointFromMultiaddr(ma, port))
	}

	out := dedupeStrings(endpoints)
	if len(out) == 0 {
		out = []string{"http://localhost:" + strconv.Itoa(port)}
	}
	return out
}

// MapAddrsToDBEndpoints converts a set of peer multiaddrs to DB HTTP endpoints using dbPort.
func MapAddrsToDBEndpoints(addrs []multiaddr.Multiaddr, dbPort int) []string {
	if dbPort <= 0 {
		dbPort = 5001
	}
	eps := make([]string, 0, len(addrs))
	for _, ma := range addrs {
		eps = append(eps, endpointFromMultiaddr(ma, dbPort))
	}
	return dedupeStrings(eps)
}

func endpointFromMultiaddr(ma multiaddr.Multiaddr, port int) string {
    var host string
    // Prefer DNS if present, then IP
    if v, err := ma.ValueForProtocol(multiaddr.P_DNS); err == nil && v != "" {
        host = v
    }
    if host == "" {
        if v, err := ma.ValueForProtocol(multiaddr.P_DNS4); err == nil && v != "" { host = v }
    }
    if host == "" {
        if v, err := ma.ValueForProtocol(multiaddr.P_DNS6); err == nil && v != "" { host = v }
    }
    if host == "" {
        if v, err := ma.ValueForProtocol(multiaddr.P_IP4); err == nil && v != "" { host = v }
    }
    if host == "" {
        if v, err := ma.ValueForProtocol(multiaddr.P_IP6); err == nil && v != "" { host = v }
    }
    if host == "" {
        host = "localhost"
    }
    return "http://" + host + ":" + strconv.Itoa(port)
}

func dedupeStrings(in []string) []string {
	m := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := m[s]; ok {
			continue
		}
		m[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
