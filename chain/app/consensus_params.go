package app

import (
	"fmt"
	"sort"
	"strings"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"

	"github.com/DeBrosOfficial/network/chain/app/params"
)

// The consensus parameters live in the genesis document's top-level consensus field, not in
// app_state, so ValidateLockedGenesis never sees them. Three of them bound the work and the
// fees of every block: block.max_gas (-1 is unlimited, and the base fee and the shielded proof work
// are bounded by it), block.max_bytes, and abci.vote_extensions_enable_height (0 switches the
// inclusion lists off). They are validated here from InitChain's request, by chain-id class like
// the locked parameters (params.ClassOfChainID):
//   - orama-localnet-<name>: free.
//   - orama-devnet-<name> and orama-stagenet-<name>: a finite block gas limit, a block size above
//     zero and not over ConsensusMaxBlockBytes, and vote extensions on from some height.
//   - anything else (production): exactly the values `orama setup --create-network` writes.
const (
	// ConsensusMaxGas is the block gas limit of a genesis (core/cmd/orama/internal/setup
	// consensusMaxGas, chain/scripts/stagenet/deploy.sh).
	ConsensusMaxGas int64 = 100_000_000
	// ConsensusMaxBlockBytes is the block size of a genesis, CometBFT's default of 21 MiB, and the
	// most a test network may set. It equals x/inclusion's DefaultMaxBlockBytes, the budget the
	// inclusion handlers fall back on.
	ConsensusMaxBlockBytes int64 = 21 * 1024 * 1024
	// ConsensusVoteExtensionsEnableHeight is the height vote extensions turn on at (core/cmd/orama/
	// internal/setup voteExtensionsEnableHeight): block 1 stays an ordinary block.
	ConsensusVoteExtensionsEnableHeight int64 = 2
)

// ValidateConsensusParams returns an error naming every consensus parameter of cp that the class of
// chainID does not allow. A nil cp is refused on every class but a localnet.
func ValidateConsensusParams(chainID string, cp *cmtproto.ConsensusParams) error {
	class := params.ClassOfChainID(chainID)
	if class == params.ChainLocalnet {
		return nil
	}
	if cp == nil {
		return fmt.Errorf("InitChain carries no consensus parameters on chain-id %q", chainID)
	}
	production := class == params.ChainProduction
	var maxGas, maxBytes, voteHeight int64
	if cp.Block != nil {
		maxGas, maxBytes = cp.Block.MaxGas, cp.Block.MaxBytes
	}
	if cp.Abci != nil {
		voteHeight = cp.Abci.VoteExtensionsEnableHeight
	}
	var diffs []string
	switch {
	case production && maxGas != ConsensusMaxGas:
		diffs = append(diffs, fmt.Sprintf("block.max_gas is %d, want %d", maxGas, ConsensusMaxGas))
	case maxGas <= 0:
		diffs = append(diffs, fmt.Sprintf("block.max_gas is %d, want a finite limit above zero", maxGas))
	}
	switch {
	case production && maxBytes != ConsensusMaxBlockBytes:
		diffs = append(diffs, fmt.Sprintf("block.max_bytes is %d, want %d", maxBytes, ConsensusMaxBlockBytes))
	case maxBytes <= 0 || maxBytes > ConsensusMaxBlockBytes:
		diffs = append(diffs, fmt.Sprintf("block.max_bytes is %d, want above zero and at most %d", maxBytes, ConsensusMaxBlockBytes))
	}
	switch {
	case production && voteHeight != ConsensusVoteExtensionsEnableHeight:
		diffs = append(diffs, fmt.Sprintf("abci.vote_extensions_enable_height is %d, want %d", voteHeight, ConsensusVoteExtensionsEnableHeight))
	case voteHeight <= 0:
		diffs = append(diffs, fmt.Sprintf("abci.vote_extensions_enable_height is %d, want the height they turn on at (above zero)", voteHeight))
	}
	if len(diffs) == 0 {
		return nil
	}
	sort.Strings(diffs)
	return fmt.Errorf("consensus parameters differ from the allowed values on chain-id %q:\n  %s", chainID, strings.Join(diffs, "\n  "))
}
