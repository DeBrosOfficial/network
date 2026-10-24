package gatewayspec

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestGatewayYAMLConfig_clusterSecretPathRoundTrip is the regression test for
// bug #215 reopen. The first fix derived JWT keys from cfg.ClusterSecret, but
// namespace gateways spawned via systemd had cfg.ClusterSecret == "" because
// the YAML schema lacked any field to carry it. This test guards the YAML tag
// so every namespace gateway YAML written by SystemdSpawner.SpawnGateway
// carries the path the standalone binary needs.
func TestGatewayYAMLConfig_clusterSecretPathRoundTrip(t *testing.T) {
	cfg := GatewayYAMLConfig{
		ListenAddr:        ":6001",
		ClientNamespace:   "anchat-test",
		RQLiteDSN:         "http://localhost:10000",
		OlricServers:      []string{"localhost:3320"},
		ClusterSecretPath: "/opt/orama/.orama/secrets/cluster-secret",
	}
	out, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(out), "cluster_secret_path: /opt/orama/.orama/secrets/cluster-secret") {
		t.Fatalf("YAML output missing expected cluster_secret_path line:\n%s", out)
	}

	// Round-trip into a struct shaped like cmd/gateway/config.go's yamlCfg
	// (internal type, duplicated here intentionally so this test catches
	// drift between the two declarations).
	type webrtc struct {
		Enabled    bool   `yaml:"enabled"`
		SFUPort    int    `yaml:"sfu_port"`
		TURNDomain string `yaml:"turn_domain"`
		TURNSecret string `yaml:"turn_secret"`
	}
	type yamlCfgMirror struct {
		ListenAddr            string   `yaml:"listen_addr"`
		ClientNamespace       string   `yaml:"client_namespace"`
		RQLiteDSN             string   `yaml:"rqlite_dsn"`
		GlobalRQLiteDSN       string   `yaml:"global_rqlite_dsn"`
		Peers                 []string `yaml:"bootstrap_peers"`
		EnableHTTPS           bool     `yaml:"enable_https"`
		DomainName            string   `yaml:"domain_name"`
		TLSCacheDir           string   `yaml:"tls_cache_dir"`
		OlricServers          []string `yaml:"olric_servers"`
		OlricTimeout          string   `yaml:"olric_timeout"`
		IPFSClusterAPIURL     string   `yaml:"ipfs_cluster_api_url"`
		IPFSAPIURL            string   `yaml:"ipfs_api_url"`
		IPFSTimeout           string   `yaml:"ipfs_timeout"`
		IPFSReplicationFactor int      `yaml:"ipfs_replication_factor"`
		WebRTC                webrtc   `yaml:"webrtc"`
		ClusterSecretPath     string   `yaml:"cluster_secret_path"`
	}
	var parsed yamlCfgMirror
	if err := yaml.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if parsed.ClusterSecretPath != cfg.ClusterSecretPath {
		t.Errorf("round-trip mismatch: got %q, want %q", parsed.ClusterSecretPath, cfg.ClusterSecretPath)
	}
}

// TestGatewayYAMLConfig_secretsEncryptionKeyRoundTrip is the regression test
// for the bugboard #837 follow-up: the host gateway received the serverless
// secrets encryption key but namespace gateways spawned via systemd did not,
// because the YAML schema had no field to carry it — so `function secrets
// list` returned 501 on those namespaces. This guards the yaml tag and that
// the standalone gateway's yamlCfg mirror can read it back.
func TestGatewayYAMLConfig_secretsEncryptionKeyRoundTrip(t *testing.T) {
	const key = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	cfg := GatewayYAMLConfig{
		ListenAddr:           ":6001",
		ClientNamespace:      "anchat-test",
		RQLiteDSN:            "http://localhost:10000",
		OlricServers:         []string{"localhost:3320"},
		SecretsEncryptionKey: key,
	}
	out, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(out), "secrets_encryption_key: "+key) {
		t.Fatalf("YAML output missing expected secrets_encryption_key line:\n%s", out)
	}

	// Mirror of cmd/gateway/config.go's yamlCfg so this test catches drift
	// between the two declarations (the standalone gateway uses strict
	// decoding and would reject an unknown field).
	type webrtc struct {
		Enabled    bool   `yaml:"enabled"`
		SFUPort    int    `yaml:"sfu_port"`
		TURNDomain string `yaml:"turn_domain"`
		TURNSecret string `yaml:"turn_secret"`
	}
	type yamlCfgMirror struct {
		ListenAddr           string   `yaml:"listen_addr"`
		ClientNamespace      string   `yaml:"client_namespace"`
		RQLiteDSN            string   `yaml:"rqlite_dsn"`
		OlricServers         []string `yaml:"olric_servers"`
		WebRTC               webrtc   `yaml:"webrtc"`
		SecretsEncryptionKey string   `yaml:"secrets_encryption_key"`
		ClusterSecretPath    string   `yaml:"cluster_secret_path"`
	}
	var parsed yamlCfgMirror
	if err := yaml.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if parsed.SecretsEncryptionKey != key {
		t.Errorf("round-trip mismatch: got %q, want %q", parsed.SecretsEncryptionKey, key)
	}
}

// TestGatewayYAMLConfig_secretsKeyOmitWhenEmpty: a host with no secrets key
// (legacy/test rigs) must not emit a stray secrets_encryption_key line that
// operators could mistake for an empty-key directive.
func TestGatewayYAMLConfig_secretsKeyOmitWhenEmpty(t *testing.T) {
	cfg := GatewayYAMLConfig{
		ListenAddr:      ":6001",
		ClientNamespace: "ns",
		RQLiteDSN:       "http://localhost:10000",
		OlricServers:    []string{"localhost:3320"},
		// SecretsEncryptionKey intentionally empty.
	}
	out, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(out), "secrets_encryption_key") {
		t.Errorf("empty SecretsEncryptionKey should be omitted from YAML; got:\n%s", out)
	}
}

// TestGatewayYAMLConfig_omitWhenEmpty: when the host has no cluster secret,
// the field is omitted from the YAML so legacy single-node test rigs don't
// see a stray "cluster_secret_path: " line that operators might mistake for
// a configuration directive.
func TestGatewayYAMLConfig_omitWhenEmpty(t *testing.T) {
	cfg := GatewayYAMLConfig{
		ListenAddr:      ":6001",
		ClientNamespace: "ns",
		RQLiteDSN:       "http://localhost:10000",
		OlricServers:    []string{"localhost:3320"},
		// ClusterSecretPath intentionally empty.
	}
	out, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(out), "cluster_secret_path") {
		t.Errorf("empty ClusterSecretPath should be omitted from YAML; got:\n%s", out)
	}
}

// TestGatewayYAMLConfig_ntfyBaseURLRoundTrip pins the YAML tag the standalone
// gateway binary reads the value back from.
func TestGatewayYAMLConfig_ntfyBaseURLRoundTrip(t *testing.T) {
	const base = "https://push.devnet.orama.network"
	cfg := GatewayYAMLConfig{
		ListenAddr:      ":6001",
		ClientNamespace: "anchat-v2",
		RQLiteDSN:       "http://localhost:10005",
		NtfyBaseURL:     base,
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(data), "ntfy_base_url: "+base) {
		t.Fatalf("ntfy_base_url missing from marshalled YAML:\n%s", data)
	}

	var back GatewayYAMLConfig
	if err := yaml.Unmarshal(data, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if back.NtfyBaseURL != base {
		t.Errorf("NtfyBaseURL = %q; want %q", back.NtfyBaseURL, base)
	}
}
