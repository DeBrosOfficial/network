package gateway

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Orama-Network/orama/core/pkg/logging"
)

func newTestGateway(t *testing.T) *Gateway {
	logger, err := logging.NewColoredLogger(logging.ComponentGeneral, true)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	return &Gateway{logger: logger}
}

func TestAnonProxyHandler_MethodValidation(t *testing.T) {
	gw := newTestGateway(t)

	// Test GET request (should fail - only POST allowed)
	req := httptest.NewRequest(http.MethodGet, "/v1/proxy/anon", nil)
	w := httptest.NewRecorder()

	gw.anonProxyHandler(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

func TestAnonProxyHandler_InvalidJSON(t *testing.T) {
	gw := newTestGateway(t)

	// Test invalid JSON
	req := httptest.NewRequest(http.MethodPost, "/v1/proxy/anon", bytes.NewBufferString("invalid json"))
	w := httptest.NewRecorder()

	gw.anonProxyHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestAnonProxyHandler_InvalidURL(t *testing.T) {
	gw := newTestGateway(t)

	tests := []struct {
		name    string
		payload anonProxyRequest
	}{
		{
			name: "invalid URL scheme",
			payload: anonProxyRequest{
				URL:    "ftp://example.com",
				Method: "GET",
			},
		},
		{
			name: "malformed URL",
			payload: anonProxyRequest{
				URL:    "://invalid",
				Method: "GET",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.payload)
			req := httptest.NewRequest(http.MethodPost, "/v1/proxy/anon", bytes.NewBuffer(body))
			w := httptest.NewRecorder()

			gw.anonProxyHandler(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
			}
		})
	}
}

func TestAnonProxyHandler_PrivateAddressBlocking(t *testing.T) {
	gw := newTestGateway(t)

	tests := []struct {
		name string
		url  string
	}{
		{"localhost", "http://localhost/test"},
		{"localhost", "http://localhost/test"},
		{"private 10.x", "http://10.0.0.1/test"},
		{"private 192.168.x", "http://192.168.1.1/test"},
		{"private 172.16.x", "http://172.16.0.1/test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := anonProxyRequest{
				URL:    tt.url,
				Method: "GET",
			}
			body, _ := json.Marshal(payload)
			req := httptest.NewRequest(http.MethodPost, "/v1/proxy/anon", bytes.NewBuffer(body))
			w := httptest.NewRecorder()

			gw.anonProxyHandler(w, req)

			if w.Code != http.StatusForbidden {
				t.Errorf("Expected status %d for %s, got %d", http.StatusForbidden, tt.url, w.Code)
			}
		})
	}
}

func TestAnonProxyHandler_InvalidMethod(t *testing.T) {
	gw := newTestGateway(t)

	payload := anonProxyRequest{
		URL:    "https://example.com",
		Method: "INVALID",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/v1/proxy/anon", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	gw.anonProxyHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestIsHopByHopHeader(t *testing.T) {
	tests := []struct {
		header   string
		expected bool
	}{
		{"Connection", true},
		{"Keep-Alive", true},
		{"Proxy-Authorization", true},
		{"Transfer-Encoding", true},
		{"Upgrade", true},
		{"Content-Type", false},
		{"Authorization", false},
		{"User-Agent", false},
	}

	for _, tt := range tests {
		t.Run(tt.header, func(t *testing.T) {
			result := isHopByHopHeader(tt.header)
			if result != tt.expected {
				t.Errorf("isHopByHopHeader(%s) = %v, want %v", tt.header, result, tt.expected)
			}
		})
	}
}

// Bug #712: a reserved destination is DESTINATION_NOT_ALLOWED in every
// notation the exit's resolver reads, before Tor is asked; a public address in
// a numeric notation is not refused. The handler hands the host to Tor
// unresolved, so it is judged on the text (netguard.ReservedHost).
func TestAnonProxyHandler_destinationNotations(t *testing.T) {
	gw := newTestGateway(t)
	torAsked := false
	prev := anonProxyRunning
	anonProxyRunning = func() bool { torAsked = true; return false }
	t.Cleanup(func() { anonProxyRunning = prev })

	send := func(u string) *httptest.ResponseRecorder {
		body, _ := json.Marshal(anonProxyRequest{URL: u, Method: "GET"})
		w := httptest.NewRecorder()
		gw.anonProxyHandler(w, httptest.NewRequest(http.MethodPost, "/v1/proxy/anon", bytes.NewBuffer(body)))
		return w
	}
	for _, u := range []string{
		"http://127.0.0.1/", "http://localhost/", "http://LOCALHOST./", "http://app.localhost/",
		"http://2130706433:10104/", "http://0x7f000001/", "http://017700000001/", "http://0177.0.0.1/",
		"http://127.1/", "http://0x7f.1/", "http://167772161/", "http://10.1/", "http://0xa9fea9fe/latest/meta-data/",
		"http://100.64.0.1/", "http://0.0.0.0:10104/", "http://0/", "http://198.18.0.2/",
		"http://[::1]/", "http://[::ffff:127.0.0.1]:10104/", "http://[::ffff:7f00:1]/", "http://[fe80::1%25eth0]/",
	} {
		torAsked = false
		w := send(u)
		if w.Code != http.StatusForbidden || !bytes.Contains(w.Body.Bytes(), []byte(CodeDestinationNotAllowed)) {
			t.Errorf("%s: got %d %s, want 403 %s", u, w.Code, w.Body.String(), CodeDestinationNotAllowed)
		}
		if torAsked {
			t.Errorf("%s: Tor was asked before the destination was refused", u)
		}
	}
	for _, u := range []string{"https://example.com/", "http://134744072/", "http://8.8.8.8/", "http://[2606:4700:4700::1111]/"} {
		if w := send(u); w.Code != http.StatusServiceUnavailable {
			t.Errorf("%s: got %d %s, want it past validation (503 with Tor down)", u, w.Code, w.Body.String())
		}
	}
}
