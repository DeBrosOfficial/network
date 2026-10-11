package keeper_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"

	"github.com/DeBrosOfficial/network/chain/x/relay/keeper"
	"github.com/DeBrosOfficial/network/chain/x/relay/types"
)

// updateMsg is a MsgUpdateRelay for rk's node carrying fingerprint fp, cross-signed by key.
func updateMsg(rk relayKey, fp []byte, key ed25519.PrivateKey) *types.MsgUpdateRelay {
	return &types.MsgUpdateRelay{
		Operator:         rk.operator.String(),
		NodeId:           rk.nodeID,
		RsaFingerprint:   fp,
		Ed25519Signature: ed25519.Sign(key, types.CrossCertMessage(rk.nodeID, fp)),
	}
}

func fingerprintOf(mark byte) []byte { return bytes.Repeat([]byte{mark}, types.RSAFingerprintLen) }

// eventsOf runs fn with a fresh event manager and returns the events of the given type it emitted.
func eventsOf(f *testFixture, eventType string, fn func(ctx sdk.Context)) []sdk.Event {
	ctx := f.Ctx.WithEventManager(sdk.NewEventManager())
	fn(ctx)
	var out []sdk.Event
	for _, e := range ctx.EventManager().Events() {
		if e.Type == eventType {
			out = append(out, e)
		}
	}
	return out
}

func attr(e sdk.Event, key string) string {
	for _, a := range e.Attributes {
		if a.Key == key {
			return a.Value
		}
	}
	return ""
}

func TestRegisterRelay_refusals(t *testing.T) {
	f := newTestFixture(t)
	f.init(t, 1, []sdk.AccAddress{acc(1)}, nil)
	operator, other := acc(2), acc(3)
	rk := newRelayKey(t, "node-a", 0x11, operator, "10.1.0.1")
	f.register(t, rk, false)
	second := newRelayKey(t, "node-b", 0x22, operator, "10.1.0.2")
	f.Nodes.add(second)

	short := registerMsg(second)
	short.RsaFingerprint = short.RsaFingerprint[:types.RSAFingerprintLen-1]
	noNode := registerMsg(second)
	noNode.NodeId = ""
	dup := registerMsg(second)
	dup.RsaFingerprint = rk.fp
	dup.Ed25519Signature = ed25519.Sign(second.priv, types.CrossCertMessage(second.nodeID, rk.fp))
	again := registerMsg(rk)
	again.RsaFingerprint = fingerprintOf(0x66)
	again.Ed25519Signature = ed25519.Sign(rk.priv, types.CrossCertMessage(rk.nodeID, again.RsaFingerprint))
	notOwner := registerMsg(second)
	notOwner.Operator = other.String()
	badSig := registerMsg(second)
	badSig.Ed25519Signature = ed25519.Sign(rk.priv, types.CrossCertMessage(second.nodeID, second.fp))
	unknown := registerMsg(second)
	unknown.NodeId = "no-such-node"

	for name, c := range map[string]struct {
		msg  *types.MsgRegisterRelay
		want string
	}{
		"fingerprint of 19 bytes":      {short, "rsa fingerprint must be 20 bytes"},
		"empty node id":                {noNode, "node id is empty"},
		"fingerprint already taken":    {dup, "fingerprint already registered"},
		"node already has a relay":     {again, "MsgUpdateRelay rotates"},
		"operator does not own node":   {notOwner, "does not own node"},
		"cross-signature of another":   {badSig, types.ErrCrossCertMismatch.Error()},
		"node the chain does not know": {unknown, "no-such-node"},
	} {
		_, err := f.Msg.RegisterRelay(f.Ctx, c.msg)
		require.Error(t, err, name)
		require.Contains(t, err.Error(), c.want, name)
	}
	relays, err := keeper.NewQueryServerImpl(f.Keeper).Relays(f.Ctx, &types.QueryRelaysRequest{})
	require.NoError(t, err)
	require.Len(t, relays.Relays, 1, "a refused registration leaves nothing behind")
}

func TestRegisterRelay_emitsWhatAnIndexerNeeds(t *testing.T) {
	f := newTestFixture(t)
	f.init(t, 1, []sdk.AccAddress{acc(1)}, nil)
	rk := newRelayKey(t, "node-a", 0xab, acc(2), "10.1.0.1")
	f.Nodes.add(rk)

	events := eventsOf(f, types.EventTypeRelayRegistered, func(ctx sdk.Context) {
		_, err := f.Msg.RegisterRelay(ctx, registerMsg(rk))
		require.NoError(t, err)
	})
	require.Len(t, events, 1)
	require.Equal(t, strings.Repeat("AB", types.RSAFingerprintLen), attr(events[0], types.AttributeKeyRSAFingerprint), "40 upper-case hex digits")
	require.Equal(t, "node-a", attr(events[0], types.AttributeKeyNodeID))
	require.Equal(t, acc(2).String(), attr(events[0], types.AttributeKeyOperator))
	require.Equal(t, hex.EncodeToString(rk.pub), attr(events[0], types.AttributeKeyEd25519ID))
}

func TestUpdateRelay_rotatesTheFingerprintAndKeepsTheJail(t *testing.T) {
	f := newTestFixture(t)
	f.init(t, 1, []sdk.AccAddress{acc(1)}, nil)
	operator := acc(2)
	rk := newRelayKey(t, "node-a", 0x11, operator, "10.1.0.1")
	f.register(t, rk, false)
	require.NoError(t, f.Keeper.JailRelay(f.Ctx, rk.fp))

	newFP := fingerprintOf(0x77)
	events := eventsOf(f, types.EventTypeRelayUpdated, func(ctx sdk.Context) {
		msg := updateMsg(rk, newFP, rk.priv)
		msg.Exit = true
		_, err := f.Msg.UpdateRelay(ctx, msg)
		require.NoError(t, err)
	})
	require.Len(t, events, 1)
	require.Equal(t, strings.Repeat("77", types.RSAFingerprintLen), attr(events[0], types.AttributeKeyRSAFingerprint))
	require.Equal(t, strings.Repeat("11", types.RSAFingerprintLen), attr(events[0], types.AttributeKeyPreviousRSAFingerprint))

	_, err := f.Keeper.Relays.Get(f.Ctx, rk.fp)
	require.Error(t, err, "the old fingerprint is no longer a registered relay")
	relay, err := f.Keeper.Relays.Get(f.Ctx, newFP)
	require.NoError(t, err)
	require.Equal(t, "node-a", relay.NodeId)
	require.True(t, relay.Exit)
	require.True(t, relay.Jailed, "rotating a key does not unjail a relay")
	indexed, err := f.Keeper.NodeIndex.Get(f.Ctx, "node-a")
	require.NoError(t, err)
	require.Equal(t, newFP, indexed)

	res, err := keeper.NewQueryServerImpl(f.Keeper).OperatorRelays(f.Ctx, &types.QueryOperatorRelaysRequest{Operator: operator.String()})
	require.NoError(t, err)
	require.Len(t, res.Relays, 1, "the operator's list follows the rotation")
	require.Equal(t, newFP, res.Relays[0].Relay.RsaFingerprint)
}

func TestUpdateRelay_followsTheNodesRotatedEd25519Key(t *testing.T) {
	f := newTestFixture(t)
	f.init(t, 1, []sdk.AccAddress{acc(1)}, nil)
	rk := newRelayKey(t, "node-a", 0x11, acc(2), "10.1.0.1")
	f.register(t, rk, false)

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	node := f.Nodes.byID["node-a"]
	node.pub = pub
	f.Nodes.byID["node-a"] = node

	stale := updateMsg(rk, rk.fp, rk.priv)
	_, err = f.Msg.UpdateRelay(f.Ctx, stale)
	require.ErrorIs(t, err, types.ErrCrossCertMismatch, "the old key no longer cross-signs for the node")

	_, err = f.Msg.UpdateRelay(f.Ctx, updateMsg(rk, rk.fp, priv))
	require.NoError(t, err)
	relay, err := f.Keeper.Relays.Get(f.Ctx, rk.fp)
	require.NoError(t, err)
	require.Equal(t, []byte(pub), relay.Ed25519Id, "the relay now carries the key the node binds")
}

func TestUpdateRelay_refusals(t *testing.T) {
	f := newTestFixture(t)
	reporter := acc(1)
	f.init(t, 1, []sdk.AccAddress{reporter}, nil)
	operator, other := acc(2), acc(3)
	rk := newRelayKey(t, "node-a", 0x11, operator, "10.1.0.1")
	f.register(t, rk, false)
	bare := newRelayKey(t, "node-b", 0x22, operator, "10.1.0.2")
	f.Nodes.add(bare)
	third := newRelayKey(t, "node-c", 0x33, other, "10.1.0.3")
	f.register(t, third, false)

	notOwner := updateMsg(rk, fingerprintOf(0x44), rk.priv)
	notOwner.Operator = other.String()
	taken := updateMsg(rk, third.fp, rk.priv)
	short := updateMsg(rk, fingerprintOf(0x44)[:19], rk.priv)
	badSig := updateMsg(rk, fingerprintOf(0x44), third.priv)
	noRelay := updateMsg(bare, fingerprintOf(0x55), bare.priv)
	asReporter := updateMsg(rk, fingerprintOf(0x44), rk.priv)
	asReporter.Operator = reporter.String()

	for name, c := range map[string]struct {
		msg  *types.MsgUpdateRelay
		want string
	}{
		"operator does not own the node": {notOwner, "does not own node"},
		"fingerprint of another relay":   {taken, "fingerprint already registered"},
		"fingerprint of 19 bytes":        {short, "rsa fingerprint must be 20 bytes"},
		"cross-signature of another key": {badSig, types.ErrCrossCertMismatch.Error()},
		"a node with no relay":           {noRelay, types.ErrRelayNotRegistered.Error()},
		"a reporter":                     {asReporter, types.ErrReporterOperatesRelay.Error()},
	} {
		_, err := f.Msg.UpdateRelay(f.Ctx, c.msg)
		require.Error(t, err, name)
		require.Contains(t, err.Error(), c.want, name)
	}
	relay, err := f.Keeper.Relays.Get(f.Ctx, rk.fp)
	require.NoError(t, err, "every refusal leaves the relay where it was")
	require.Equal(t, "node-a", relay.NodeId)
}

func TestReportEpoch_refusesARelayThatIsNotRegistered(t *testing.T) {
	f := newTestFixture(t)
	reporter := acc(1)
	f.init(t, 1, []sdk.AccAddress{reporter}, nil)
	rk := newRelayKey(t, "node-a", 0x11, acc(2), "10.1.0.1")
	f.register(t, rk, false)
	stranger := newRelayKey(t, "node-x", 0x99, acc(3), "10.9.0.1")

	err := f.submit(t, reporter, 1, []types.RelayObservation{obs(rk, 10, "1", false), obs(stranger, 10, "1", false)})
	require.ErrorContains(t, err, "relay is not registered", "one unregistered relay refuses the whole chunk, so a reporter reports only what the chain knows")

	rotated := fingerprintOf(0x77)
	_, err = f.Msg.UpdateRelay(f.Ctx, updateMsg(rk, rotated, rk.priv))
	require.NoError(t, err)
	err = f.submitAt(t, reporter, 1, []types.RelayObservation{obs(rk, 10, "1", false)})
	require.ErrorContains(t, err, "relay is not registered", "a fingerprint a rotation replaced is not a registered relay")
}

func TestSettle_paysOnlyTheRegisteredFingerprintAfterARotation(t *testing.T) {
	f := newTestFixture(t)
	reporter, operator := acc(1), acc(2)
	f.init(t, 1, []sdk.AccAddress{reporter}, nil)
	rk := newRelayKey(t, "node-a", 0x11, operator, "10.1.0.1")
	f.register(t, rk, false)
	f.activate(t, []sdk.AccAddress{reporter}, []types.RelayObservation{obs(rk, 1, "1", false)})

	f.Emission.setCeiling(2, math.NewInt(1000))
	require.NoError(t, f.submit(t, reporter, 2, []types.RelayObservation{obs(rk, 40, "1", false)}))
	_, err := f.Msg.UpdateRelay(f.Ctx, updateMsg(rk, fingerprintOf(0x77), rk.priv))
	require.NoError(t, err)

	result, err := f.Keeper.SettleEpoch(f.Ctx, 2)
	require.NoError(t, err)
	require.True(t, result.Minted.IsZero(), "the report named a fingerprint that is no longer registered")
	require.Empty(t, f.payouts(t, 2))
	require.True(t, f.Earnings.balance(operator).IsZero())
}

func TestSettle_emitsWhoWasPaidAndHowTheSplitFell(t *testing.T) {
	f := newTestFixture(t)
	reporter, operator := acc(1), acc(2)
	f.init(t, 1, []sdk.AccAddress{reporter}, nil)
	rk := newRelayKey(t, "node-a", 0x11, operator, "10.1.0.1")
	f.register(t, rk, false)
	f.activate(t, []sdk.AccAddress{reporter}, []types.RelayObservation{obs(rk, 1, "1", false)})
	f.Emission.setCeiling(2, math.NewInt(1000))
	require.NoError(t, f.submit(t, reporter, 2, []types.RelayObservation{obs(rk, 40, "1", false)}))

	ctx := f.Ctx.WithEventManager(sdk.NewEventManager())
	_, err := f.Keeper.SettleEpoch(ctx, 2)
	require.NoError(t, err)
	var payout, paid []sdk.Event
	for _, e := range ctx.EventManager().Events() {
		switch e.Type {
		case types.EventTypeRelayPayout:
			payout = append(payout, e)
		case types.EventTypeOperatorPaid:
			paid = append(paid, e)
		}
	}
	require.Len(t, payout, 1)
	require.Equal(t, "2", attr(payout[0], types.AttributeKeyEpoch))
	require.Equal(t, strings.Repeat("11", types.RSAFingerprintLen), attr(payout[0], types.AttributeKeyRSAFingerprint))
	require.Equal(t, "node-a", attr(payout[0], types.AttributeKeyNodeID))
	require.Equal(t, operator.String(), attr(payout[0], types.AttributeKeyOperator))
	require.Equal(t, "40", attr(payout[0], types.AttributeKeyAmount))
	require.Len(t, paid, 1)
	require.Equal(t, "40", attr(paid[0], types.AttributeKeyAmount))
	require.Equal(t, f.Earnings.balance(operator).String(), attr(paid[0], types.AttributeKeyEarnings), "the event names what reached the earnings account")
	earn, burn, archive := f.Earnings.balance(operator), f.Service.burned, f.Service.archive
	require.True(t, earn.Add(burn).Add(archive).Equal(math.NewInt(40)))
}

func TestOperatorRelays_listsEachRelayWithItsLastPayout(t *testing.T) {
	f := newTestFixture(t)
	reporter, operator, other := acc(1), acc(2), acc(3)
	f.init(t, 1, []sdk.AccAddress{reporter}, nil)
	paid := newRelayKey(t, "node-a", 0x11, operator, "10.1.0.1")
	idle := newRelayKey(t, "node-b", 0x22, operator, "10.2.0.1")
	foreign := newRelayKey(t, "node-c", 0x33, other, "10.3.0.1")
	f.register(t, paid, false)
	f.register(t, idle, false)
	f.register(t, foreign, false)
	f.activate(t, []sdk.AccAddress{reporter}, []types.RelayObservation{obs(paid, 1, "1", false)})
	for epoch, weight := range map[uint64]int64{2: 40, 3: 60} {
		f.Emission.setCeiling(epoch, math.NewInt(1000))
		require.NoError(t, f.submit(t, reporter, epoch, []types.RelayObservation{obs(paid, weight, "1", false), obs(foreign, 10, "1", false)}))
		_, err := f.Keeper.SettleEpoch(f.Ctx, epoch)
		require.NoError(t, err)
	}

	q := keeper.NewQueryServerImpl(f.Keeper)
	res, err := q.OperatorRelays(f.Ctx, &types.QueryOperatorRelaysRequest{Operator: operator.String()})
	require.NoError(t, err)
	require.Len(t, res.Relays, 2, "another operator's relay is not listed")
	require.Equal(t, paid.fp, res.Relays[0].Relay.RsaFingerprint)
	require.Equal(t, uint64(3), res.Relays[0].LastPayoutEpoch)
	require.Equal(t, "60", res.Relays[0].LastPayoutAmount.String())
	require.Equal(t, idle.fp, res.Relays[1].Relay.RsaFingerprint)
	require.Equal(t, uint64(0), res.Relays[1].LastPayoutEpoch)
	require.True(t, res.Relays[1].LastPayoutAmount.IsZero())

	page, err := q.OperatorRelays(f.Ctx, &types.QueryOperatorRelaysRequest{Operator: operator.String(), Pagination: &query.PageRequest{Limit: 1}})
	require.NoError(t, err)
	require.Len(t, page.Relays, 1)
	require.NotNil(t, page.Pagination.NextKey, "a page of one leaves the other for the next page")
}

func TestOperatorRelays_refusalsAndEmpty(t *testing.T) {
	f := newTestFixture(t)
	q := keeper.NewQueryServerImpl(f.Keeper)
	_, err := q.OperatorRelays(f.Ctx, &types.QueryOperatorRelaysRequest{})
	require.Error(t, err)
	_, err = q.OperatorRelays(f.Ctx, &types.QueryOperatorRelaysRequest{Operator: "not-an-address"})
	require.Error(t, err)
	res, err := q.OperatorRelays(f.Ctx, &types.QueryOperatorRelaysRequest{Operator: acc(9).String()})
	require.NoError(t, err)
	require.NotNil(t, res.Relays)
	require.Empty(t, res.Relays)
}

func TestRelayPayouts_pagesOneEpochOnly(t *testing.T) {
	f := newTestFixture(t)
	reporter := acc(1)
	f.init(t, 1, []sdk.AccAddress{reporter}, nil)
	a := newRelayKey(t, "node-a", 0x11, acc(2), "10.1.0.1")
	b := newRelayKey(t, "node-b", 0x22, acc(3), "10.2.0.1")
	f.register(t, a, false)
	f.register(t, b, false)
	f.activate(t, []sdk.AccAddress{reporter}, []types.RelayObservation{obs(a, 1, "1", false)})
	for epoch := uint64(2); epoch <= 3; epoch++ {
		f.Emission.setCeiling(epoch, math.NewInt(1000))
		require.NoError(t, f.submit(t, reporter, epoch, []types.RelayObservation{obs(a, 10*int64(epoch), "1", false), obs(b, 5, "1", false)}))
		_, err := f.Keeper.SettleEpoch(f.Ctx, epoch)
		require.NoError(t, err)
	}

	q := keeper.NewQueryServerImpl(f.Keeper)
	res, err := q.RelayPayouts(f.Ctx, &types.QueryRelayPayoutsRequest{Epoch: 3})
	require.NoError(t, err)
	require.Len(t, res.Payouts, 2)
	for _, p := range res.Payouts {
		require.Equal(t, uint64(3), p.Epoch)
	}
	first, err := q.RelayPayouts(f.Ctx, &types.QueryRelayPayoutsRequest{Epoch: 2, Pagination: &query.PageRequest{Limit: 1}})
	require.NoError(t, err)
	require.Len(t, first.Payouts, 1)
	require.NotNil(t, first.Pagination.NextKey)

	none, err := q.RelayPayouts(f.Ctx, &types.QueryRelayPayoutsRequest{Epoch: 9})
	require.NoError(t, err)
	require.NotNil(t, none.Payouts)
	require.Empty(t, none.Payouts)
	_, err = q.RelayPayouts(f.Ctx, &types.QueryRelayPayoutsRequest{})
	require.Error(t, err, "epoch 0 is not an epoch")
}

func TestGenesis_rebuildsTheOperatorIndexAndTheLastPayouts(t *testing.T) {
	f := newTestFixture(t)
	reporter, operator := acc(1), acc(2)
	f.init(t, 1, []sdk.AccAddress{reporter}, nil)
	rk := newRelayKey(t, "node-a", 0x11, operator, "10.1.0.1")
	f.register(t, rk, false)
	f.activate(t, []sdk.AccAddress{reporter}, []types.RelayObservation{obs(rk, 1, "1", false)})
	for epoch, weight := range map[uint64]int64{2: 40, 3: 60} {
		f.Emission.setCeiling(epoch, math.NewInt(1000))
		require.NoError(t, f.submit(t, reporter, epoch, []types.RelayObservation{obs(rk, weight, "1", false)}))
		_, err := f.Keeper.SettleEpoch(f.Ctx, epoch)
		require.NoError(t, err)
	}
	exported, err := f.Keeper.ExportGenesis(f.Ctx)
	require.NoError(t, err)

	fresh := newTestFixture(t)
	require.NoError(t, fresh.Keeper.InitGenesis(fresh.Ctx, *exported))
	res, err := keeper.NewQueryServerImpl(fresh.Keeper).OperatorRelays(fresh.Ctx, &types.QueryOperatorRelaysRequest{Operator: operator.String()})
	require.NoError(t, err)
	require.Len(t, res.Relays, 1)
	require.Equal(t, uint64(3), res.Relays[0].LastPayoutEpoch, "the newest payout wins, whatever order genesis lists them in")
	require.Equal(t, "60", res.Relays[0].LastPayoutAmount.String())
}
