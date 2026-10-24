package functions

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// writeFunctionYAML writes a function.yaml into a fresh temp dir and returns it.
func writeFunctionYAML(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "function.yaml"), []byte(body), 0o600); err != nil {
		t.Fatalf("write function.yaml: %v", err)
	}
	return dir
}

func TestLoadConfig_RawHTTPResponse_true(t *testing.T) {
	dir := writeFunctionYAML(t, "name: rpc-proxy\nraw_http_response: true\n")

	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if !cfg.RawHTTPResponse {
		t.Error("RawHTTPResponse = false, want true")
	}
}

func TestLoadConfig_RawHTTPResponse_defaultsFalse(t *testing.T) {
	dir := writeFunctionYAML(t, "name: plain-fn\n")

	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.RawHTTPResponse {
		t.Error("RawHTTPResponse = true, want false (omitted in yaml)")
	}
}

func TestLoadConfig_RawHTTPResponse_explicitFalse(t *testing.T) {
	dir := writeFunctionYAML(t, "name: plain-fn\nraw_http_response: false\n")

	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.RawHTTPResponse {
		t.Error("RawHTTPResponse = true, want false")
	}
}

func TestLoadConfig_WSAuth_capability(t *testing.T) {
	dir := writeFunctionYAML(t, "name: rpc-router\nws_persistent: true\nws_auth: capability\n")

	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.WSAuth != "capability" {
		t.Errorf("WSAuth = %q, want capability", cfg.WSAuth)
	}
}

// bug-740 / feat-938: rate_limit in function.yaml becomes the deploy metadata's
// rate_limit, the window in seconds.
func TestLoadConfig_RateLimit(t *testing.T) {
	dir := writeFunctionYAML(t, "name: user-create\nrate_limit:\n  per_caller:\n    requests: 5\n    window: 1m\n  per_ip:\n    requests: 2\n    window: 1h\n")

	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	got, err := rateLimitMetadata(cfg.RateLimit)
	if err != nil {
		t.Fatalf("rateLimitMetadata: %v", err)
	}
	want := map[string]interface{}{
		"per_caller": map[string]int{"requests": 5, "window_seconds": 60},
		"per_ip":     map[string]int{"requests": 2, "window_seconds": 3600},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("metadata = %#v, want %#v", got, want)
	}
}

func TestLoadConfig_RateLimit_absentIsNone(t *testing.T) {
	cfg, err := LoadConfig(writeFunctionYAML(t, "name: plain-fn\n"))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if got, err := rateLimitMetadata(cfg.RateLimit); got != nil || err != nil {
		t.Errorf("no rate_limit gave %#v, %v", got, err)
	}
}

func TestLoadConfig_RateLimit_refusesAWindowItCannotRead(t *testing.T) {
	for name, window := range map[string]string{"not a duration": "hourly", "fractional seconds": "1500ms", "missing": "\"\""} {
		dir := writeFunctionYAML(t, "name: user-create\nrate_limit:\n  per_ip:\n    requests: 2\n    window: "+window+"\n")
		if _, err := LoadConfig(dir); err == nil || !strings.Contains(err.Error(), "rate_limit.per_ip.window") {
			t.Errorf("%s: got %v, want an error naming rate_limit.per_ip.window", name, err)
		}
	}
}
