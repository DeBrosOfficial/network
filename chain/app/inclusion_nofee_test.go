package app_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	abci "github.com/cometbft/cometbft/abci/types"

	"github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

	"github.com/DeBrosOfficial/network/chain/app"
	"github.com/DeBrosOfficial/network/chain/x/inclusion"
)

// txWithoutAFee is a transaction the SDK decoder accepts, with one message, one signer and one
// signature, whose auth info states no fee at all. The SDK's GetFee and GetGas dereference that
// missing field, so code that reads the fee of a decoded transaction without the ante chain's
// panic recovery crashes on it. (The signature is garbage: nothing here verifies it.)
func txWithoutAFee(t testing.TB) []byte {
	t.Helper()
	app.SetAddressPrefixes()
	key := secp256k1.GenPrivKey()
	from := sdk.AccAddress(key.PubKey().Address()).String()
	msg, err := types.NewAnyWithValue(&banktypes.MsgSend{FromAddress: from, ToAddress: from})
	require.NoError(t, err)
	pub, err := types.NewAnyWithValue(key.PubKey())
	require.NoError(t, err)
	body, err := (&txtypes.TxBody{Messages: []*types.Any{msg}}).Marshal()
	require.NoError(t, err)
	auth, err := (&txtypes.AuthInfo{SignerInfos: []*txtypes.SignerInfo{{
		PublicKey: pub,
		ModeInfo:  &txtypes.ModeInfo{Sum: &txtypes.ModeInfo_Single_{Single: &txtypes.ModeInfo_Single{Mode: signingtypes.SignMode_SIGN_MODE_DIRECT}}},
	}}}).Marshal()
	require.NoError(t, err)
	raw, err := (&txtypes.TxRaw{BodyBytes: body, AuthInfoBytes: auth, Signatures: [][]byte{make([]byte, 64)}}).Marshal()
	require.NoError(t, err)
	return raw
}

// A validator's vote extension that lists such a transaction must be refused like any other bad
// list. VerifyVoteExtension that panics on it is turned into an error by BaseApp, and CometBFT
// halts a node when the application returns an error from VerifyVoteExtension: one validator could
// stop every other. Found by FuzzClientTxDecode, whose empty transaction decodes and then panics
// when its fee is read; FuzzVerifyVoteExtension holds the call to the rule.
func TestInclusion_verifyVoteExtensionRefusesATxWithoutAFee(t *testing.T) {
	c := newInclusionChain(t, inclusionEnableHeight)
	tx := txWithoutAFee(t)
	_, err := c.app.TxConfig().TxDecoder()(tx)
	require.NoError(t, err, "the SDK decoder accepts a transaction with no fee")

	list, err := inclusion.EncodeList([][]byte{tx})
	require.NoError(t, err)
	resp, err := c.app.VerifyVoteExtension(&abci.RequestVerifyVoteExtension{
		Height: c.height + 1, Hash: make([]byte, 32), ValidatorAddress: c.consAddr(0), VoteExtension: list,
	})
	require.NoError(t, err, "VerifyVoteExtension must answer, not fail: a failure halts the node")
	require.Equal(t, abci.ResponseVerifyVoteExtension_REJECT, resp.Status)
}
