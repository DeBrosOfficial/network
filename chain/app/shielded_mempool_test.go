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

// scriptVerifier is a verifier binary that waits for a gate file to exist, says it is ready, and
// exits at its first request, as a killed child does. The gate makes "not ready yet" a state a test
// holds for as long as it wants instead of a race against the start.
func scriptVerifier(t *testing.T, gate string) string {
	t.Helper()
	script := "#!/bin/sh\n" +
		"while [ ! -e '" + gate + "' ]; do sleep 0.02; done\n" +
		"printf '\\000\\000\\000\\011\\377\\377\\377\\377\\377\\377\\377\\377\\000'\n" +
		"head -c 1 >/dev/null\n" +
		"exit 1\n"
	path := filepath.Join(t.TempDir(), "orama-orchard-verifier")
	require.NoError(t, os.WriteFile(path, []byte(script), 0o755))
	return path
}

func newScriptMempoolVerifier(t *testing.T, gate string) *mempoolVerifier {
	t.Helper()
	path := scriptVerifier(t, gate)
	pin, err := FileSHA256(path)
	require.NoError(t, err)
	proc, err := orchardproc.New(orchardproc.Config{Path: path, ChainID: "orama-localnet-1", SHA256: pin, RequestTimeout: mempoolRequestTimeout})
	require.NoError(t, err)
	m := newMempoolVerifier(proc, log.NewNopLogger())
	m.configured = true
	t.Cleanup(func() { _ = m.Close() })
	return m
}

func release(t *testing.T, gate string) { require.NoError(t, os.WriteFile(gate, nil, 0o600)) }

// bundleForTest has the canonical framing up to the proof, which is all the sighash reads.
func bundleForTest() []byte {
	b := make([]byte, 1+bundle.ActionLen+bundle.HeaderLen+16)
	b[0] = 1
	return b
}

// CheckTx never starts the child itself, because starting builds the verifying key, seconds of work
// done while CometBFT's ABCI lock is held. Until the child is ready the mempool says it is starting,
// at once, and the child starts in the background. A fault ends the child, and the next request is
// answered the same way while a new one starts.
func TestMempoolVerifier_startsInTheBackgroundAndNeverOnTheRequest(t *testing.T) {
	gate := filepath.Join(t.TempDir(), "gate")
	m := newScriptMempoolVerifier(t, gate)

	require.False(t, m.Ready())
	start := time.Now()
	err := m.Verify(bundleForTest(), nil)
	require.ErrorIs(t, err, verify.ErrVerifierStarting)
	require.NotErrorIs(t, err, verify.ErrVerifierFault, "starting is not a fault")
	require.Less(t, time.Since(start), time.Second, "the request must not wait for the child to start")
	require.False(t, m.Ready(), "the child cannot be ready before its gate opens")

	release(t, gate)
	require.Eventually(t, m.Ready, 10*time.Second, 5*time.Millisecond, "the background start makes the child ready")

	err = m.Verify(bundleForTest(), nil) // the child exits at this request: a fault
	require.ErrorIs(t, err, verify.ErrVerifierFault)
	require.False(t, m.Ready(), "a fault ends the child, so the verifier is not ready")
	again := time.Now()
	require.ErrorIs(t, m.Verify(bundleForTest(), nil), verify.ErrVerifierStarting)
	require.Less(t, time.Since(again), time.Second)
	require.Eventually(t, m.Ready, 10*time.Second, 5*time.Millisecond, "and a new child starts off the request path")
}

// A child that exits while idle is not started again by the request that finds it gone: the request
// is told the verifier is starting, and the start happens in the background.
func TestMempoolVerifier_aChildThatExitedWhileIdleIsRestartedOffTheRequestPath(t *testing.T) {
	gate := filepath.Join(t.TempDir(), "gate")
	release(t, gate)
	m := newScriptMempoolVerifier(t, gate)
	m.warm()
	require.Eventually(t, m.Ready, 10*time.Second, 5*time.Millisecond)

	require.NoError(t, m.proc.Close()) // the child is gone while the wrapper still believes it is ready
	require.True(t, m.ready.Load())
	start := time.Now()
	err := m.Verify(bundleForTest(), nil)
	require.ErrorIs(t, err, verify.ErrVerifierStarting)
	require.Less(t, time.Since(start), time.Second, "the request did not wait for a new child")
	require.Eventually(t, m.Ready, 10*time.Second, 5*time.Millisecond, "the background start brought a child up")
}

// Close during a start does not wait for it, and the child that start brings up is closed.
func TestMempoolVerifier_closeDuringAStartDoesNotWaitAndLeavesNoChild(t *testing.T) {
	gate := filepath.Join(t.TempDir(), "gate")
	m := newScriptMempoolVerifier(t, gate)
	m.warm()
	require.Eventually(t, func() bool { m.mu.Lock(); defer m.mu.Unlock(); return m.warming }, 5*time.Second, time.Millisecond)

	start := time.Now()
	require.NoError(t, m.Close())
	require.Less(t, time.Since(start), time.Second, "Close waited for the start")

	m.warm() // after Close nothing starts
	m.mu.Lock()
	require.Equal(t, 1, m.attempts)
	m.mu.Unlock()

	release(t, gate) // the start finishes, sees the app is closed and closes its child
	require.Eventually(t, func() bool { m.mu.Lock(); defer m.mu.Unlock(); return !m.warming }, 10*time.Second, 5*time.Millisecond)
	require.False(t, m.ready.Load(), "a child that came up after Close is not used")
}

// A start that fails is not tried again for the back-off, however many requests arrive.
func TestMempoolVerifier_aFailedStartIsNotRetriedInsideTheBackoff(t *testing.T) {
	proc, err := orchardproc.New(orchardproc.Config{Path: filepath.Join(t.TempDir(), "missing"), ChainID: "orama-localnet-1", SHA256: "00"})
	require.NoError(t, err)
	m := newMempoolVerifier(proc, log.NewNopLogger())
	m.configured = true
	now := time.Unix(1_700_000_000, 0)
	m.now = func() time.Time { return now }
	settle := func() {
		require.Eventually(t, func() bool { m.mu.Lock(); defer m.mu.Unlock(); return !m.warming }, 5*time.Second, time.Millisecond)
	}

	for i := 0; i < 20; i++ {
		_ = m.Verify(bundleForTest(), nil)
		settle()
	}
	m.mu.Lock()
	require.Equal(t, 1, m.attempts, "20 requests, one start")
	m.mu.Unlock()

	now = now.Add(warmBackoff + time.Second)
	_ = m.Verify(bundleForTest(), nil)
	settle()
	m.mu.Lock()
	require.Equal(t, 2, m.attempts, "after the back-off the start is tried again")
	m.mu.Unlock()
}

// The mempool's wait is bounded well under the block verifier's watchdog.
func TestMempoolRequestTimeout_isShort(t *testing.T) {
	require.LessOrEqual(t, mempoolRequestTimeout, 5*time.Second)
	require.Less(t, mempoolRequestTimeout, orchardproc.DefaultRequestTimeout)
}

// Close ends the mempool verifier's child (OramaApp.Close calls it for every verifier that has one).
func TestMempoolVerifier_closeEndsTheChild(t *testing.T) {
	gate := filepath.Join(t.TempDir(), "gate")
	release(t, gate)
	m := newScriptMempoolVerifier(t, gate)
	m.warm()
	require.Eventually(t, m.Ready, 10*time.Second, 5*time.Millisecond)
	require.NoError(t, m.Close())
	require.False(t, m.Ready())
}
