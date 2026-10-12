package app

import (
	"errors"
	"fmt"
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

// mempoolVerifier is the out-of-process verifier CheckTx and simulations use. It never starts the
// child itself on a request: starting builds the Halo 2 verifying key, seconds of work that CheckTx
// would do while holding the ABCI lock. It starts the child in the background and refuses with a
// local fault until the child has answered ready, and again after any fault or panic, which end the
// child (orchardproc stops it).
type mempoolVerifier struct {
	proc    *orchardproc.Verifier
	logger  log.Logger
	ready   atomic.Bool
	warming atomic.Bool
	// configured is false on a node with no verifier binary (a localnet or a scripted devnet): then
	// the request goes straight to proc, which answers "not linked" without starting anything.
	configured bool
}

var _ verify.Verifier = (*mempoolVerifier)(nil)

func newMempoolVerifier(proc *orchardproc.Verifier, logger log.Logger) *mempoolVerifier {
	return &mempoolVerifier{proc: proc, logger: logger}
}

// ID is the process verifier's identity, so verify.Check counts it as the second verifier.
func (m *mempoolVerifier) ID() string { return m.proc.ID() }

// Close stops the child.
func (m *mempoolVerifier) Close() error { return m.proc.Close() }

// warm starts the child in the background. A second call while one is under way does nothing.
func (m *mempoolVerifier) warm() {
	if !m.warming.CompareAndSwap(false, true) {
		return
	}
	go func() {
		defer m.warming.Store(false)
		if err := m.proc.Warm(); err != nil {
			m.logger.Error("the mempool's shielded verifier did not start; CheckTx refuses shielded bundles until it does", "err", err)
			return
		}
		m.ready.Store(true)
	}()
}

// Verify implements verify.Verifier.
func (m *mempoolVerifier) Verify(bundle, binding []byte) error {
	if !m.configured {
		return m.proc.Verify(bundle, binding)
	}
	if !m.ready.Load() {
		m.warm()
		return fmt.Errorf("%w: the mempool's verifier is starting", verify.ErrVerifierFault)
	}
	err := m.proc.Verify(bundle, binding)
	if errors.Is(err, verify.ErrVerifierFault) || errors.Is(err, verify.ErrVerifierPanic) {
		m.ready.Store(false) // orchardproc ended the child; start the next one off the request path
		m.warm()
	}
	return err
}
