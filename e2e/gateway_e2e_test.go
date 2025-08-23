//go:build e2e

package e2e

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func getEnv(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func requireAPIKey(t *testing.T) string {
	t.Helper()
	key := strings.TrimSpace(os.Getenv("GATEWAY_API_KEY"))
	if key == "" {
		t.Skip("GATEWAY_API_KEY not set; skipping gateway auth-required tests")
	}
	return key
}

func gatewayBaseURL() string {
	return getEnv("GATEWAY_BASE_URL", "http://127.0.0.1:8080")
}

func httpClient() *http.Client {
	return &http.Client{Timeout: 10 * time.Second}
}

func authHeader(key string) http.Header {
	h := http.Header{}
	h.Set("Authorization", "Bearer "+key)
	h.Set("Content-Type", "application/json")
	return h
}

func TestGateway_Health(t *testing.T) {
	base := gatewayBaseURL()
	resp, err := httpClient().Get(base + "/v1/health")
	if err != nil {
		t.Fatalf("health request error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: %d", resp.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("status not ok: %+v", body)
	}
}

func TestGateway_Storage_PutGetListExistsDelete(t *testing.T) {
	key := requireAPIKey(t)
	base := gatewayBaseURL()

	// Unique key and payload
	ts := time.Now().UnixNano()
	kvKey := fmt.Sprintf("e2e-gw/%d", ts)
	payload := randomBytes(32)

	// PUT
	{
		req, err := http.NewRequest(http.MethodPost, base+"/v1/storage/put?key="+url.QueryEscape(kvKey), strings.NewReader(string(payload)))
		if err != nil { t.Fatalf("put new req: %v", err) }
		req.Header = authHeader(key)
		resp, err := httpClient().Do(req)
		if err != nil { t.Fatalf("put do: %v", err) }
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("put status: %d", resp.StatusCode)
		}
	}

	// EXISTS
	{
		req, _ := http.NewRequest(http.MethodGet, base+"/v1/storage/exists?key="+url.QueryEscape(kvKey), nil)
		req.Header = authHeader(key)
		resp, err := httpClient().Do(req)
		if err != nil { t.Fatalf("exists do: %v", err) }
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK { t.Fatalf("exists status: %d", resp.StatusCode) }
		var b struct{ Exists bool `json:"exists"` }
		if err := json.NewDecoder(resp.Body).Decode(&b); err != nil { t.Fatalf("exists decode: %v", err) }
		if !b.Exists { t.Fatalf("exists=false for %s", kvKey) }
	}

	// GET
	{
		req, _ := http.NewRequest(http.MethodGet, base+"/v1/storage/get?key="+url.QueryEscape(kvKey), nil)
		req.Header = authHeader(key)
		resp, err := httpClient().Do(req)
		if err != nil { t.Fatalf("get do: %v", err) }
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK { t.Fatalf("get status: %d", resp.StatusCode) }
		got := make([]byte, len(payload))
		n, _ := resp.Body.Read(got)
		if n == 0 || string(got[:n]) != string(payload[:n]) {
			t.Fatalf("payload mismatch: want %q got %q", string(payload), string(got[:n]))
		}
	}

	// LIST (prefix)
	{
		prefix := url.QueryEscape(strings.Split(kvKey, "/")[0])
		req, _ := http.NewRequest(http.MethodGet, base+"/v1/storage/list?prefix="+prefix, nil)
		req.Header = authHeader(key)
		resp, err := httpClient().Do(req)
		if err != nil { t.Fatalf("list do: %v", err) }
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK { t.Fatalf("list status: %d", resp.StatusCode) }
		var b struct{ Keys []string `json:"keys"` }
		if err := json.NewDecoder(resp.Body).Decode(&b); err != nil { t.Fatalf("list decode: %v", err) }
		found := false
		for _, k := range b.Keys { if k == kvKey { found = true; break } }
		if !found { t.Fatalf("key %s not found in list", kvKey) }
	}

	// DELETE
	{
		req, _ := http.NewRequest(http.MethodPost, base+"/v1/storage/delete?key="+url.QueryEscape(kvKey), nil)
		req.Header = authHeader(key)
		resp, err := httpClient().Do(req)
		if err != nil { t.Fatalf("delete do: %v", err) }
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK { t.Fatalf("delete status: %d", resp.StatusCode) }
	}
}

func TestGateway_PubSub_WS_Echo(t *testing.T) {
	key := requireAPIKey(t)
	base := gatewayBaseURL()

	topic := fmt.Sprintf("e2e-ws-%d", time.Now().UnixNano())
	wsURL, hdr := toWSURL(base+"/v1/pubsub/ws?topic="+url.QueryEscape(topic)), http.Header{}
	hdr.Set("Authorization", "Bearer "+key)

	c, _, err := websocket.DefaultDialer.Dial(wsURL, hdr)
	if err != nil {
		t.Fatalf("ws dial: %v", err)
	}
	defer c.Close()
	defer c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))

	msg := []byte("hello-ws")
	if err := c.WriteMessage(websocket.TextMessage, msg); err != nil {
		t.Fatalf("ws write: %v", err)
	}

	_, data, err := c.ReadMessage()
	if err != nil { t.Fatalf("ws read: %v", err) }
	if string(data) != string(msg) { t.Fatalf("ws echo mismatch: %q", string(data)) }
}

func TestGateway_PubSub_RestPublishToWS(t *testing.T) {
	key := requireAPIKey(t)
	base := gatewayBaseURL()

	topic := fmt.Sprintf("e2e-rest-%d", time.Now().UnixNano())
	wsURL, hdr := toWSURL(base+"/v1/pubsub/ws?topic="+url.QueryEscape(topic)), http.Header{}
	hdr.Set("Authorization", "Bearer "+key)
	c, _, err := websocket.DefaultDialer.Dial(wsURL, hdr)
	if err != nil { t.Fatalf("ws dial: %v", err) }
	defer c.Close()

	// Publish via REST
	payload := randomBytes(24)
	b64 := base64.StdEncoding.EncodeToString(payload)
	body := fmt.Sprintf(`{"topic":"%s","data_base64":"%s"}`, topic, b64)
	req, _ := http.NewRequest(http.MethodPost, base+"/v1/pubsub/publish", strings.NewReader(body))
	req.Header = authHeader(key)
	resp, err := httpClient().Do(req)
	if err != nil { t.Fatalf("publish do: %v", err) }
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK { t.Fatalf("publish status: %d", resp.StatusCode) }

	// Expect the message via WS
	_ = c.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, data, err := c.ReadMessage()
	if err != nil { t.Fatalf("ws read: %v", err) }
	if string(data) != string(payload) { t.Fatalf("payload mismatch: %q != %q", string(data), string(payload)) }

	// Topics list should include our topic (without namespace prefix)
	req2, _ := http.NewRequest(http.MethodGet, base+"/v1/pubsub/topics", nil)
	req2.Header = authHeader(key)
	resp2, err := httpClient().Do(req2)
	if err != nil { t.Fatalf("topics do: %v", err) }
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK { t.Fatalf("topics status: %d", resp2.StatusCode) }
	var tlist struct{ Topics []string `json:"topics"` }
	if err := json.NewDecoder(resp2.Body).Decode(&tlist); err != nil { t.Fatalf("topics decode: %v", err) }
	found := false
	for _, tt := range tlist.Topics { if tt == topic { found = true; break } }
	if !found { t.Fatalf("topic %s not found in topics list", topic) }
}

func toWSURL(httpURL string) string {
	u, err := url.Parse(httpURL)
	if err != nil { return httpURL }
	if u.Scheme == "https" { u.Scheme = "wss" } else { u.Scheme = "ws" }
	return u.String()
}

func randomBytes(n int) []byte {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return b
}
