//go:build e2e_fleet

package docsclaims

import (
	"regexp"
	"strings"
	"testing"

	"github.com/DeBrosOfficial/network/e2e/features/internal/cliconf"
)

const (
	// wasmPolicyKeys defines AssumedBlockSeconds, the block interval the chain's
	// upload sunset is counted in, which is the interval oramad init writes as
	// timeout_commit (the Cosmos SDK sets 5s; oramad does not change it).
	wasmPolicyKeys = "chain/x/wasmpolicy/types/keys.go"
	performanceDoc = "website/src/docs/contributor/performance.mdx"
)

// sixSecondBlocks matches a line that counts blocks at 6 seconds ("6 s blocks",
// "6-second blocks", "14 days at 6 s"), not one that merely names a 6 s period
// such as the provider's polling interval.
var sixSecondBlocks = regexp.MustCompile(`(?i)(\b6[ -](s|sec|seconds?)[ -]blocks?\b|\bat (a )?6[ -]?(s|sec|seconds?)\b)`)

// TestBlockTime_docsStateTheIntervalTheCodeSets: the chain's blocks come every
// AssumedBlockSeconds (5 s, timeout_commit of config.toml), and the chain and
// performance pages say so. A line that counts blocks at 6 s (some defaults are
// written that way in the code) must also give the 5 s the chain runs at, so no
// page leaves a reader with two block times.
func TestBlockTime_docsStateTheIntervalTheCodeSets(t *testing.T) {
	t.Parallel()
	seconds := codeConst(t, wasmPolicyKeys, "AssumedBlockSeconds")
	want := regexp.MustCompile(`\b` + seconds + `[ -](s|second|seconds)\b`)
	for _, doc := range []string{chainDoc, performanceDoc} {
		text := cliconf.ReadRepoFile(t, doc)
		if !strings.Contains(text, "timeout_commit") || !want.MatchString(text) {
			t.Errorf("%s does not state the block interval the code sets: timeout_commit of %s s (%s)", doc, seconds, wasmPolicyKeys)
		}
	}
	for _, doc := range docs(t) {
		for _, l := range grep(t, doc, sixSecondBlocks) {
			if !want.MatchString(l.Text) {
				t.Errorf("%s\n  counts blocks at 6 s without the %s s the chain runs at", l, seconds)
			}
		}
	}
}
