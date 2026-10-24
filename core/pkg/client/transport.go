package client

import (
	"net/http"
	"strings"
	"time"
)

// getGatewayURL returns the gateway URL from config, defaulting to localhost:10104
func getGatewayURL(c *Client) string {
	cfg := c.Config()
	if cfg != nil && cfg.GatewayURL != "" {
		return strings.TrimSuffix(cfg.GatewayURL, "/")
	}
	return "http://localhost:10104"
}

// addAuthHeaders adds authentication headers to the request
func addAuthHeaders(req *http.Request, c *Client) {
	cfg := c.Config()
	if cfg == nil {
		return
	}

	// Prefer JWT if available
	if cfg.JWT != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.JWT)
		return
	}

	// Fallback to API key
	if cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
		req.Header.Set("X-API-Key", cfg.APIKey)
	}
}

// gatewayHTTPClient returns the http.Client for one request to the gateway,
// bounded by timeout. It sends through the configured HTTPClient's Transport
// and Jar, so one injected transport (a private CA, a proxy) reaches every
// sub-client. A redirect is answered as it is, never followed: Go strips
// Authorization on a cross-host hop but forwards X-API-Key, which a request
// with an API key carries too.
func gatewayHTTPClient(c *Client, timeout time.Duration) *http.Client {
	hc := &http.Client{
		Timeout:       timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	if cfg := c.Config(); cfg != nil && cfg.HTTPClient != nil {
		hc.Transport = cfg.HTTPClient.Transport
		hc.Jar = cfg.HTTPClient.Jar
	}
	return hc
}
