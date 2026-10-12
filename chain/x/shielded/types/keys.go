package types

import (
	"cosmossdk.io/collections"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)

const (
	// ModuleName is the name of x/shielded and of the module account that holds every pool's
	// coins and the coins behind queued unshields.
	ModuleName = "shielded"

	// StoreKey is the IAVL store key for x/shielded.
	StoreKey = ModuleName

	// TransientKey is the transient store that marks a nullifier as pending: in the mempool's
	// check state and, in a block, between a bundle's execution and the end-of-block flush.
	TransientKey = "transient_" + ModuleName

	// NullifierStoreFile is the dedicated nullifier database under the node's data directory.
	NullifierStoreFile = "shielded_nullifiers"

	// VintageOrchardV1 is the only circuit vintage today: orchard 0.15.5, PostNu6_3.
	VintageOrchardV1 uint32 = 1

	// SubtreeLeaves is the number of leaves in one subtree whose root the module reports when it
	// fills: 2^16, the shard size of the Orchard note-commitment tree (lightwalletd's
	// GetSubtreeRoots, zcash SHARD_HEIGHT 16). It is the SUBTREE_LEVEL of the Rust tree code.
	SubtreeLeaves uint64 = 1 << 16

	// EventTypeSubtreeComplete is emitted when an appended commitment fills a subtree. Its
	// attributes are AttributeSubtreeIndex (the subtree's number, from 0) and AttributeSubtreeRoot
	// (its root, lowercase hex). The event is not state: nothing in the app hash depends on it, and
	// the indexer reads it to serve the subtree roots (chain/indexer).
	EventTypeSubtreeComplete = "shielded_subtree_complete"
	AttributeSubtreeIndex    = "index"
	AttributeSubtreeRoot     = "root"
)

// The collections prefixes of the IAVL store.
var (
	ParamsKey          = collections.NewPrefix(0)
	PoolsPrefix        = collections.NewPrefix(1)
	LimitersPrefix     = collections.NewPrefix(2)
	QueuePrefix        = collections.NewPrefix(3)
	NextQueueIDPrefix  = collections.NewPrefix(4)
	FrontierKey        = collections.NewPrefix(5)
	TreeSizeKey        = collections.NewPrefix(6)
	CurrentRootKey     = collections.NewPrefix(7)
	AnchorsPrefix      = collections.NewPrefix(8)
	AnchorHeightPrefix = collections.NewPrefix(9)
	AccumulatorKey     = collections.NewPrefix(10)
	NullifierCountKey  = collections.NewPrefix(11)
)

// The collections prefixes of the transient store.
var (
	PendingListPrefix  = collections.NewPrefix(0)
	PendingSetPrefix   = collections.NewPrefix(1)
	PendingSeqPrefix   = collections.NewPrefix(2)
	SignerlessCountKey = collections.NewPrefix(3)
)

// SignerlessAddress is the fixed address a MsgShieldedTransfer names as its signer. It is a
// constant of the protocol, not an account: nothing signs for it, the tx carries no signature,
// and it links no user to the transfer.
func SignerlessAddress() sdk.AccAddress {
	return authtypes.NewModuleAddress(ModuleName)
}
