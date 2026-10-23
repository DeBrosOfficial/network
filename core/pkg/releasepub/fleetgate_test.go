package releasepub

import (
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Orama-Network/orama/core/pkg/fleetattest"
	"github.com/Orama-Network/orama/core/pkg/releasepub/pubtest"
)

// gate is a FleetGate over a report of commit with verdict, trusting its signer.
func gate(t *testing.T, commit, verdict string) FleetGate {
	t.Helper()
	dir, runner := pubtest.FleetReport(t, commit, verdict)
	return FleetGate{ReportDir: dir, RunnerKeys: []ed25519.PublicKey{runner}}
}

func otherKey(t *testing.T) ed25519.PublicKey {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return pub
}

func TestFleetGate_acceptsAPassingSignedRunOfTheCommit(t *testing.T) {
	for _, commit := range []string{pubtest.Commit, pubtest.FullCommit} {
		s, err := gate(t, pubtest.FullCommit, fleetattest.VerdictPass).Check(t.Context(), commit)
		if err != nil || s.RunID != pubtest.FleetRun {
			t.Fatalf("commit %s: %+v, %v", commit, s, err)
		}
	}
}

func TestFleetGate_refusals(t *testing.T) {
	signedByOther := gate(t, pubtest.FullCommit, fleetattest.VerdictPass)
	signedByOther.RunnerKeys = []ed25519.PublicKey{otherKey(t)}
	edited := gate(t, pubtest.FullCommit, fleetattest.VerdictPass)
	if err := os.WriteFile(filepath.Join(edited.ReportDir, fleetattest.ReportFile), []byte(`{"verdict":"PASS"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	noKeys := gate(t, pubtest.FullCommit, fleetattest.VerdictPass)
	noKeys.RunnerKeys = nil
	for _, tc := range []struct {
		name   string
		gate   FleetGate
		commit string
		want   string
	}{
		{"signed by a key not on the list", signedByOther, pubtest.Commit, "not signed by an allowed runner key"},
		{"report edited after signing", edited, pubtest.Commit, "changed after the runner signed it"},
		{"no runner key allowed", noKeys, pubtest.Commit, RunnerKeysSource},
		{"no report", FleetGate{RunnerKeys: []ed25519.PublicKey{otherKey(t)}}, pubtest.Commit, "no fleet e2e report"},
		{"another commit", gate(t, strings.Repeat("f", 40), fleetattest.VerdictPass), pubtest.Commit, "the run tested commit ffff"},
		{"a dirty tree", gate(t, pubtest.FullCommit+"-dirty", fleetattest.VerdictPass), pubtest.Commit, "uncommitted changes"},
		{"a failed run", gate(t, pubtest.FullCommit, "FAIL"), pubtest.Commit, "verdict FAIL"},
		{"an incomplete run", gate(t, pubtest.FullCommit, "INCOMPLETE"), pubtest.Commit, "verdict INCOMPLETE"},
		{"a too-short commit", gate(t, pubtest.FullCommit, fleetattest.VerdictPass), "abc", "at least 7 hex digits"},
		{"an empty archive commit", gate(t, pubtest.FullCommit, fleetattest.VerdictPass), "", "at least 7 hex digits"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := tc.gate.Check(t.Context(), tc.commit); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestCut_nightlyAndMainNeedAPassingFleetReport(t *testing.T) {
	agent := newFakeAgent(t)
	for _, channel := range []string{"nightly", "main"} {
		repo := newRepo(t, agent)
		before := agent.approvals()
		p := cutParams(t, repo, agent, mustChannel(t, channel), archive(t, "0.3.1", "amd64", "x"))
		p.Fleet = gate(t, pubtest.FullCommit, "FAIL")
		if _, err := Cut(t.Context(), p); err == nil || !strings.Contains(err.Error(), "refusing to release "+pubtest.Commit+" on "+channel) {
			t.Fatalf("%s with a failed run: %v", channel, err)
		}
		p.Fleet = FleetGate{}
		p.DryRun = true
		if _, err := Cut(t.Context(), p); err == nil || !strings.Contains(err.Error(), "no fleet e2e report") {
			t.Fatalf("%s dry run without a report: %v", channel, err)
		}
		if agent.approvals() != before {
			t.Fatalf("a refused cut asked for %d approvals", agent.approvals()-before)
		}
	}
}

func TestCut_namesTheFleetRunItStandsOn(t *testing.T) {
	agent := newFakeAgent(t)
	repo := newRepo(t, agent)
	plan, err := Cut(t.Context(), cutParams(t, repo, agent, mustChannel(t, "nightly"), archive(t, "0.3.1", "amd64", "x")))
	if err != nil {
		t.Fatal(err)
	}
	if plan.Commit != pubtest.Commit || plan.FleetRun != pubtest.FleetRun {
		t.Fatalf("plan commit %q run %q", plan.Commit, plan.FleetRun)
	}
}

func TestCut_devBuildWithoutAReportNeedsNone(t *testing.T) {
	agent := newFakeAgent(t)
	repo := newRepo(t, agent)
	p := cutParams(t, repo, agent, mustChannel(t, "dev/feat"), archive(t, "0.3.1", "amd64", "x"))
	p.Fleet = FleetGate{}
	plan, err := Cut(t.Context(), p)
	if err != nil || plan.FleetRun != "" {
		t.Fatalf("plan %+v, %v", plan, err)
	}
	p.Repo = newRepo(t, agent)
	p.Fleet = gate(t, pubtest.FullCommit, "FAIL")
	if _, err := Cut(t.Context(), p); err == nil || !strings.Contains(err.Error(), "verdict FAIL") {
		t.Fatalf("a dev build naming a failed run: %v", err)
	}
}

func TestCut_refusesArchivesOfTwoCommits(t *testing.T) {
	agent := newFakeAgent(t)
	repo := newRepo(t, agent)
	arm := pubtest.ArchiveWith(t, "0.3.1", "arm64", "y", pubtest.Options{Commit: "def4567"})
	p := cutParams(t, repo, agent, mustChannel(t, "nightly"), archive(t, "0.3.1", "amd64", "x"), arm)
	if _, err := Cut(t.Context(), p); err == nil || !strings.Contains(err.Error(), "commits abc1234 and def4567") {
		t.Fatalf("err = %v", err)
	}
}

func TestRunnerKeys_theCompiledListParses(t *testing.T) {
	if _, err := RunnerKeys(); err != nil {
		t.Fatal(err)
	}
}

func TestParseRunnerKeys(t *testing.T) {
	k := fleetattest.FormatPublicKey(otherKey(t))
	keys, err := parseRunnerKeys("# comment\n\n" + k + "  # the runner\n")
	if err != nil || len(keys) != 1 {
		t.Fatalf("keys %d, %v", len(keys), err)
	}
	if _, err := parseRunnerKeys(k + "\n" + k + "\n"); err == nil || !strings.Contains(err.Error(), "line 2: the key is listed twice") {
		t.Fatalf("duplicate: %v", err)
	}
	if _, err := parseRunnerKeys("not-a-key\n"); err == nil || !strings.Contains(err.Error(), "line 1") {
		t.Fatalf("garbage: %v", err)
	}
}
