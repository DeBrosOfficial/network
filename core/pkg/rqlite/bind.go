package rqlite

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

// BindAddr is the host:port rqlited should listen on. adv is the advertised
// address (WireGuard IP:port). Binding 0.0.0.0 / :: is refused; without an
// advertise host we fall back to loopback so a single-node without mesh still
// starts.
func BindAddr(adv string, port int) (string, error) {
	host := BindHost(adv)
	if host == "0.0.0.0" || host == "::" || host == "[::]" {
		return "", fmt.Errorf("rqlite must not bind %q", host)
	}
	return net.JoinHostPort(host, strconv.Itoa(port)), nil
}

// BindHost is the IP rqlited (and local DSNs) should use. Empty or wildcard
// advertise addresses become 127.0.0.1.
func BindHost(adv string) string {
	adv = strings.TrimSpace(adv)
	if adv == "" {
		return "127.0.0.1"
	}
	if host, _, err := net.SplitHostPort(adv); err == nil {
		host = strings.Trim(host, "[]")
		if host == "" || host == "0.0.0.0" || host == "::" {
			return "127.0.0.1"
		}
		return host
	}
	if ip := net.ParseIP(strings.Trim(adv, "[]")); ip != nil {
		if ip.IsUnspecified() {
			return "127.0.0.1"
		}
		return ip.String()
	}
	return "127.0.0.1"
}
