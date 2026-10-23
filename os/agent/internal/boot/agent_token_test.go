package boot

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withAgentTokenPath points agentTokenPath at a file under a fresh directory
// for the test.
func withAgentTokenPath(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "secrets", "agent-token")
	old := agentTokenPath
	agentTokenPath = path
	t.Cleanup(func() { agentTokenPath = old })
	return path
}

func TestAgentToken_storedOwnerOnlyAndReadBack(t *testing.T) {
	path := withAgentTokenPath(t)
	a := &Agent{}
	if err := a.storeAgentToken("  tok-123\n"); err != nil {
		t.Fatal(err)
	}
	if a.agentToken != "tok-123" {
		t.Errorf("in memory %q, want the trimmed token", a.agentToken)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("token file mode %v, want 0600", info.Mode().Perm())
	}
	dir, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if dir.Mode().Perm() != 0o700 {
		t.Errorf("secrets directory mode %v, want 0700", dir.Mode().Perm())
	}

	// What a reboot does: a fresh agent reads what enrollment wrote.
	rebooted := &Agent{}
	if err := rebooted.loadAgentToken(); err != nil {
		t.Fatal(err)
	}
	if rebooted.agentToken != "tok-123" {
		t.Errorf("after reboot %q, want tok-123", rebooted.agentToken)
	}
}

func TestAgentToken_anEmptyTokenIsNeverStored(t *testing.T) {
	path := withAgentTokenPath(t)
	for _, tok := range []string{"", "   ", "\n\t"} {
		if err := (&Agent{}).storeAgentToken(tok); err == nil {
			t.Errorf("stored %q", tok)
		}
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("a refused token left a file behind: %v", err)
	}
}

// A node enrolled before tokens existed has no file, and one whose file is
// blank has no credential: both are errors, so no receiver starts with an
// empty token.
func TestAgentToken_noUsableFileIsAnError(t *testing.T) {
	path := withAgentTokenPath(t)
	a := &Agent{}
	if err := a.loadAgentToken(); err == nil || !strings.Contains(err.Error(), path) {
		t.Errorf("missing file: %v, want an error naming %s", err, path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(" \n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := a.loadAgentToken(); err == nil {
		t.Error("a blank token file loaded")
	}
	if a.agentToken != "" {
		t.Errorf("token %q set from a failed load", a.agentToken)
	}
}
