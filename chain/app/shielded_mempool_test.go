package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"cosmossdk.io/log/v2"

	"github.com/Orama-Network/orama/chain/x/shielded/bundle"
	"github.com/Orama-Network/orama/chain/x/shielded/testutil"
	"github.com/Orama-Network/orama/chain/x/shielded/verify"
	"github.com/Orama-Network/orama/chain/x/shielded/verify/orchardproc"
)

// heldVerifier is a verifier binary that is ready at once and holds the first request it is given
// until the gate file exists: it creates the inflight file once it has the request, and exits at the
// gate, as a hung child does when it is killed. The test, not a timer, decides how long the request
// is in flight.
func heldVerifier(t *testing.T, gate, inflight string) string {
	t.Helper()
	script := "#!/bin/sh\n" +
		"printf '\\000\\000\\000\\011\\377\\377\\377\\377\\377\\377\\377\\377\\000'\n" +
		"head -c 1 >/dev/null\n" +
		": > '" + inflight + "'\n" +
		"while [ ! -e '" + gate + "' ]; do sleep 0.02; done\n" +
		"exit 1\n"
	path := filepath.Join(t.TempDir(), "orama-orchard-verifier")
	require.NoError(t, os.WriteFile(path, []byte(script), 0o755))
	return path
}

// startSettles is how long a test waits for a background start to end. The start ends, ready or
// failed, within the child's StartTimeout (none of these tests sets one, so it is the default); the
// margin is for the goroutine that records the outcome. Waiting for the start's own end, rather than
// for a guess at how long a start takes, keeps a loaded machine from failing a test whose child
// was merely slow to start (bug-931).
const startSettles = orchardproc.DefaultStartTimeout + 5*time.Second

// warming reports whether a background start is under way.
func warming(m *mempoolVerifier) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.warming
}

// attempts is the number of starts attempted. It is read under the lock and returned, so an
// assertion on it never fails while the lock is held: that left the lock taken and the test's
// cleanup (Close) waiting for it forever.
func attempts(m *mempoolVerifier) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.attempts
}

// awaitStart waits for the background start under way to end and requires that it brought a child
// up. A start that ended without one failed (m.failed), which is a different fault from a slow start.
func awaitStart(t *testing.T, m *mempoolVerifier, msgAndArgs ...any) {
	t.Helper()
	require.Eventually(t, func() bool { return !warming(m) }, startSettles, 5*time.Millisecond, "the background start did not end within its StartTimeout")
	m.mu.Lock()
	failed := m.failed
	m.mu.Unlock()
	require.False(t, failed, "the background start failed")
	require.True(t, m.Ready(), msgAndArgs...)
}

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
	awaitStart(t, m, "the background start makes the child ready")

	err = m.Verify(bundleForTest(), nil) // the child exits at this request: a fault
	require.ErrorIs(t, err, verify.ErrVerifierFault)
	require.False(t, m.Ready(), "a fault ends the child, so the verifier is not ready")
	again := time.Now()
	require.ErrorIs(t, m.Verify(bundleForTest(), nil), verify.ErrVerifierStarting)
	require.Less(t, time.Since(again), time.Second)
	awaitStart(t, m, "and a new child starts off the request path")
}

// A child that exits while idle is not started again by the request that finds it gone: the request
// is told the verifier is starting, and the start happens in the background.
func TestMempoolVerifier_aChildThatExitedWhileIdleIsRestartedOffTheRequestPath(t *testing.T) {
	gate := filepath.Join(t.TempDir(), "gate")
	release(t, gate)
	m := newScriptMempoolVerifier(t, gate)
	m.warm()
	awaitStart(t, m)

	require.NoError(t, m.proc.Close()) // the child is gone while the wrapper still believes it is ready
	require.True(t, m.ready.Load())
	start := time.Now()
	err := m.Verify(bundleForTest(), nil)
	require.ErrorIs(t, err, verify.ErrVerifierStarting)
	require.Less(t, time.Since(start), time.Second, "the request did not wait for a new child")
	awaitStart(t, m, "the background start brought a child up")
}

// Close during a start does not wait for it, and the child that start brings up is closed.
func TestMempoolVerifier_closeDuringAStartDoesNotWaitAndLeavesNoChild(t *testing.T) {
	gate := filepath.Join(t.TempDir(), "gate")
	m := newScriptMempoolVerifier(t, gate)
	m.warm()
	require.True(t, warming(m), "warm marks the start under way before it returns")

	start := time.Now()
	require.NoError(t, m.Close())
	require.Less(t, time.Since(start), time.Second, "Close waited for the start")

	m.warm() // after Close nothing starts
	require.Equal(t, 1, attempts(m))

	release(t, gate) // the start finishes, sees the app is closed and closes its child
	require.Eventually(t, func() bool { return !warming(m) }, startSettles, 5*time.Millisecond)
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
		require.Eventually(t, func() bool { return !warming(m) }, startSettles, time.Millisecond)
	}

	for i := 0; i < 20; i++ {
		_ = m.Verify(bundleForTest(), nil)
		settle()
	}
	require.Equal(t, 1, attempts(m), "20 requests, one start")

	now = now.Add(warmBackoff + time.Second)
	_ = m.Verify(bundleForTest(), nil)
	settle()
	require.Equal(t, 2, attempts(m), "after the back-off the start is tried again")
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
	awaitStart(t, m)
	require.NoError(t, m.Close())
	require.False(t, m.Ready())
}

// A request in flight (a gRPC simulation runs outside the ABCI lock) makes a second request busy, and
// busy is not gone: the verifier stays ready and no second child is started. The child holds the first
// request until the test opens its gate, so the second request is asked while the first is provably
// in flight; it used to be a 200 ms sleep racing a 2 s request timeout, which a loaded machine lost:
// the first request timed out, ended the child and started another before the asserts ran (bug-931).
func TestMempoolVerifier_aBusyChildStaysReady(t *testing.T) {
	dir := t.TempDir()
	gate, inflight := filepath.Join(dir, "gate"), filepath.Join(dir, "inflight")
	path := heldVerifier(t, gate, inflight)
	pin, err := FileSHA256(path)
	require.NoError(t, err)
	proc, err := orchardproc.New(orchardproc.Config{Path: path, ChainID: "orama-localnet-1", SHA256: pin, BusyWait: 20 * time.Millisecond})
	require.NoError(t, err)
	m := newMempoolVerifier(proc, log.NewNopLogger())
	m.configured = true
	t.Cleanup(func() { _ = m.Close() })
	m.warm()
	awaitStart(t, m)

	first := make(chan error, 1)
	go func() { first <- m.Verify(bundleForTest(), nil) }() // holds the child until the gate opens
	require.Eventually(t, func() bool { _, err := os.Stat(inflight); return err == nil }, startSettles, 5*time.Millisecond, "the child never received the first request")
	err = m.Verify(bundleForTest(), nil)
	require.ErrorIs(t, err, verify.ErrVerifierStarting)
	require.True(t, m.ready.Load(), "busy must not clear ready")
	require.Equal(t, 1, attempts(m), "no second start for a child that is alive")

	release(t, gate)                                     // the held child exits, as a killed one does
	require.ErrorIs(t, <-first, verify.ErrVerifierFault) // and the request it held is a fault
}

// A first start that fails is tried again after the back-off, and the keeper's readiness gate is what
// asks: Ready itself starts the child, so the gate in front of Verify cannot keep it down. Through
// keeper.CheckProposalProof.
func TestMempoolVerifier_aFailedFirstStartIsRetriedThroughTheKeepersGate(t *testing.T) {
	gate := filepath.Join(t.TempDir(), "gate")
	release(t, gate)
	script := scriptVerifier(t, gate)
	pin, err := FileSHA256(script)
	require.NoError(t, err)
	raw, err := os.ReadFile(script)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "orama-orchard-verifier") // not there yet: the first start fails
	proc, err := orchardproc.New(orchardproc.Config{Path: path, ChainID: "orama-localnet-1", SHA256: pin})
	require.NoError(t, err)
	m := newMempoolVerifier(proc, log.NewNopLogger())
	m.configured = true
	now := time.Unix(1_700_000_000, 0)
	m.now = func() time.Time { return now }
	t.Cleanup(func() { _ = m.Close() })

	e := testutil.NewEnv(t, nil)
	k := e.KeeperWithMempool([]verify.Verifier{e.V1, e.V2}, []verify.Verifier{m, e.V1})
	settled := func() bool { return !warming(m) }
	b := bundleForTest()

	require.ErrorIs(t, k.CheckProposalProof(b, nil, time.Second), verify.ErrVerifierStarting)
	require.Eventually(t, settled, startSettles, time.Millisecond)
	require.ErrorIs(t, k.CheckProposalProof(b, nil, time.Second), verify.ErrVerifierStarting, "inside the back-off")
	require.Equal(t, 1, attempts(m))

	require.NoError(t, os.WriteFile(path, raw, 0o755)) // the binary appears
	now = now.Add(warmBackoff + time.Second)
	require.ErrorIs(t, k.CheckProposalProof(b, nil, time.Second), verify.ErrVerifierStarting, "the call that finds the back-off over starts the child")
	awaitStart(t, m, "and a later call finds it ready")
}
