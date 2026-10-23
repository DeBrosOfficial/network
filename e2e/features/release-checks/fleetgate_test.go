//go:build e2e_fleet

package releasechecks

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Orama-Network/orama/core/pkg/fleetattest"
	"github.com/Orama-Network/orama/core/pkg/releasepub/pubtest"
	"github.com/Orama-Network/orama/core/pkg/releaseverify/releasetest"
	"github.com/Orama-Network/orama/e2e/harness"
)

// TestReleaseCut_standsOnARunnerSignedFleetReport: the shipped CLI's
// `orama maint release cut` refuses a nightly release without a fleet report,
// and with a passing report of the archive's commit signed by a key that is
// not compiled in; a dev/<branch> build plans without one
// (docs/whitepaper/technical-reference/vol1/29-build-signing-and-release.md,
// "The fleet gate"). Every call is a --dry-run against a throwaway root, so
// nothing is signed and the wallet is never asked.
func TestReleaseCut_standsOnARunnerSignedFleetReport(t *testing.T) {
	t.Parallel()
	cli := harness.CLI(t)
	repo := harness.WorkTemp(t)
	releasetest.NewRepo(t, time.Now()).WriteRoot(t, filepath.Join(repo, "root.json"))
	archive := pubtest.Archive(t, "0.3.1", "amd64", "fleet gate e2e")
	unlisted, _ := pubtest.FleetReport(t, pubtest.FullCommit, fleetattest.VerdictPass)
	cut := []string{"maint", "release", "cut", "--dry-run", "--dir", repo, "--archive", archive}

	for _, tc := range []struct {
		name  string
		extra []string
		want  string
	}{
		{"no report", []string{"--channel", "nightly"}, "no fleet e2e report"},
		{"a report signed by a key not compiled in", []string{"--channel", "nightly", "--fleet-report", unlisted}, "refusing to release " + pubtest.Commit + " on nightly"},
	} {
		res, err := cli.Run(t.Context(), append(append([]string{}, cut...), tc.extra...)...)
		if err != nil {
			t.Fatal(err)
		}
		if res.Exit == 0 || !strings.Contains(res.Stderr+res.Stdout, tc.want) {
			t.Errorf("%s: exit %d, want a refusal naming %q\nstdout: %s\nstderr: %s", tc.name, res.Exit, tc.want, res.Stdout, res.Stderr)
		}
	}

	out := cli.MustOK(t, append(append([]string{}, cut...), "--channel", "dev/e2e")...).Stdout
	if !strings.Contains(out, "Would cut dev/e2e 0.3.1") {
		t.Errorf("a dev build without a report did not plan:\n%s", out)
	}
}
