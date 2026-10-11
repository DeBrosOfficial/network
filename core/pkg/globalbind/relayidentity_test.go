package globalbind

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/DeBrosOfficial/network/pkg/clusterreg"
)

func torSecret() (expanded []byte, pub ed25519.PublicKey) {
	seed := bytes.Repeat([]byte{0x42}, 32)
	sum := sha512.Sum512(seed)
	return sum[:], ed25519.NewKeyFromSeed(seed).Public().(ed25519.PublicKey)
}

func TestRelayIdentity_roundTripsThroughTheNodesJSON(t *testing.T) {
	expanded, pub := torSecret()
	fingerprint := bytes.Repeat([]byte{0xab}, clusterreg.RelayFingerprintLen)
	id, err := NewRelayIdentity(expanded, fingerprint, vectorChain, vectorOperator, "node-a")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(id)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), strings.Repeat("AB", 20)) {
		t.Errorf("the fingerprint is not 40 upper-case hex digits: %s", raw)
	}
	got, err := ParseRelayIdentity(raw, vectorChain, vectorOperator, "node-a")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.Fingerprint, fingerprint) || !bytes.Equal(got.Binding.Pubkey, pub) || got.Binding.Service != clusterreg.RelayService {
		t.Errorf("parsed %+v", got)
	}
	if !bytes.Equal(got.Binding.Signature, id.Binding.Signature) || !bytes.Equal(got.CrossCert, id.CrossCert) {
		t.Error("signatures changed on the way")
	}
	if strings.Contains(string(raw), hex.EncodeToString(expanded)) {
		t.Error("the secret is in the output")
	}
}

func TestParseRelayIdentity_refusals(t *testing.T) {
	expanded, _ := torSecret()
	fingerprint := bytes.Repeat([]byte{0xab}, clusterreg.RelayFingerprintLen)
	id, err := NewRelayIdentity(expanded, fingerprint, vectorChain, vectorOperator, "node-a")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(id)
	var doc map[string]string
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	cases := map[string]struct {
		edit             func(map[string]string)
		chain, node, who string
	}{
		"another chain":            {func(map[string]string) {}, "other-chain", "node-a", vectorOperator},
		"another node":             {func(map[string]string) {}, vectorChain, "node-b", vectorOperator},
		"another operator":         {func(map[string]string) {}, vectorChain, "node-a", "orama1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqnr0x9w"},
		"a lower-case fingerprint": {func(d map[string]string) { d["rsa_fingerprint"] = strings.ToLower(d["rsa_fingerprint"]) }, vectorChain, "node-a", vectorOperator},
		"a short fingerprint":      {func(d map[string]string) { d["rsa_fingerprint"] = d["rsa_fingerprint"][:38] }, vectorChain, "node-a", vectorOperator},
		"another service":          {func(d map[string]string) { d["service"] = "tor" }, vectorChain, "node-a", vectorOperator},
		"a secp256k1 key":          {func(d map[string]string) { d["key_type"] = KeyTypeSecp256k1 }, vectorChain, "node-a", vectorOperator},
		"a cut cross-certificate":  {func(d map[string]string) { d["cross_cert"] = d["cross_cert"][:126] }, vectorChain, "node-a", vectorOperator},
		"a cross-certificate of a different fingerprint": {func(d map[string]string) {
			other, _ := NewRelayIdentity(expanded, bytes.Repeat([]byte{0xcd}, 20), vectorChain, vectorOperator, "node-a")
			o, _ := json.Marshal(other)
			var od map[string]string
			_ = json.Unmarshal(o, &od)
			d["cross_cert"] = od["cross_cert"]
		}, vectorChain, "node-a", vectorOperator},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			d := map[string]string{}
			for k, v := range doc {
				d[k] = v
			}
			c.edit(d)
			edited, _ := json.Marshal(d)
			if _, err := ParseRelayIdentity(edited, c.chain, c.who, c.node); err == nil {
				t.Fatal("the identity was accepted")
			}
		})
	}
	if _, err := ParseRelayIdentity([]byte("not json"), vectorChain, vectorOperator, "node-a"); err == nil {
		t.Error("text was accepted")
	}
}
