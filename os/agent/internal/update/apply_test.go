package update

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// releaseServer serves one update whose image is image, described by info
// (its URL is filled in).
func releaseServer(t *testing.T, image []byte, info VersionInfo) *Manager {
	t.Helper()
	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("/v1/latest", func(w http.ResponseWriter, r *http.Request) {
		info.URL = srv.URL + "/image"
		_ = json.NewEncoder(w).Encode(info)
	})
	mux.HandleFunc("/image", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(image) })
	srv = httptest.NewTLSServer(mux)
	t.Cleanup(srv.Close)
	m := NewManager(srv.URL + "/v1/latest")
	m.client = srv.Client()
	return m
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// Every refusal happens before the image is written to the standby partition:
// on this machine the write would be dd to /dev/sda1 or /dev/sda2, so a test
// that got that far would fail on it rather than pass.
func TestCheckAndApply_refusesBeforeWriting(t *testing.T) {
	image := []byte("orama-os image bytes")
	for name, tc := range map[string]struct {
		info VersionInfo
		want string
	}{
		"checksum mismatch": {VersionInfo{Version: "next", SHA256: sha256Hex([]byte("other")), Signature: vectorSigA}, "checksum"},
		"unsigned":          {VersionInfo{Version: "next", SHA256: sha256Hex(image)}, "no signature"},
		"signed by someone else": {VersionInfo{Version: "next", SHA256: sha256Hex(image),
			Signature: vectorSigA}, "signature verification failed"},
		"garbage signature": {VersionInfo{Version: "next", SHA256: sha256Hex(image), Signature: "0xdead"},
			"signature verification failed"},
	} {
		err := releaseServer(t, image, tc.info).checkAndApply()
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: got %v, want an error containing %q", name, err, tc.want)
		}
	}
}

// The version the node runs is not reinstalled. Off a node the installed
// version reads as "unknown".
func TestCheckAndApply_theRunningVersionIsNotReinstalled(t *testing.T) {
	m := releaseServer(t, nil, VersionInfo{Version: readCurrentVersion()})
	if err := m.checkAndApply(); err != nil {
		t.Errorf("got %v, want nothing done", err)
	}
}

func TestVerifyChecksum(t *testing.T) {
	path := writeURLFile(t, "payload")
	if err := verifyChecksum(path, sha256Hex([]byte("payload"))); err != nil {
		t.Errorf("matching checksum refused: %v", err)
	}
	if err := verifyChecksum(path, sha256Hex([]byte("other"))); err == nil {
		t.Error("a wrong checksum passed")
	}
	if err := verifyChecksum(path+".missing", sha256Hex([]byte("payload"))); err == nil {
		t.Error("a missing file passed")
	}
}

func TestStop_isIdempotent(t *testing.T) {
	m := NewManager(DefaultUpdateURL)
	m.Stop()
	m.Stop()
}
