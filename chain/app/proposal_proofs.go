package app

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	shieldedante "github.com/DeBrosOfficial/network/chain/x/shielded/ante"
	orchardverify "github.com/DeBrosOfficial/network/chain/x/shielded/verify/orchard"
)

// The proof check of a block proposal. A shielded bundle costs every validator a Halo 2 verification,
// and a bundle that kills or hangs the verifier would stop every validator that executes it
// (shieldedkeeper.VerifierHalt). So a proposal is checked before it is voted for: every shielded
// bundle in it goes through the mempool's verifiers, and a proposal with a bundle that is rejected,
// that faults the verifier or that cannot be checked (the verifier is starting) gets a nil vote and
// the proposer rotates. Nothing here ever halts the node: a halt is for FinalizeBlock, where the
// block is decided. PrepareProposal leaves such bundles out of the block it builds, and the
// inclusion admitter does not admit them, so an honest proposer never builds a block its peers
// refuse for this reason. Honest blocks are verified twice, once here and once in FinalizeBlock.
//
// A rejection is a deterministic verdict on the bundle, so every honest validator refuses the same
// proposals for it. A fault is local to a validator: it votes nil alone, which costs nothing but
// its vote.

// proofsOK reports whether every shielded bundle of tx passes the mempool's verifiers. A tx with no
// shielded bundle passes.
func (app *OramaApp) proofsOK(tx sdk.Tx) bool {
	if !app.canCheckProofs() {
		return true
	}
	checks, err := shieldedante.ProofChecks(tx)
	if err != nil {
		return false
	}
	for _, c := range checks {
		if err := app.ShieldedKeeper.CheckProposalProof(c.Bundle, c.Binding); err != nil {
			app.Logger().Debug("a shielded bundle in a proposal did not pass the proof check", "module", "x/shielded", "err", err)
			return false
		}
	}
	return true
}

// canCheckProofs reports whether this node has verifiers to check a proposal with: the library is
// linked and the verifier binary is configured. A localnet or a scripted devnet runs without them,
// on every node alike, and refuses every bundle at execution; there the proposal check would refuse
// every shielded proposal instead of leaving that to execution, so it does not run. Every other
// chain refuses to start without them (CheckNodeCanVerify).
func (app *OramaApp) canCheckProofs() bool {
	if !orchardverify.Linked || len(app.ShieldedMempoolVerifiers) != 2 {
		return false
	}
	m, ok := app.ShieldedMempoolVerifiers[1].(*mempoolVerifier)
	return ok && m.configured
}

// txProofsOK is proofsOK for a transaction's bytes. Bytes that do not decode carry no bundle: the
// SDK handlers decide what to do with them.
func (h *inclusionHandlers) txProofsOK(raw []byte) bool {
	if h.rules.proofs == nil {
		return true
	}
	tx, err := h.rules.decode(raw)
	if err != nil {
		return true
	}
	return h.rules.proofs(tx)
}

// allProofsOK reports whether every transaction of a proposal passes the proof check.
func (h *inclusionHandlers) allProofsOK(txs [][]byte) bool {
	for _, raw := range txs {
		if !h.txProofsOK(raw) {
			return false
		}
	}
	return true
}

// dropFailingProofs returns txs, in order, without the transactions that fail the proof check.
func (h *inclusionHandlers) dropFailingProofs(txs [][]byte) [][]byte {
	out := make([][]byte, 0, len(txs))
	for _, raw := range txs {
		if h.txProofsOK(raw) {
			out = append(out, raw)
		}
	}
	return out
}
