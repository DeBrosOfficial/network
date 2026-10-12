package app

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"cosmossdk.io/log/v2"

	"github.com/DeBrosOfficial/network/chain/x/shielded/verify"
	"github.com/DeBrosOfficial/network/chain/x/shielded/verify/orchardproc"
)

// mempoolRequestTimeout is how long CheckTx waits for the mempool's verifier child. A valid bundle
// verifies in well under a second; CometBFT's local ABCI client serialises CheckTx with
// FinalizeBlock, so whatever a CheckTx waits is a delay of the next block, and a bundle that hangs
// the child must cost no more than this, not the block verifier's 30 s watchdog.
const mempoolRequestTimeout = 3 * time.Second

// warmBackoff is the least time between two attempts to start the mempool's child after one failed.
const warmBackoff = 10 * time.Second

// mempoolVerifier is the out-of-process verifier CheckTx, simulations and proposal checks use. It
// never starts the child on a request: starting builds the Halo 2 verifying key, seconds of work that
// would be done while CometBFT's ABCI lock is held. It starts the child in the background and answers
// verify.ErrVerifierStarting until the child is ready, and again after anything ends the child
// (orchardproc stops it after a fault or a panic, and it can exit while idle).
type mempoolVerifier struct {
	proc   *orchardproc.Verifier
	logger log.Logger
	now    func() time.Time

	ready atomic.Bool
	// configured is false on a node with no verifier binary (a localnet or a scripted devnet): then
	// the request goes straight to proc, which answers "not linked" without starting anything.
	configured bool

	mu       sync.Mutex
	warming  bool
	closed   bool
	failed   bool      // a start has failed before: repeats are logged below Error
	nextWarm time.Time // no start before this, after a failure
	attempts int       // starts attempted, for tests
}

var _ verify.Verifier = (*mempoolVerifier)(nil)

func newMempoolVerifier(proc *orchardproc.Verifier, logger log.Logger) *mempoolVerifier {
	return &mempoolVerifier{proc: proc, logger: logger, now: time.Now}
}

// ID is the process verifier's identity, so verify.Check counts it as the second verifier.
func (m *mempoolVerifier) ID() string { return m.proc.ID() }

// Ready reports whether a request would be asked of a running child (keeper.readier).
func (m *mempoolVerifier) Ready() bool { return !m.configured || m.ready.Load() }

// Close stops the child. It never waits for a start under way: the start closes the child it
// finished after Close was called.
func (m *mempoolVerifier) Close() error {
	m.mu.Lock()
	m.closed = true
	starting := m.warming
	m.mu.Unlock()
	m.ready.Store(false)
	if starting {
		return nil
	}
	return m.proc.Close()
}

// warm starts the child in the background. A call while a start is under way, after Close, or inside
// the back-off after a failed start does nothing.
func (m *mempoolVerifier) warm() {
	m.mu.Lock()
	if m.closed || m.warming || m.now().Before(m.nextWarm) {
		m.mu.Unlock()
		return
	}
	m.warming = true
	m.attempts++
	m.mu.Unlock()
	go func() {
		err := m.proc.Warm()
		m.mu.Lock()
		m.warming = false
		closed := m.closed
		if err != nil {
			m.nextWarm = m.now().Add(warmBackoff)
		}
		first := !m.failed
		if err != nil {
			m.failed = true
		}
		m.mu.Unlock()
		switch {
		case closed:
			_ = m.proc.Close() // started after the app closed: do not leave the child behind
		case err == nil:
			m.mu.Lock()
			m.failed = false
			m.mu.Unlock()
			m.ready.Store(true)
		case first:
			m.logger.Error("the mempool's shielded verifier did not start; CheckTx and proposal checks refuse shielded bundles until it does", "err", err)
		default:
			m.logger.Debug("the mempool's shielded verifier did not start again", "err", err)
		}
	}()
}

// Verify implements verify.Verifier.
func (m *mempoolVerifier) Verify(bundle, binding []byte) error {
	if !m.configured {
		return m.proc.Verify(bundle, binding)
	}
	if !m.ready.Load() {
		m.warm()
		return fmt.Errorf("%w: the mempool's verifier process is not ready", verify.ErrVerifierStarting)
	}
	err := m.proc.VerifyRunning(bundle, binding)
	switch {
	case errors.Is(err, orchardproc.ErrNotRunning):
		m.ready.Store(false) // the child exited while idle or is busy: start the next one off the request path
		m.warm()
		return fmt.Errorf("%w: the mempool's verifier process is not running", verify.ErrVerifierStarting)
	case errors.Is(err, verify.ErrVerifierFault), errors.Is(err, verify.ErrVerifierPanic):
		m.ready.Store(false) // orchardproc ended the child
		m.warm()
	}
	return err
}
