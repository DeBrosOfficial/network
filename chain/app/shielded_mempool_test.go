package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"cosmossdk.io/log/v2"

	"github.com/DeBrosOfficial/network/chain/x/shielded/bundle"
	"github.com/DeBrosOfficial/network/chain/x/shielded/verify"
	"github.com/DeBrosOfficial/network/chain/x/shielded/verify/orchardproc"
)

// A verifier that says it is ready and exits at its first request, as a killed child does.
const readyThenDies = "#!/bin/sh\n" +
	"printf '\\000\\000\\000\\011\\377\\377\\377\\377\\377\\377\\377\\377\\000'\n" +
	"head -c 1 >/dev/null\n" +
	"exit 1\n"

func newScriptMempoolVerifier(t *testing.T) *mempoolVerifier {
	t.Helper()
	path := filepath.Join(t.TempDir(), "orama-orchard-verifier")
	require.NoError(t, os.WriteFile(path, []byte(readyThenDies), 0o755))
	pin, err := FileSHA256(path)
	require.NoError(t, err)
	proc, err := orchardproc.New(orchardproc.Config{Path: path, ChainID: "orama-localnet-1", SHA256: pin, RequestTimeout: mempoolRequestTimeout})
	require.NoError(t, err)
	m := newMempoolVerifier(proc, log.NewNopLogger())
	m.configured = true
	t.Cleanup(func() { _ = m.Close() })
	return m
}

// bundleForTest has the canonical framing up to the proof, which is all the sighash reads.
func bundleForTest() []byte {
	b := make([]byte, 1+bundle.ActionLen+bundle.HeaderLen+16)
	b[0] = 1
	return b
}

// CheckTx never starts the child itself, because starting builds the verifying key, seconds of work
// done while CometBFT's ABCI lock is held: until the child is ready the mempool refuses with a local
// fault, at once, and the child starts in the background. A fault ends the child, and the next
// request is refused the same way while a new one starts.
func TestMempoolVerifier_startsInTheBackgroundAndNeverOnTheRequest(t *testing.T) {
	m := newScriptMempoolVerifier(t)
	start := time.Now()
	err := m.Verify(bundleForTest(), nil)
	require.ErrorIs(t, err, verify.ErrVerifierFault)
	require.ErrorContains(t, err, "starting")
	require.Less(t, time.Since(start), time.Second, "the request must not wait for the child to start")

	require.Eventually(t, m.ready.Load, 10*time.Second, 5*time.Millisecond, "the background start makes the child ready")

	err = m.Verify(bundleForTest(), nil) // the child exits at this request: a fault
	require.ErrorIs(t, err, verify.ErrVerifierFault)
	require.False(t, m.ready.Load(), "a fault ends the child, so the verifier is not ready")
	again := time.Now()
	err = m.Verify(bundleForTest(), nil)
	require.ErrorContains(t, err, "starting")
	require.Less(t, time.Since(again), time.Second)
	require.Eventually(t, m.ready.Load, 10*time.Second, 5*time.Millisecond, "and a new child starts off the request path")
}

// The mempool's wait is bounded well under the block verifier's watchdog.
func TestMempoolRequestTimeout_isShort(t *testing.T) {
	require.LessOrEqual(t, mempoolRequestTimeout, 5*time.Second)
	require.Less(t, mempoolRequestTimeout, orchardproc.DefaultRequestTimeout)
}

// Close ends the mempool verifier's child (OramaApp.Close calls it for every verifier that has one).
func TestMempoolVerifier_closeEndsTheChild(t *testing.T) {
	m := newScriptMempoolVerifier(t)
	m.warm()
	require.Eventually(t, m.ready.Load, 10*time.Second, 5*time.Millisecond)
	require.NoError(t, m.Close())
}
