// Package corsorigins decides which browser origins may read a gateway's
// answers: the built-in rule every gateway applies (its base domain, loopback
// and the Tauri webviews) and the origins each namespace adds for its own
// apps, stored in the cluster registry and cached per namespace.
package corsorigins

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

const (
	// MaxOriginsPerNamespace bounds what a namespace may list. An app has a
	// handful of front ends; the list is answered from memory on every
	// request to the namespace's host, and a long one is a sign of a
	// wildcard being spelled out.
	MaxOriginsPerNamespace = 20

	// maxOriginLength is the longest origin accepted: a scheme, a DNS name of
	// at most 253 characters and a port.
	maxOriginLength = 300

	// schemeTauri is the scheme Tauri's webview serves an app from on macOS,
	// Linux and iOS (tauri://localhost). On Windows and Android the webview
	// cannot serve a custom scheme and Tauri uses http://tauri.localhost
	// (https://tauri.localhost with useHttpsScheme), which the loopback rule
	// already admits.
	schemeTauri = "tauri"
)

// allowedSchemes are the schemes an origin may carry, built in or listed.
var allowedSchemes = map[string]bool{"http": true, "https": true, schemeTauri: true}

// Canonical parses a browser Origin header value into its canonical form,
// scheme://host[:port], lower case and with the scheme's default port
// dropped, or returns an error saying why it is not one. "null" and "*" are
// not origins: a page with an opaque origin must not be let in by listing it,
// and a wildcard would admit every site on the web.
func Canonical(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("an origin is required")
	}
	if len(raw) > maxOriginLength {
		return "", fmt.Errorf("an origin is at most %d characters", maxOriginLength)
	}
	if strings.Contains(raw, "*") {
		return "", fmt.Errorf("%q: wildcards are not allowed; list each origin", raw)
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("%q is not an origin: %w", raw, err)
	}
	scheme := strings.ToLower(u.Scheme)
	if !allowedSchemes[scheme] {
		return "", fmt.Errorf("%q: the scheme is http, https or tauri", raw)
	}
	if u.User != nil || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" {
		return "", fmt.Errorf("%q: an origin is scheme://host[:port], with no path, query or credentials", raw)
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return "", fmt.Errorf("%q has no host", raw)
	}
	port := u.Port()
	if (scheme == "http" && port == "80") || (scheme == "https" && port == "443") {
		port = ""
	}
	hostPart := host
	if strings.Contains(host, ":") {
		hostPart = "[" + host + "]"
	}
	if port != "" {
		return scheme + "://" + hostPart + ":" + port, nil
	}
	return scheme + "://" + hostPart, nil
}

// BuiltIn reports whether every gateway admits origin, whatever any namespace
// lists: the base domain and the names under it, and a page served from this
// machine.
//
// Loopback is localhost, any name under .localhost (RFC 6761 reserves the
// whole tree for the loopback interface, and browsers resolve it there), and
// the loopback addresses. It is what a developer's dev server and a desktop
// app's webview are served from: Tauri's tauri://localhost on macOS, Linux and
// iOS, and http(s)://tauri.localhost on Windows and Android. The host is
// compared whole, so tauri.localhost.example.com is not loopback.
func BuiltIn(origin, baseDomain string) bool {
	canonical, err := Canonical(origin)
	if err != nil {
		return false
	}
	u, err := url.Parse(canonical)
	if err != nil {
		return false
	}
	host := u.Hostname()
	baseDomain = strings.ToLower(strings.TrimSpace(baseDomain))
	if baseDomain != "" && (host == baseDomain || strings.HasSuffix(host, "."+baseDomain)) {
		return true
	}
	return isLoopbackHost(host)
}

func isLoopbackHost(host string) bool {
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
