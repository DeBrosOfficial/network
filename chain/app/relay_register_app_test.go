package app_test

import (
	stded25519 "crypto/ed25519"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"cosmossdk.io/math"

	"github.com/DeBrosOfficial/network/chain/app/params"
	emissiontypes "github.com/DeBrosOfficial/network/chain/x/emission/types"
	nodeskeeper "github.com/DeBrosOfficial/network/chain/x/nodes/keeper"
	nodestypes "github.com/DeBrosOfficial/network/chain/x/nodes/types"
	relaykeeper "github.com/DeBrosOfficial/network/chain/x/relay/keeper"
	relaytypes "github.com/DeBrosOfficial/network/chain/x/relay/types"
)

// relayNode is a registered x/nodes node carrying an ed25519 "relay" binding.
type relayNode struct {
	id       string
	operator sdk.AccAddress
	priv     stded25519.PrivateKey
}

// addRelayNode registers an operator and a RELAY node with the given endpoint on the real app.
func (c *wiringChain) addRelayNode(id, endpoint string) relayNode {
	c.t.Helper()
	return c.addNodeWith(id, endpoint, []nodestypes.Role{nodestypes.RoleRelay}, "relay")
}

// addNodeWith registers an operator and a node with the given roles, whose ed25519 key is bound under service.
func (c *wiringChain) addNodeWith(id, endpoint string, roles []nodestypes.Role, service string) relayNode {
	c.t.Helper()
	n := relayNode{id: id, operator: sdk.AccAddress(ed25519.GenPrivKey().PubKey().Address())}
	hotPriv := secp256k1.GenPrivKey()
	hot := sdk.AccAddress(hotPriv.PubKey().Address())
	hotSig, err := hotPriv.Sign(nodestypes.BindingSignBytes(testChainID, n.operator.String(), nodestypes.HotKeyService, hotPriv.PubKey().Bytes()))
	require.NoError(c.t, err)
	pub, priv, err := stded25519.GenerateKey(nil)
	require.NoError(c.t, err)
	n.priv = priv
	c.write(func(ctx sdk.Context) {
		funds := sdk.NewCoins(sdk.NewCoin(params.BaseDenom, math.NewInt(params.NoramaPerOrama).MulRaw(20)))
		require.NoError(c.t, mintNoramaForTest(c.app, ctx, funds))
		require.NoError(c.t, c.app.BankKeeper.SendCoinsFromModuleToAccount(ctx, emissiontypes.ModuleName, n.operator, funds))
		srv := nodeskeeper.NewMsgServerImpl(c.app.NodesKeeper)
		_, err := srv.RegisterOperator(ctx, &nodestypes.MsgRegisterOperator{Operator: n.operator.String()})
		require.NoError(c.t, err)
		_, err = srv.RegisterNode(ctx, &nodestypes.MsgRegisterNode{
			Operator: n.operator.String(), NodeId: id, Roles: roles, HotKey: hot.String(),
			Bindings: []nodestypes.Binding{{
				Service: nodestypes.HotKeyService, KeyType: nodestypes.KeyTypeSecp256k1, Pubkey: hotPriv.PubKey().Bytes(), Signature: hotSig,
			}, {
				Service: service, KeyType: nodestypes.KeyTypeEd25519, Pubkey: pub,
				Signature: stded25519.Sign(priv, nodestypes.BindingSignBytes(testChainID, n.operator.String(), service, pub)),
			}},
			Endpoints: []string{endpoint},
		})
		require.NoError(c.t, err)
	})
	return n
}

// registerRelay sends MsgRegisterRelay through x/relay's msg server, wired to the real x/nodes.
func (c *wiringChain) registerRelay(n relayNode, mark byte) []byte {
	c.t.Helper()
	fp := make([]byte, relaytypes.RSAFingerprintLen)
	for i := range fp {
		fp[i] = mark
	}
	c.write(func(ctx sdk.Context) {
		_, err := relaykeeper.NewMsgServerImpl(c.app.RelayKeeper).RegisterRelay(ctx, &relaytypes.MsgRegisterRelay{
			Operator: n.operator.String(), NodeId: n.id, RsaFingerprint: fp,
			Ed25519Signature: stded25519.Sign(n.priv, relaytypes.CrossCertMessage(n.id, fp)),
		})
		require.NoError(c.t, err)
	})
	return fp
}

func (c *wiringChain) storedRelay(fp []byte) relaytypes.Relay {
	c.t.Helper()
	relay, err := c.app.RelayKeeper.Relays.Get(c.app.NewContext(true), fp)
	require.NoError(c.t, err)
	return relay
}

func TestRegisterRelay_literalIPEndpointGetsItsSlash16(t *testing.T) {
	c := newWiringChain(t)
	n := c.addRelayNode("relay-ip", "https://93.184.216.34:443")
	fp := c.registerRelay(n, 0x11)
	require.Equal(t, "93.184.0.0/16", c.storedRelay(fp).Prefix16)
}

func TestRegisterRelay_noLiteralIPUsesUnidentifiedBucket(t *testing.T) {
	c := newWiringChain(t)
	a := c.addRelayNode("relay-host-a", "https://relay-a.example.com:443")
	b := c.addRelayNode("relay-host-b", "https://relay-b.example.com:443")
	fpA := c.registerRelay(a, 0x11)
	fpB := c.registerRelay(b, 0x22)
	require.Equal(t, relaytypes.UnidentifiedPrefix16, c.storedRelay(fpA).Prefix16)
	require.Equal(t, relaytypes.UnidentifiedPrefix16, c.storedRelay(fpB).Prefix16)
}

func TestRegisterRelay_identityInsideLockIsUnidentified(t *testing.T) {
	c := newWiringChainWith(t, func(gs *nodestypes.GenesisState) {
		gs.Params.NetworkIdentityLockSeconds = 1_000_000
	})
	n := c.addRelayNode("relay-new", "https://93.184.216.34:443")
	fp := c.registerRelay(n, 0x11)
	require.Equal(t, relaytypes.UnidentifiedPrefix16, c.storedRelay(fp).Prefix16)
}

// registerRelayErr sends MsgRegisterRelay for n with fingerprint fp through x/relay wired to the real x/nodes.
func (c *wiringChain) registerRelayErr(n relayNode, fp []byte) error {
	c.t.Helper()
	var err error
	c.write(func(ctx sdk.Context) {
		_, err = relaykeeper.NewMsgServerImpl(c.app.RelayKeeper).RegisterRelay(ctx, &relaytypes.MsgRegisterRelay{
			Operator: n.operator.String(), NodeId: n.id, RsaFingerprint: fp,
			Ed25519Signature: stded25519.Sign(n.priv, relaytypes.CrossCertMessage(n.id, fp)),
		})
	})
	return err
}

func TestRegisterRelay_refusesANodeWithoutTheRelayRole(t *testing.T) {
	c := newWiringChain(t)
	n := c.addNodeWith("storage-only", "https://93.184.216.34:443", []nodestypes.Role{nodestypes.RoleStorage}, "relay")
	err := c.registerRelayErr(n, bytes32(0x11)[:relaytypes.RSAFingerprintLen])
	require.ErrorContains(t, err, "does not have role ROLE_RELAY")
}

func TestRegisterRelay_refusesARelayNodeThatBindsNoRelayKey(t *testing.T) {
	c := newWiringChain(t)
	n := c.addNodeWith("relay-consensus", "https://93.184.216.34:443", []nodestypes.Role{nodestypes.RoleRelay}, "consensus")
	err := c.registerRelayErr(n, bytes32(0x11)[:relaytypes.RSAFingerprintLen])
	require.ErrorContains(t, err, `binds no ed25519 key for service "relay"`,
		"another ed25519 key of the node is never taken for the relay's: its votes would never match")
}

func TestUpdateRelay_followsAMsgUpdateNodeRebind(t *testing.T) {
	c := newWiringChain(t)
	n := c.addRelayNode("relay-rotate", "https://93.184.216.34:443")
	fp := c.registerRelay(n, 0x11)

	newPub, newPriv, err := stded25519.GenerateKey(nil)
	require.NoError(t, err)
	hot := c.nodeBindings(n.id)[0]
	c.write(func(ctx sdk.Context) {
		_, err := nodeskeeper.NewMsgServerImpl(c.app.NodesKeeper).UpdateNode(ctx, &nodestypes.MsgUpdateNode{
			Operator: n.operator.String(), NodeId: n.id,
			Bindings: []nodestypes.Binding{hot, {
				Service: "relay", KeyType: nodestypes.KeyTypeEd25519, Pubkey: newPub,
				Signature: stded25519.Sign(newPriv, nodestypes.BindingSignBytes(testChainID, n.operator.String(), "relay", newPub)),
			}},
		})
		require.NoError(t, err)
	})
	require.Equal(t, []byte(n.priv.Public().(stded25519.PublicKey)), c.storedRelay(fp).Ed25519Id, "registration snapshotted the old key")

	c.write(func(ctx sdk.Context) {
		_, err := relaykeeper.NewMsgServerImpl(c.app.RelayKeeper).UpdateRelay(ctx, &relaytypes.MsgUpdateRelay{
			Operator: n.operator.String(), NodeId: n.id, RsaFingerprint: fp,
			Ed25519Signature: stded25519.Sign(newPriv, relaytypes.CrossCertMessage(n.id, fp)),
		})
		require.NoError(t, err)
	})
	require.Equal(t, []byte(newPub), c.storedRelay(fp).Ed25519Id)
}

func bytes32(mark byte) []byte {
	out := make([]byte, 32)
	for i := range out {
		out[i] = mark
	}
	return out
}

// nodeBindings is the binding set the chain holds for a node.
func (c *wiringChain) nodeBindings(id string) []nodestypes.Binding {
	c.t.Helper()
	node, err := c.app.NodesKeeper.GetNode(c.app.NewContext(true), id)
	require.NoError(c.t, err)
	return node.Bindings
}
