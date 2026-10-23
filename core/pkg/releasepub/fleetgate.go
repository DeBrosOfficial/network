package releasepub

import (
	"bufio"
	"context"
	"crypto/ed25519"
	_ "embed"
	"fmt"
	"strings"

	"github.com/Orama-Network/orama/core/pkg/fleetattest"
)

// runnerKeysFile is the list of runner public keys whose fleet report
// attestation a release accepts: one base64 Ed25519 key per line, '#' starts
// a comment. It is compiled in, so changing who may vouch for a release is a
// reviewed commit, not a flag.
//
//go:embed runner-keys.txt
var runnerKeysFile string

// RunnerKeysSource names runnerKeysFile in messages.
const RunnerKeysSource = "core/pkg/releasepub/runner-keys.txt"

// minCommitPrefix is the shortest commit abbreviation matched against a
// report's full SHA (git's own default abbreviation).
const minCommitPrefix = 7

// RunnerKeys are the runner keys compiled into this binary.
func RunnerKeys() ([]ed25519.PublicKey, error) {
	return parseRunnerKeys(runnerKeysFile)
}

func parseRunnerKeys(text string) ([]ed25519.PublicKey, error) {
	var keys []ed25519.PublicKey
	seen := map[string]bool{}
	sc := bufio.NewScanner(strings.NewReader(text))
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = strings.TrimSpace(line[:i])
		}
		if line == "" {
			continue
		}
		key, err := fleetattest.ParsePublicKey(line)
		if err != nil {
			return nil, fmt.Errorf("%s line %d: %w", RunnerKeysSource, n, err)
		}
		if seen[line] {
			return nil, fmt.Errorf("%s line %d: the key is listed twice", RunnerKeysSource, n)
		}
		seen[line] = true
		keys = append(keys, key)
	}
	return keys, sc.Err()
}

// FleetGate is what a release of a commit must show: the artifact directory
// of a fleet e2e run and the runner keys allowed to sign its report.
type FleetGate struct {
	ReportDir  string
	RunnerKeys []ed25519.PublicKey
}

// Check verifies that the fleet report in ReportDir is signed by an allowed
// runner, names report.json by digest, tested exactly commit (a full SHA or
// an abbreviation of at least 7 hex digits) with a clean working tree, and
// passed: every feature green and the coverage gate held, uncovered items
// only where waived.
func (g FleetGate) Check(ctx context.Context, commit string) (fleetattest.Statement, error) {
	if g.ReportDir == "" {
		return fleetattest.Statement{}, fmt.Errorf("no fleet e2e report: pass the artifact directory of a passing `make e2e-fleet` run of commit %s (--fleet-report)", commit)
	}
	if len(g.RunnerKeys) == 0 {
		return fleetattest.Statement{}, fmt.Errorf("no runner key is allowed to vouch for a release: add the runner's public key to %s", RunnerKeysSource)
	}
	s, err := fleetattest.Verify(ctx, g.ReportDir, g.RunnerKeys)
	if err != nil {
		return s, fmt.Errorf("fleet report %s: %w", g.ReportDir, err)
	}
	if err := matchCommit(s.Commit, commit); err != nil {
		return s, fmt.Errorf("fleet report %s (run %s): %w", g.ReportDir, s.RunID, err)
	}
	if s.Verdict != fleetattest.VerdictPass {
		return s, fmt.Errorf("fleet report %s (run %s) has verdict %s, not %s: a release needs every feature green and the coverage gate held", g.ReportDir, s.RunID, s.Verdict, fleetattest.VerdictPass)
	}
	return s, nil
}

// matchCommit holds the commit a run tested to the one being released.
func matchCommit(tested, released string) error {
	if strings.HasSuffix(tested, "-dirty") {
		return fmt.Errorf("the run tested uncommitted changes on top of %s, not a commit", strings.TrimSuffix(tested, "-dirty"))
	}
	if len(released) < minCommitPrefix || !isLowerHex(released) {
		return fmt.Errorf("the released commit %q is not a git SHA of at least %d hex digits", released, minCommitPrefix)
	}
	if !isLowerHex(tested) || !strings.HasPrefix(tested, released) {
		return fmt.Errorf("the run tested commit %s, not %s", tested, released)
	}
	return nil
}

func isLowerHex(s string) bool {
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return s != ""
}
