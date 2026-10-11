package types_test

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1" // #nosec G505 -- a Tor relay fingerprint is SHA-1 by definition
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/DeBrosOfficial/network/chain/x/relay/types"
)

// rsaProofGoldenHex is RSAProofMessage("node-a", 32 x 0x55): the domain, a zero, the node id, a zero and
// the key. core/pkg/clusterreg's RelayRSAProofMessage test asserts the same constant.
const rsaProofGoldenHex = "6f72616d612f72656c61792f7273612d70726f6f662f7631" + "00" + "6e6f64652d61" + "00" +
	"5555555555555555555555555555555555555555555555555555555555555555"

func TestRSAProofMessage_isPinned(t *testing.T) {
	require.Equal(t, rsaProofGoldenHex, hex.EncodeToString(types.RSAProofMessage("node-a", bytes.Repeat([]byte{0x55}, 32))))
}

type identity struct {
	key *rsa.PrivateKey
	der []byte
	fp  []byte
}

func newIdentity(t *testing.T) identity {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, types.MinRSABits)
	require.NoError(t, err)
	der := x509.MarshalPKCS1PublicKey(&key.PublicKey)
	sum := sha1.Sum(der)
	return identity{key: key, der: der, fp: sum[:]}
}

func (i identity) sign(t *testing.T, nodeID string, ed25519Pub []byte) []byte {
	t.Helper()
	sum := sha256.Sum256(types.RSAProofMessage(nodeID, ed25519Pub))
	sig, err := rsa.SignPKCS1v15(rand.Reader, i.key, crypto.SHA256, sum[:])
	require.NoError(t, err)
	return sig
}

func TestVerifyRSAProof(t *testing.T) {
	owner, stranger := newIdentity(t), newIdentity(t)
	pub := bytes.Repeat([]byte{0x55}, 32)
	good := owner.sign(t, "node-a", pub)
	require.NoError(t, types.VerifyRSAProof(owner.fp, owner.der, pub, "node-a", good), "the key's holder proves its fingerprint")

	cases := map[string]struct {
		fp, der, pub []byte
		node         string
		sig          []byte
	}{
		"another key's fingerprint":      {stranger.fp, owner.der, pub, "node-a", good},
		"another key's public key":       {owner.fp, stranger.der, pub, "node-a", good},
		"another key's signature":        {owner.fp, owner.der, pub, "node-a", stranger.sign(t, "node-a", pub)},
		"a signature for another node":   {owner.fp, owner.der, pub, "node-b", good},
		"a signature for another key":    {owner.fp, owner.der, bytes.Repeat([]byte{0x66}, 32), "node-a", good},
		"no public key":                  {owner.fp, nil, pub, "node-a", good},
		"a public key over the bound":    {owner.fp, make([]byte, types.MaxRSAPublicKeyLen+1), pub, "node-a", good},
		"a public key with a byte added": {owner.fp, append(append([]byte(nil), owner.der...), 0), pub, "node-a", good},
		"no signature":                   {owner.fp, owner.der, pub, "node-a", nil},
		"a signature one byte short":     {owner.fp, owner.der, pub, "node-a", good[:len(good)-1]},
	}
	for name, c := range cases {
		err := types.VerifyRSAProof(c.fp, c.der, c.pub, c.node, c.sig)
		require.ErrorIs(t, err, types.ErrRSAProofMismatch, name)
	}
}

func TestVerifyRSAProof_aKeyThatHashesRightButIsNotAnRSAKeyIsRefused(t *testing.T) {
	junk := []byte("not an rsa key at all")
	sum := sha1.Sum(junk)
	err := types.VerifyRSAProof(sum[:], junk, make([]byte, 32), "node-a", []byte("sig"))
	require.ErrorIs(t, err, types.ErrRSAProofMismatch)
}

// A key outside the sizes Tor and the chain accept is refused before any signature is checked, however
// its fingerprint was made: the modulus below is not a real key, only a number of the wrong size.
func TestVerifyRSAProof_aKeyOfTheWrongSizeIsRefused(t *testing.T) {
	for name, bits := range map[string]uint{"512 bits": 512, "4104 bits": 4104} {
		n := new(big.Int).Lsh(big.NewInt(1), bits-1)
		n.Add(n, big.NewInt(1))
		der := x509.MarshalPKCS1PublicKey(&rsa.PublicKey{N: n, E: 65537})
		sum := sha1.Sum(der)
		err := types.VerifyRSAProof(sum[:], der, make([]byte, 32), "node-a", make([]byte, bits/8))
		require.ErrorIs(t, err, types.ErrRSAProofMismatch, name)
		require.ErrorContains(t, err, "bits", name)
	}
}
