package update

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeURLFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "update-url")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestResolveUpdateURL_absentFileIsTheDefault(t *testing.T) {
	got, err := ResolveUpdateURL(filepath.Join(t.TempDir(), "update-url"))
	if err != nil {
		t.Fatal(err)
	}
	if got != DefaultUpdateURL {
		t.Fatalf("got %q, want %q", got, DefaultUpdateURL)
	}
}

func TestResolveUpdateURL_imageOverride(t *testing.T) {
	got, err := ResolveUpdateURL(writeURLFile(t, "https://updates.test.example:8443/v1/latest\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://updates.test.example:8443/v1/latest" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveUpdateURL_refusesWhatIsNotAPlainHTTPSURL(t *testing.T) {
	for _, bad := range []string{
		"",
		"http://updates.test.example/v1/latest",
		"updates.test.example/v1/latest",
		"https:///v1/latest",
		"https://updates.test.example/v1/latest?arch=arm64",
		"https://updates.test.example/v1/latest#x",
		"https://bad host/v1/latest",
	} {
		if got, err := ResolveUpdateURL(writeURLFile(t, bad)); err == nil {
			t.Errorf("%q: accepted as %q, want an error", bad, got)
		}
	}
}

// The check goes to the configured endpoint with the node's arch, and a 204
// (nothing newer) installs nothing.
func TestCheckAndApply_queriesTheConfiguredEndpoint(t *testing.T) {
	var gotPath, gotArch string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotArch = r.URL.Path, r.URL.Query().Get("arch")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	m := NewManager(srv.URL + "/v1/latest")
	m.client = srv.Client()
	if err := m.checkAndApply(); err != nil {
		t.Fatal(err)
	}
	if gotPath != "/v1/latest" {
		t.Fatalf("checked %q, want /v1/latest", gotPath)
	}
	if gotArch != detectArch() {
		t.Fatalf("arch %q, want %q", gotArch, detectArch())
	}
}

func TestCheckAndApply_reportsAServerError(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	m := NewManager(srv.URL + "/v1/latest")
	m.client = srv.Client()
	err := m.checkAndApply()
	if err == nil || !strings.Contains(err.Error(), "502") {
		t.Fatalf("got %v, want the 502 reported", err)
	}
}
