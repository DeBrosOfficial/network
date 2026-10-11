package types_test

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"

	chainparams "github.com/DeBrosOfficial/network/chain/app/params"
	"github.com/DeBrosOfficial/network/chain/x/relay/types"
)

// These hex strings are the bytes core/pkg/clusterreg's EncodeRegisterRelay and EncodeUpdateRelay
// write for the same message (relay_test.go there asserts the same constants), so the CLI's
// hand-written encoder and the chain's Marshal cannot drift apart. Exit is field 4, a varint, and
// is absent when false.
const (
	registerRelayExitHex   = "0a2c6f72616d613139726c34636d32686d7238616679346b6c6470787a33666b61346a677571306135747570307312066e6f64652d611a14111111111111111111111111111111111111111120012a4022222222222222222222222222222222222222222222222222222222222222222222222222222222222222222222222222222222222222222222222222222222"
	registerRelayNoExitHex = "0a2c6f72616d613139726c34636d32686d7238616679346b6c6470787a33666b61346a677571306135747570307312066e6f64652d611a1411111111111111111111111111111111111111112a4022222222222222222222222222222222222222222222222222222222222222222222222222222222222222222222222222222222222222222222222222222222"
)

// crossCertGoldenHex is the cross-certificate message for node "node-a" and a fingerprint of
// twenty 0x11 bytes: the domain, a zero, the node id, a zero, the fingerprint.
const crossCertGoldenHex = "6f72616d612f72656c61792f7273612d63726f73732d636572742f763100" + "6e6f64652d61" + "00" + "1111111111111111111111111111111111111111"

func wireOperator(t *testing.T) string {
	t.Helper()
	return "orama19rl4cm2hmr8afy4kldpxz3fka4jguq0a5tup0s"
}

func TestMsgRegisterRelay_wireMatchesTheCLIEncoder(t *testing.T) {
	_ = chainparams.Bech32Prefix
	msg := types.MsgRegisterRelay{
		Operator: wireOperator(t), NodeId: "node-a", RsaFingerprint: bytes.Repeat([]byte{0x11}, 20),
		Exit: true, Ed25519Signature: bytes.Repeat([]byte{0x22}, 64),
	}
	got, err := msg.Marshal()
	require.NoError(t, err)
	require.Equal(t, registerRelayExitHex, hex.EncodeToString(got))

	msg.Exit = false
	got, err = msg.Marshal()
	require.NoError(t, err)
	require.Equal(t, registerRelayNoExitHex, hex.EncodeToString(got), "a false exit is not written")
}

func TestMsgUpdateRelay_wireIsRegisterRelaysWire(t *testing.T) {
	update := types.MsgUpdateRelay{
		Operator: wireOperator(t), NodeId: "node-a", RsaFingerprint: bytes.Repeat([]byte{0x11}, 20),
		Exit: true, Ed25519Signature: bytes.Repeat([]byte{0x22}, 64),
	}
	got, err := update.Marshal()
	require.NoError(t, err)
	require.Equal(t, registerRelayExitHex, hex.EncodeToString(got))
}

func TestCrossCertMessage_isPinned(t *testing.T) {
	got := types.CrossCertMessage("node-a", bytes.Repeat([]byte{0x11}, 20))
	require.Equal(t, crossCertGoldenHex, hex.EncodeToString(got))
}
