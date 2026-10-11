package keeper

import "fmt"

// VerifierHalt is the panic value that stops a node whose shielded verifier faulted while it
// executed a block (checkInBlock). It is a panic, not an error, because an error would become the
// transaction's result and this node alone would record it. The app lets it through the
// transaction runner's recovery (chain/app shielded.go), so it ends FinalizeBlock; consensus
// stops, the block is not committed, and the node executes the same block after a restart.
type VerifierHalt struct {
	Height int64
	Err    error
}

func (h VerifierHalt) Error() string {
	return fmt.Sprintf("the shielded verifier failed while this node executed block %d, so it cannot reach the same verdict as its peers "+
		"and stops instead of recording a result of its own: %v. "+
		"Fix the verifier and restart the node, which executes the block again: check that orama-orchard-verifier "+
		"(--shielded-verifier) exists, matches the pin linked into oramad and can run, that the machine has memory "+
		"(an OOM kill of the child is the usual cause), and read the lines above in the log", h.Height, h.Err)
}

func (h VerifierHalt) Unwrap() error { return h.Err }
