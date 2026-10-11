//go:build e2e_fleet

package chainservices

import (
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1" // #nosec G505 -- a Tor relay fingerprint is SHA-1 by definition
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"testing"

	"github.com/DeBrosOfficial/network/e2e/features/internal/chain"
)

// Relay identity sizes (x/relay/types/limits.go), the cross-certificate and RSA proof domains
// (x/relay/types/crosscert.go, rsaproof.go).
const (
	rsaFingerprintLen = 20
	rsaBits           = 1024
	crossCertDomain   = "orama/relay/rsa-cross-cert/v1"
	rsaProofDomain    = "orama/relay/rsa-proof/v1"
	inputsRootLen     = 32
)

// rsaIdentity is a relay's RSA identity: its key, the PKCS#1 DER of the public half and the SHA-1 of that
// DER, which is the fingerprint Tor and the chain name the relay by.
type rsaIdentity struct {
	key *rsa.PrivateKey
	der []byte
	fp  []byte
}

func newRSAIdentity(t *testing.T) rsaIdentity {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, rsaBits)
	if err != nil {
		t.Fatal(err)
	}
	der := x509.MarshalPKCS1PublicKey(&key.PublicKey)
	sum := sha1.Sum(der)
	return rsaIdentity{key: key, der: der, fp: sum[:]}
}

// crossCert is the relay's ed25519 signature over domain||0||node id||0||fp.
func crossCert(priv ed25519.PrivateKey, nodeID string, fp []byte) []byte {
	msg := append(append(append(append([]byte(crossCertDomain), 0), nodeID...), 0), fp...)
	return ed25519.Sign(priv, msg)
}

// proof is the RSA identity's signature (PKCS#1 v1.5 over SHA-256) over domain||0||node id||0||the
// relay's ed25519 key: proof that whoever registers the fingerprint holds its key.
func (r rsaIdentity) proof(t *testing.T, nodeID string, ed25519Pub ed25519.PublicKey) []byte {
	t.Helper()
	msg := append(append(append(append([]byte(rsaProofDomain), 0), nodeID...), 0), ed25519Pub...)
	sum := sha256.Sum256(msg)
	sig, err := rsa.SignPKCS1v15(rand.Reader, r.key, crypto.SHA256, sum[:])
	if err != nil {
		t.Fatal(err)
	}
	return sig
}

// relayFields are the fields of MsgRegisterRelay and MsgUpdateRelay, which are the same.
type relayFields struct {
	operator, nodeID string
	fingerprint      []byte
	exit             bool
	ed25519Sig       []byte
	rsaPublicKey     []byte
	rsaSig           []byte
}

// validRelay is a registration of node nodeID for the RSA identity id, signed the way the chain checks it.
func validRelay(t *testing.T, operator, nodeID string, id rsaIdentity, priv ed25519.PrivateKey) relayFields {
	t.Helper()
	return relayFields{
		operator: operator, nodeID: nodeID, fingerprint: id.fp,
		ed25519Sig: crossCert(priv, nodeID, id.fp), rsaPublicKey: id.der,
		rsaSig: id.proof(t, nodeID, priv.Public().(ed25519.PublicKey)),
	}
}

func (f relayFields) msg(typeURL string) chain.Msg {
	b64 := base64.StdEncoding.EncodeToString
	return chain.NewMsg(typeURL, map[string]any{
		"operator": f.operator, "node_id": f.nodeID, "rsa_fingerprint": b64(f.fingerprint), "exit": f.exit,
		"ed25519_signature": b64(f.ed25519Sig), "rsa_public_key": b64(f.rsaPublicKey), "rsa_signature": b64(f.rsaSig),
	})
}

func (f relayFields) register() chain.Msg { return f.msg("/orama.relay.v1.MsgRegisterRelay") }
func (f relayFields) update() chain.Msg   { return f.msg("/orama.relay.v1.MsgUpdateRelay") }

// TestRelayRegister_crossCertifiedRelay: an operator registers its node's Tor
// identity: the RSA fingerprint cross-signed by the node's ed25519 "relay"
// binding key and proved by the RSA identity key itself. The registration is
// stored and readable by fingerprint; the same fingerprint again, and the same
// node with another fingerprint, are refused. (x/relay has no message that
// removes a relay, so the record stays for the disposable run chain; the node
// itself is retired at cleanup.)
func TestRelayRegister_crossCertifiedRelay(t *testing.T) {
	t.Parallel()
	c := chain.New(t)
	op := c.FundedValidator(t, chain.OperatorNode, chain.Orama(1))
	c.EnsureOperator(t, op)
	id, bindings := c.RegisterTestNode(t, op, []string{chain.RoleRelay}, "relay")
	rsaID := newRSAIdentity(t)
	reg := validRelay(t, op.Address, id, rsaID, bindings[0].Priv)
	chain.RequireOK(t, "register a cross-certified relay", c.Submit(t, op, chain.TxOptions{}, reg.register()))
	var v struct {
		Relay struct {
			NodeID   string `json:"node_id"`
			Operator string `json:"operator"`
		} `json:"relay"`
	}
	c.Query(t, op.Node, &v, "relay", "relay", hex.EncodeToString(rsaID.fp))
	if v.Relay.NodeID != id || v.Relay.Operator != op.Address {
		t.Errorf("relay record %+v", v.Relay)
	}
	chain.RequireRefused(t, "same fingerprint", c.Submit(t, op, chain.TxOptions{}, reg.register()), "fingerprint already registered")
	second := validRelay(t, op.Address, id, newRSAIdentity(t), bindings[0].Priv)
	chain.RequireRefused(t, "same node, new fingerprint", c.Submit(t, op, chain.TxOptions{}, second.register()), "is already registered")
	c.RequireInvariants(t, "a relay registration")
}

// TestRelayRegister_refusals: a cross-certificate by another key, over
// another fingerprint or node id, a fingerprint of the wrong size, an RSA key
// that is not the fingerprint's or a proof by another key, an operator that
// does not own the node, and a node that does not exist are refused before
// anything is stored.
func TestRelayRegister_refusals(t *testing.T) {
	t.Parallel()
	c := chain.New(t)
	op := c.FundedValidator(t, chain.OperatorNode, chain.Orama(1))
	c.EnsureOperator(t, op)
	id, bindings := c.RegisterTestNode(t, op, []string{chain.RoleRelay}, "relay")
	priv := bindings[0].Priv
	_, stranger, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	rsaID, strangerRSA := newRSAIdentity(t), newRSAIdentity(t)
	other := c.FundedValidator(t, 1, chain.Orama(1))
	edit := func(f relayFields, with func(*relayFields)) relayFields {
		with(&f)
		return f
	}
	good := validRelay(t, op.Address, id, rsaID, priv)
	cases := map[string]struct {
		k    chain.Key
		msg  relayFields
		want string
	}{
		"signed by another key":    {op, edit(good, func(f *relayFields) { f.ed25519Sig = crossCert(stranger, id, rsaID.fp) }), "does not match the ed25519 cross-signature"},
		"over another fingerprint": {op, edit(good, func(f *relayFields) { f.ed25519Sig = crossCert(priv, id, newRSAIdentity(t).fp) }), "does not match the ed25519 cross-signature"},
		"over another node id":     {op, edit(good, func(f *relayFields) { f.ed25519Sig = crossCert(priv, id+"x", rsaID.fp) }), "does not match the ed25519 cross-signature"},
		"19-byte fingerprint":      {op, edit(good, func(f *relayFields) { f.fingerprint = rsaID.fp[:19] }), "rsa fingerprint must be 20 bytes"},
		"not the node's operator":  {other, validRelay(t, other.Address, id, rsaID, priv), "operator does not own node"},
		"unknown node":             {op, validRelay(t, op.Address, "e2e-no-such-node", rsaID, priv), "e2e-no-such-node"},
		"empty node id":            {op, validRelay(t, op.Address, "", rsaID, priv), "node id is empty"},
		"another key's fingerprint, the squatter's own key": {op, edit(good, func(f *relayFields) {
			f.fingerprint = strangerRSA.fp
			f.ed25519Sig = crossCert(priv, id, strangerRSA.fp)
		}), "rsa identity does not prove the fingerprint"},
		"the right key, another key's signature": {op, edit(good, func(f *relayFields) { f.rsaSig = strangerRSA.proof(t, id, priv.Public().(ed25519.PublicKey)) }), "rsa identity does not prove the fingerprint"},
		"a proof made for another node":          {op, edit(good, func(f *relayFields) { f.rsaSig = rsaID.proof(t, id+"x", priv.Public().(ed25519.PublicKey)) }), "rsa identity does not prove the fingerprint"},
		"no rsa key":                             {op, edit(good, func(f *relayFields) { f.rsaPublicKey, f.rsaSig = nil, nil }), "rsa identity does not prove the fingerprint"},
	}
	for name, tc := range cases {
		chain.RequireRefused(t, name, c.Submit(t, tc.k, chain.TxOptions{}, tc.msg.register()), tc.want)
	}
	if out := c.QueryFails(t, op.Node, "relay", "relay", hex.EncodeToString(rsaID.fp)); !chain.NotFound(out) {
		t.Errorf("a refused fingerprint is registered: %s", out)
	}
}

// TestRelayRegister_needsTheRelayRoleAndTheRelayKey: a node without ROLE_RELAY
// is refused (a node's roles are fixed at registration, so there is no way to
// add it), and so is a relay node whose ed25519 key is bound under another
// service than "relay": x/relay reads the Tor identity from that binding only,
// never from another key of the node.
func TestRelayRegister_needsTheRelayRoleAndTheRelayKey(t *testing.T) {
	t.Parallel()
	c := chain.New(t)
	op := c.FundedValidator(t, chain.OperatorNode, chain.Orama(1))
	c.EnsureOperator(t, op)
	rsaID := newRSAIdentity(t)
	storageID, storageBindings := c.RegisterTestNode(t, op, []string{chain.RoleStorage}, "relay")
	storage := validRelay(t, op.Address, storageID, rsaID, storageBindings[0].Priv)
	chain.RequireRefused(t, "a storage node", c.Submit(t, op, chain.TxOptions{}, storage.register()), "does not have role ROLE_RELAY")

	otherID, otherBindings := c.RegisterTestNode(t, op, []string{chain.RoleRelay}, "tor")
	noKey := validRelay(t, op.Address, otherID, rsaID, otherBindings[0].Priv)
	chain.RequireRefused(t, "a relay node with no relay key", c.Submit(t, op, chain.TxOptions{}, noKey.register()), `binds no ed25519 key for service "relay"`)
	if out := c.QueryFails(t, op.Node, "relay", "relay", hex.EncodeToString(rsaID.fp)); !chain.NotFound(out) {
		t.Errorf("a refused fingerprint is registered: %s", out)
	}
}

// TestRelayUpdate_rotatesAFingerprint: MsgUpdateRelay replaces the fingerprint
// of the relay registered on a node (a Tor RSA key rotation) and its exit
// declaration; the old fingerprint stops being a registered relay, the new one
// is, and the operator's relay list follows. It is refused for a node with no
// relay, a fingerprint another relay holds, a cross-signature by another key,
// an RSA identity that cannot prove the new fingerprint, and an operator that
// does not own the node.
func TestRelayUpdate_rotatesAFingerprint(t *testing.T) {
	t.Parallel()
	c := chain.New(t)
	op := c.FundedValidator(t, chain.OperatorNode, chain.Orama(1))
	c.EnsureOperator(t, op)
	other := c.FundedValidator(t, 1, chain.Orama(1))
	c.EnsureOperator(t, other)
	id, bindings := c.RegisterTestNode(t, op, []string{chain.RoleRelay}, "relay")
	bareID, bareBindings := c.RegisterTestNode(t, op, []string{chain.RoleRelay}, "relay")
	foreignID, foreignBindings := c.RegisterTestNode(t, other, []string{chain.RoleRelay}, "relay")
	priv := bindings[0].Priv
	first, foreign, rotated := newRSAIdentity(t), newRSAIdentity(t), newRSAIdentity(t)
	chain.RequireOK(t, "register the relay", c.Submit(t, op, chain.TxOptions{}, validRelay(t, op.Address, id, first, priv).register()))
	chain.RequireOK(t, "register another operator's relay", c.Submit(t, other, chain.TxOptions{}, validRelay(t, other.Address, foreignID, foreign, foreignBindings[0].Priv).register()))

	_, stranger, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	edit := func(f relayFields, with func(*relayFields)) relayFields {
		with(&f)
		return f
	}
	good := validRelay(t, op.Address, id, rotated, priv)
	refusals := map[string]struct {
		k    chain.Key
		msg  relayFields
		want string
	}{
		"a node with no relay":            {op, validRelay(t, op.Address, bareID, rotated, bareBindings[0].Priv), "no relay is registered on the node"},
		"a fingerprint another relay has": {op, validRelay(t, op.Address, id, foreign, priv), "fingerprint already registered"},
		"signed by another key":           {op, edit(good, func(f *relayFields) { f.ed25519Sig = crossCert(stranger, id, rotated.fp) }), "does not match the ed25519 cross-signature"},
		"not the node's operator":         {other, validRelay(t, other.Address, id, rotated, priv), "operator does not own node"},
		"19-byte fingerprint":             {op, edit(good, func(f *relayFields) { f.fingerprint = rotated.fp[:19] }), "rsa fingerprint must be 20 bytes"},
		"an rsa key that cannot prove it": {op, edit(good, func(f *relayFields) { f.rsaSig = first.proof(t, id, priv.Public().(ed25519.PublicKey)) }), "rsa identity does not prove the fingerprint"},
	}
	for name, tc := range refusals {
		chain.RequireRefused(t, name, c.Submit(t, tc.k, chain.TxOptions{}, tc.msg.update()), tc.want)
	}
	if out := c.QueryOut(t, op.Node, "relay", "relay", hex.EncodeToString(first.fp)); out.Exit != 0 {
		t.Fatalf("a refused update removed the relay: %s", out.Stderr)
	}

	rotation := validRelay(t, op.Address, id, rotated, priv)
	rotation.exit = true
	chain.RequireOK(t, "rotate the fingerprint", c.Submit(t, op, chain.TxOptions{}, rotation.update()))
	if out := c.QueryFails(t, op.Node, "relay", "relay", hex.EncodeToString(first.fp)); !chain.NotFound(out) {
		t.Errorf("the replaced fingerprint is still registered: %s", out)
	}
	var v struct {
		Relay struct {
			NodeID string `json:"node_id"`
			Exit   bool   `json:"exit"`
		} `json:"relay"`
	}
	c.Query(t, op.Node, &v, "relay", "relay", hex.EncodeToString(rotated.fp))
	if v.Relay.NodeID != id || !v.Relay.Exit {
		t.Errorf("rotated relay %+v, want node %s declaring exit", v.Relay, id)
	}
	var list struct {
		Relays []struct {
			Relay struct {
				NodeID string `json:"node_id"`
			} `json:"relay"`
			LastPayoutEpoch  string `json:"last_payout_epoch"`
			LastPayoutAmount string `json:"last_payout_amount"`
		} `json:"relays"`
	}
	c.Query(t, op.Node, &list, "relay", "operator-relays", op.Address)
	var listed bool
	for _, r := range list.Relays {
		if r.Relay.NodeID == id {
			listed = true
			if r.LastPayoutEpoch != "" && r.LastPayoutEpoch != "0" || r.LastPayoutAmount != "0" {
				t.Errorf("a relay never paid lists a payout: %+v", r)
			}
		}
		if r.Relay.NodeID == foreignID {
			t.Errorf("operator %s lists the relay of node %s, which belongs to %s", op.Address, foreignID, other.Address)
		}
	}
	if !listed {
		t.Errorf("operator %s does not list its relay on node %s: %+v", op.Address, id, list.Relays)
	}
	c.RequireInvariants(t, "a relay rotation")
}

// TestRelayPayouts_anUnsettledEpochPaidNobody: the payouts of an epoch no relay
// was reported on are empty, not an error, and epoch 0 is refused. (The run
// chain has no reporter set, so no epoch pays; the paid case is checked on
// stagenet by relay-reporter.)
func TestRelayPayouts_anUnsettledEpochPaidNobody(t *testing.T) {
	t.Parallel()
	c := chain.New(t)
	n := c.Node(t, chain.OperatorNode)
	var v struct {
		Payouts []any `json:"payouts"`
	}
	c.Query(t, n, &v, "relay", "payouts", "1")
	if len(v.Payouts) != 0 {
		t.Errorf("epoch 1 paid %v on a chain with no reporter", v.Payouts)
	}
	c.QueryFails(t, n, "relay", "payouts", "0")
}

// TestRelayReporters_setIsClosedToMessages: the reporter set changes only
// inside a passed structural proposal's execution (x/relay keeper
// reporters.go: the flag is never set by a message), so MsgUpdateReporters
// is refused for any signer; the run's reporter set is the genesis one
// (empty), so MsgReportEpoch from a validator is refused as not a reporter,
// and no epoch has a relay result (the relay end block writes none for an
// epoch nobody reported on, however many epochs the run chain has closed).
func TestRelayReporters_setIsClosedToMessages(t *testing.T) {
	t.Parallel()
	c := chain.New(t)
	k := c.FundedValidator(t, 1, chain.Orama(1))
	upd := chain.NewMsg("/orama.relay.v1.MsgUpdateReporters", map[string]any{"signer": k.Address, "reporters": []string{k.Address}})
	chain.RequireRefused(t, "update reporters", c.Submit(t, k, chain.TxOptions{}, upd), "reporter set changes only when")
	var rep struct {
		Reporters []string `json:"reporters"`
	}
	c.Query(t, k.Node, &rep, "relay", "reporters")
	if len(rep.Reporters) != 0 {
		t.Fatalf("reporters %v on the run chain, want the empty genesis set", rep.Reporters)
	}
	report := chain.NewMsg("/orama.relay.v1.MsgReportEpoch", map[string]any{"reporter": k.Address, "epoch": "1",
		"chunk_index": 0, "chunk_count": 1, "entries": []any{}, "inputs_root": base64.StdEncoding.EncodeToString(make([]byte, inputsRootLen))})
	chain.RequireRefused(t, "report by a non-reporter", c.Submit(t, k, chain.TxOptions{}, report), "not a reporter")
	var params struct {
		Params map[string]any `json:"params"`
	}
	c.Query(t, k.Node, &params, "relay", "params")
	if len(params.Params) == 0 {
		t.Errorf("relay params are empty")
	}
	if out := c.QueryFails(t, k.Node, "relay", "epoch", "1"); !chain.NotFound(out) {
		t.Errorf("epoch 1 has a relay result on a chain with no reporter: %s", out)
	}
}
