package gateway

import (
	"strings"
	"testing"
)

func TestValidateListenAddr(t *testing.T) {
	tests := []struct {
		name      string
		addr      string
		wantErr   bool
		errSubstr string
	}{
		{"valid :8080", ":8080", false, ""},
		{"valid 0.0.0.0:443", "0.0.0.0:443", false, ""},
		{"valid 127.0.0.1:6001", "127.0.0.1:6001", false, ""},
		{"valid :80", ":80", false, ""},
		{"valid high port", ":65535", false, ""},
		{"invalid no colon", "8080", true, "invalid format"},
		{"invalid port zero", ":0", true, "port must be a number"},
		{"invalid port too high", ":99999", true, "port must be a number"},
		{"invalid non-numeric port", ":abc", true, "port must be a number"},
		{"empty string", "", true, "invalid format"},
		{"invalid negative port", ":-1", true, "port must be a number"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateListenAddr(tt.addr)
			if tt.wantErr {
				if err == nil {
					t.Errorf("validateListenAddr(%q) = nil, want error containing %q", tt.addr, tt.errSubstr)
				} else if tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("validateListenAddr(%q) error = %q, want error containing %q", tt.addr, err.Error(), tt.errSubstr)
				}
			} else {
				if err != nil {
					t.Errorf("validateListenAddr(%q) = %v, want nil", tt.addr, err)
				}
			}
		})
	}
}

func TestValidateRQLiteDSN(t *testing.T) {
	tests := []struct {
		name      string
		dsn       string
		wantErr   bool
		errSubstr string
	}{
		{"valid http localhost", "http://localhost:4001", false, ""},
		{"valid https", "https://db.example.com", false, ""},
		{"valid http with path", "http://192.168.1.1:4001/db", false, ""},
		{"valid https with port", "https://db.example.com:4001", false, ""},
		{"invalid scheme ftp", "ftp://localhost", true, "scheme must be http or https"},
		{"invalid scheme tcp", "tcp://localhost:4001", true, "scheme must be http or https"},
		{"missing host", "http://", true, "host must not be empty"},
		{"no scheme", "localhost:4001", true, "scheme must be http or https"},
		{"empty string", "", true, "scheme must be http or https"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRQLiteDSN(tt.dsn)
			if tt.wantErr {
				if err == nil {
					t.Errorf("validateRQLiteDSN(%q) = nil, want error containing %q", tt.dsn, tt.errSubstr)
				} else if tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("validateRQLiteDSN(%q) error = %q, want error containing %q", tt.dsn, err.Error(), tt.errSubstr)
				}
			} else {
				if err != nil {
					t.Errorf("validateRQLiteDSN(%q) = %v, want nil", tt.dsn, err)
				}
			}
		})
	}
}

func TestExtractTCPPort_Gateway(t *testing.T) {
	tests := []struct {
		name      string
		multiaddr string
		want      string
	}{
		{
			"standard multiaddr",
			"/ip4/127.0.0.1/tcp/4001/p2p/12D3KooWExample",
			"4001",
		},
		{
			"no tcp component",
			"/ip4/127.0.0.1/udp/4001",
			"",
		},
		{
			"multiple tcp segments uses last",
			"/ip4/127.0.0.1/tcp/4001/tcp/5001/p2p/12D3KooWExample",
			"5001",
		},
		{
			"tcp port at end",
			"/ip4/0.0.0.0/tcp/8080",
			"8080",
		},
		{
			"empty string",
			"",
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractTCPPort(tt.multiaddr)
			if got != tt.want {
				t.Errorf("extractTCPPort(%q) = %q, want %q", tt.multiaddr, got, tt.want)
			}
		})
	}
}

func TestValidateConfig_Empty(t *testing.T) {
	cfg := &Config{}
	errs := cfg.ValidateConfig()

	if len(errs) == 0 {
		t.Fatal("empty config should produce validation errors")
	}

	// Should have errors for listen_addr and client_namespace at minimum
	var foundListenAddr, foundClientNamespace bool
	for _, err := range errs {
		msg := err.Error()
		if strings.Contains(msg, "listen_addr") {
			foundListenAddr = true
		}
		if strings.Contains(msg, "client_namespace") {
			foundClientNamespace = true
		}
	}

	if !foundListenAddr {
		t.Error("expected validation error for listen_addr, got none")
	}
	if !foundClientNamespace {
		t.Error("expected validation error for client_namespace, got none")
	}
}

func TestValidateConfig_ValidMinimal(t *testing.T) {
	cfg := &Config{
		ListenAddr:      ":8080",
		ClientNamespace: "default",
	}
	errs := cfg.ValidateConfig()

	if len(errs) > 0 {
		t.Errorf("valid minimal config should not produce errors, got: %v", errs)
	}
}

func TestValidateConfig_DuplicateBootstrapPeers(t *testing.T) {
	peer := "/ip4/127.0.0.1/tcp/4001/p2p/12D3KooWHbcFcrGPXKUrHcxvd8MXEeUzRYyvY8fQcpEBxncSUwhj"
	cfg := &Config{
		ListenAddr:      ":8080",
		ClientNamespace: "default",
		BootstrapPeers:  []string{peer, peer},
	}
	errs := cfg.ValidateConfig()

	var foundDuplicate bool
	for _, err := range errs {
		if strings.Contains(err.Error(), "duplicate") {
			foundDuplicate = true
			break
		}
	}

	if !foundDuplicate {
		t.Error("expected duplicate bootstrap peer error, got none")
	}
}

func TestValidateConfig_InvalidMultiaddr(t *testing.T) {
	cfg := &Config{
		ListenAddr:      ":8080",
		ClientNamespace: "default",
		BootstrapPeers:  []string{"not-a-multiaddr"},
	}
	errs := cfg.ValidateConfig()

	if len(errs) == 0 {
		t.Fatal("invalid multiaddr should produce validation error")
	}

	var foundInvalid bool
	for _, err := range errs {
		if strings.Contains(err.Error(), "invalid multiaddr") {
			foundInvalid = true
			break
		}
	}

	if !foundInvalid {
		t.Errorf("expected 'invalid multiaddr' error, got: %v", errs)
	}
}

func TestValidateConfig_MissingP2PComponent(t *testing.T) {
	cfg := &Config{
		ListenAddr:      ":8080",
		ClientNamespace: "default",
		BootstrapPeers:  []string{"/ip4/127.0.0.1/tcp/4001"},
	}
	errs := cfg.ValidateConfig()

	var foundMissingP2P bool
	for _, err := range errs {
		if strings.Contains(err.Error(), "missing /p2p/") {
			foundMissingP2P = true
			break
		}
	}

	if !foundMissingP2P {
		t.Errorf("expected 'missing /p2p/' error, got: %v", errs)
	}
}

func TestValidateConfig_InvalidListenAddr(t *testing.T) {
	cfg := &Config{
		ListenAddr:      "not-valid",
		ClientNamespace: "default",
	}
	errs := cfg.ValidateConfig()

	if len(errs) == 0 {
		t.Fatal("invalid listen_addr should produce validation error")
	}

	var foundListenAddr bool
	for _, err := range errs {
		if strings.Contains(err.Error(), "listen_addr") {
			foundListenAddr = true
			break
		}
	}

	if !foundListenAddr {
		t.Errorf("expected listen_addr error, got: %v", errs)
	}
}

func TestValidateConfig_InvalidRQLiteDSN(t *testing.T) {
	cfg := &Config{
		ListenAddr:      ":8080",
		ClientNamespace: "default",
		RQLiteDSN:       "ftp://invalid",
	}
	errs := cfg.ValidateConfig()

	var foundDSN bool
	for _, err := range errs {
		if strings.Contains(err.Error(), "rqlite_dsn") {
			foundDSN = true
			break
		}
	}

	if !foundDSN {
		t.Errorf("expected rqlite_dsn error, got: %v", errs)
	}
}

func TestValidateConfig_DomainNameWithoutHTTPSIsFine(t *testing.T) {
	cfg := &Config{
		ListenAddr:      ":10104",
		ClientNamespace: "default",
		DomainName:      "example.com",
	}
	errs := cfg.ValidateConfig()
	if len(errs) > 0 {
		t.Errorf("domain_name without enable_https must be valid (Caddy terminates TLS), got: %v", errs)
	}
}

func TestValidateConfig_EmptyRQLiteDSNSkipped(t *testing.T) {
	cfg := &Config{
		ListenAddr:      ":8080",
		ClientNamespace: "default",
		RQLiteDSN:       "",
	}
	errs := cfg.ValidateConfig()

	for _, err := range errs {
		if strings.Contains(err.Error(), "rqlite_dsn") {
			t.Errorf("empty rqlite_dsn should not produce error, got: %v", err)
		}
	}
}
