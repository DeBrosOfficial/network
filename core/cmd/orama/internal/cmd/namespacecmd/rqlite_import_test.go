package namespacecmd

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/spf13/cobra"
)

// importJWT is shaped like a token, so ORAMA_TOKEN is sent as it is.
const importJWT = "eyJhbGciOiJFZERTQSJ9.eyJzdWIiOiIweG93bmVyIn0.c2ln"

// importGateway answers whoami for namespace "acme" and counts the imports.
func importGateway(t *testing.T) (imports func() int) {
	t.Helper()
	var mu sync.Mutex
	n := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/auth/whoami":
			_, _ = w.Write([]byte(`{"authenticated":true,"namespace":"acme"}`))
		case "/v1/rqlite/import":
			mu.Lock()
			n++
			mu.Unlock()
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ORAMA_API_URL", srv.URL)
	t.Setenv("ORAMA_GATEWAY_URL", "")
	t.Setenv("ORAMA_GATEWAY", "")
	t.Setenv("ORAMA_TOKEN", importJWT)
	return func() int {
		mu.Lock()
		defer mu.Unlock()
		return n
	}
}

// importCommand is `orama namespace rqlite import -i <file>` with the given
// extra flags, reading stdin from in.
func importCommand(t *testing.T, in string, flags ...string) *cobra.Command {
	t.Helper()
	file := filepath.Join(t.TempDir(), "ns.db")
	if err := os.WriteFile(file, []byte("SQLite format 3\x00rows"), 0o600); err != nil {
		t.Fatal(err)
	}
	c := &cobra.Command{RunE: rqliteImport}
	c.Flags().StringP("input", "i", "", "")
	c.Flags().Bool("yes", false, "")
	c.SetIn(strings.NewReader(in))
	c.SetOut(&bytes.Buffer{})
	if err := c.ParseFlags(append([]string{"-i", file}, flags...)); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestRQLiteImport_offersYes(t *testing.T) {
	if rqliteImportCmd.Flags().Lookup("yes") == nil {
		t.Fatal("orama namespace rqlite import has no --yes")
	}
}

func TestRQLiteImport_yesReplacesWithoutTypingTheName(t *testing.T) {
	imports := importGateway(t)
	c := importCommand(t, "", "--yes")

	if err := c.RunE(c, nil); err != nil {
		t.Fatalf("rqlite import --yes: %v", err)
	}
	if imports() != 1 {
		t.Fatalf("rqlite import --yes sent %d imports, want 1", imports())
	}
}

func TestRQLiteImport_withoutYesNeedsTheNamespaceName(t *testing.T) {
	imports := importGateway(t)

	unconfirmed := importCommand(t, "")
	if err := unconfirmed.RunE(unconfirmed, nil); err == nil || !strings.Contains(err.Error(), "aborted") {
		t.Fatalf("an import nobody confirmed returned %v, want aborted", err)
	}
	if imports() != 0 {
		t.Fatal("an unconfirmed import reached the gateway")
	}

	c := importCommand(t, "acme\n")
	if err := c.RunE(c, nil); err != nil {
		t.Fatalf("an import confirmed with the namespace name: %v", err)
	}
	if imports() != 1 {
		t.Fatalf("a confirmed import sent %d imports, want 1", imports())
	}
}
