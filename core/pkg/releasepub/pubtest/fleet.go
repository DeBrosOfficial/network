package pubtest

import (
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Orama-Network/orama/core/pkg/fleetattest"
)

// FullCommit is the full SHA the archives' Commit abbreviates.
const FullCommit = Commit + "def567890abcdef1234567890abcdef12345"

// FleetRun is the run id of a test fleet report.
const FleetRun = "abcd1234"

// FleetReport writes the artifact directory of a fleet run of commit with
// verdict, signed by a new runner key, and returns it with the key's public
// half.
func FleetReport(t testing.TB, commit, verdict string) (string, ed25519.PublicKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return FleetReportSignedBy(t, priv, commit, verdict), pub
}

// FleetReportSignedBy is FleetReport signed by runner.
func FleetReportSignedBy(t testing.TB, runner ed25519.PrivateKey, commit, verdict string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, fleetattest.ReportFile), []byte(`{"run_id":"`+FleetRun+`","verdict":"`+verdict+`"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := fleetattest.Statement{RunID: FleetRun, Commit: commit, Verdict: verdict, SignedAt: time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)}
	if err := fleetattest.Sign(t.Context(), dir, runner, s); err != nil {
		t.Fatal(err)
	}
	return dir
}
