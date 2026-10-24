package gateway

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Orama-Network/orama/core/pkg/anonproxy"
	"github.com/Orama-Network/orama/core/pkg/logging"
	"github.com/Orama-Network/orama/core/pkg/netguard"
	"go.uber.org/zap"
)

// anonProxyRequest represents the JSON payload for proxy requests
type anonProxyRequest struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body,omitempty"`
}

// anonProxyResponse represents the JSON response from proxy requests
type anonProxyResponse struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
	Error      string            `json:"error,omitempty"`
}

const (
	maxProxyRequestSize = 10 * 1024 * 1024 // 10MB
	maxProxyTimeout     = 60 * time.Second
)

// The Tor client the proxy goes through. Tests replace them: the real ones
// need a Tor SOCKS port on a fixed address.
var (
	anonProxyRunning = anonproxy.Running
	anonProxyClient  = anonproxy.NewHTTPClient
)

// anonProxyHandler handles proxied HTTP requests through the Tor network
func (g *Gateway) anonProxyHandler(w http.ResponseWriter, r *http.Request) {
	// Only accept POST requests
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "only POST method is allowed")
		return
	}

	// Limit request body size
	r.Body = http.MaxBytesReader(w, r.Body, maxProxyRequestSize)

	// Parse request payload
	var req anonProxyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON payload: %v", err))
		return
	}

	// Validate URL
	targetURL, err := url.Parse(req.URL)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid URL: %v", err))
		return
	}

	// Only allow HTTPS for external requests
	if targetURL.Scheme != "https" && targetURL.Scheme != "http" {
		writeError(w, http.StatusBadRequest, "only http/https schemes are allowed")
		return
	}

	// Block requests to private/local addresses. The host goes to Tor
	// unresolved, so it is judged as a resolver reads it (netguard.ReservedHost).
	if netguard.ReservedHost(targetURL.Hostname()) {
		forbidden(w, CodeDestinationNotAllowed, "requests to private or local addresses are not allowed", nil)
		return
	}

	// Validate HTTP method
	method := strings.ToUpper(req.Method)
	if method == "" {
		method = "GET"
	}
	allowedMethods := map[string]bool{
		"GET":    true,
		"POST":   true,
		"PUT":    true,
		"DELETE": true,
		"PATCH":  true,
		"HEAD":   true,
	}
	if !allowedMethods[method] {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("method %s not allowed", method))
		return
	}

	// Check the Tor SOCKS port is up (after all validation)
	if !anonProxyRunning() {
		g.logger.ComponentWarn(logging.ComponentGeneral, "Tor proxy not available",
			zap.String("socks_addr", anonproxy.Address()))
		writeJSON(w, http.StatusServiceUnavailable, anonProxyResponse{
			Error: "Tor proxy not available on this node",
		})
		return
	}

	// Every connection of this client goes through Tor
	client := anonProxyClient()
	client.Timeout = maxProxyTimeout

	// Create the proxied request
	var bodyReader io.Reader
	if req.Body != "" {
		bodyReader = strings.NewReader(req.Body)
	}

	proxyReq, err := http.NewRequestWithContext(r.Context(), method, req.URL, bodyReader)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create request: %v", err))
		return
	}

	// Copy headers, excluding hop-by-hop headers
	for key, value := range req.Headers {
		if !isHopByHopHeader(key) {
			proxyReq.Header.Set(key, value)
		}
	}

	// Set default User-Agent if not provided
	if proxyReq.Header.Get("User-Agent") == "" {
		proxyReq.Header.Set("User-Agent", "Orama-Gateway/1.0")
	}

	// Execute the request
	start := time.Now()
	resp, err := client.Do(proxyReq)
	duration := time.Since(start)

	if err != nil {
		g.logger.ComponentError(logging.ComponentGeneral, "proxy request failed",
			zap.String("method", method),
			zap.String("error_class", anonproxy.ErrorClass(err)),
			zap.Duration("duration", duration))
		writeJSON(w, http.StatusBadGateway, anonProxyResponse{
			Error: "could not reach the destination through the anonymity network",
		})
		return
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxProxyRequestSize))
	if err != nil {
		g.logger.ComponentError(logging.ComponentGeneral, "failed to read proxy response",
			zap.String("method", method),
			zap.String("error_class", anonproxy.ErrorClass(err)))
		writeJSON(w, http.StatusBadGateway, anonProxyResponse{
			Error: "the destination's response could not be read through the anonymity network",
		})
		return
	}

	// Extract response headers (excluding hop-by-hop)
	respHeaders := make(map[string]string)
	for key, values := range resp.Header {
		if !isHopByHopHeader(key) && len(values) > 0 {
			respHeaders[key] = values[0]
		}
	}

	g.logger.ComponentInfo(logging.ComponentGeneral, "proxy request completed",
		zap.String("method", method),
		zap.Int("status", resp.StatusCode),
		zap.Int("bytes", len(respBody)),
		zap.Duration("duration", duration))

	// Base64-encode the body to safely handle binary data in JSON
	// This prevents corruption when binary data is converted to a string
	bodyBase64 := base64.StdEncoding.EncodeToString(respBody)

	// Return the proxied response
	writeJSON(w, http.StatusOK, anonProxyResponse{
		StatusCode: resp.StatusCode,
		Headers:    respHeaders,
		Body:       bodyBase64,
	})
}

// isHopByHopHeader returns true for HTTP hop-by-hop headers that should not be forwarded
func isHopByHopHeader(header string) bool {
	hopByHop := map[string]bool{
		"Connection":          true,
		"Keep-Alive":          true,
		"Proxy-Authenticate":  true,
		"Proxy-Authorization": true,
		"Te":                  true,
		"Trailers":            true,
		"Transfer-Encoding":   true,
		"Upgrade":             true,
	}
	return hopByHop[http.CanonicalHeaderKey(header)]
}
