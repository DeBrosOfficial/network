package types

import (
	"bytes"
	"encoding/hex"
	"testing"

	secp256k1 "github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
)

// bindingVectorSig is the signature core/pkg/globalbind.SignSecp256k1 produces
// for secret 0x11 repeated, chain orama-testnet-2, the vector operator, and
// service "provider". Both sides must stay on it.
const bindingVectorSig = "aa659956e92943f40a10a6239f18a666171e3add0d4e99ad8146f5e79541f6cf6089cbd369b45f415d4ca404d6fda34faa0a96c032485fedcb3863ea675885cf"

func TestBindingVector_matchesCoreSigner(t *testing.T) {
	secret := bytes.Repeat([]byte{0x11}, 32)
	priv := &secp256k1.PrivKey{Key: secret}
	pub := priv.PubKey().Bytes()
	const (
		chainID  = "orama-testnet-2"
		operator = "orama19rl4cm2hmr8afy4kldpxz3fka4jguq0a5tup0s"
		service  = "provider"
	)
	sig, err := priv.Sign(BindingSignBytes(chainID, operator, service, pub))
	if err != nil {
		t.Fatal(err)
	}
	if hex.EncodeToString(sig) != bindingVectorSig {
		t.Fatalf("sig %x", sig)
	}
	if err := VerifyBinding(chainID, operator, Binding{
		Service: service, KeyType: KeyTypeSecp256k1, Pubkey: pub, Signature: sig,
	}); err != nil {
		t.Fatal(err)
	}
}
