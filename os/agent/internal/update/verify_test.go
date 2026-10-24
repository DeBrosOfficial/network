package update

import (
	"encoding/hex"
	"strings"
	"testing"
)

// The vectors were made with go-ethereum (crypto.Sign over accounts.TextHash,
// v += 27) by a throwaway key, keccak256("orama-agent-update-test"), which
// signs nothing real. A and B have v = 28, the recovery id the old
// hand-rolled recovery misread as "x = r + N" and refused every time; C has
// v = 27.
const (
	vectorSigner = "0x80B4eE639C774Ca7Ec8403C72Bd85Cd9831cE92D"
	vectorMsgA   = "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
	vectorSigA   = "0x52532e208ae70cb076a2ba0d11691ac45e3eaae9af0032a40bcb9634386e27851e6e804c3a71b8ad93c48853afdac5d26b1c38ac6fa1362e606d92b2ae0555751c"
	vectorMsgB   = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	vectorSigB   = "0x2568085c1f5c3ec4a1b3447619254a4e071a3c2ddfa15ed9c42d47aef67926bf397051d279e579f6ef8e3775a9fff948ea37c240832db44db60d527a775632511c"
	vectorMsgC   = "baa5a0964d3320fbc0c6a922140453c8513ea24ab8fd0577034804a967248096"
	vectorSigC   = "0x09d3422bd3c561983dfc159a414a12d782ce86f750d39a24d774de997cf67e3748698fc982a864b7e22f3537f5485f5a934ce2a9be7b5af7cf937882f2e0b4b21b"
)

func TestRecoverSigner_recoversTheKeyThatSigned(t *testing.T) {
	for msg, sig := range map[string]string{vectorMsgA: vectorSigA, vectorMsgB: vectorSigB, vectorMsgC: vectorSigC} {
		got, err := recoverSigner(msg, sig)
		if err != nil {
			t.Fatalf("%s: %v", msg, err)
		}
		if !strings.EqualFold(got, vectorSigner) {
			t.Errorf("%s: recovered %s, want %s", msg, got, vectorSigner)
		}
	}
}

// Signers write v as 27/28 (personal_sign) or 0/1; both name the same key, and
// the 0x prefix is optional.
func TestRecoverSigner_acceptsBothSpellingsOfV(t *testing.T) {
	raw := strings.TrimPrefix(vectorSigA, "0x")
	zeroBased := raw[:128] + "01"
	got, err := recoverSigner(vectorMsgA, zeroBased)
	if err != nil || !strings.EqualFold(got, vectorSigner) {
		t.Errorf("v=1 without 0x: recovered %s (%v), want %s", got, err, vectorSigner)
	}
}

// A signature moved onto another message does not name the signer: whatever
// it recovers is someone else.
func TestRecoverSigner_anotherMessageIsAnotherSigner(t *testing.T) {
	got, err := recoverSigner(vectorMsgB, vectorSigA)
	if err == nil && strings.EqualFold(got, vectorSigner) {
		t.Fatal("a signature over one hash verified over another")
	}
}

func TestRecoverSigner_refusesMalformedSignatures(t *testing.T) {
	raw := strings.TrimPrefix(vectorSigA, "0x")
	zeroR := strings.Repeat("00", 32) + raw[64:]
	for name, sig := range map[string]string{
		"not hex":          "0xzz" + raw[4:],
		"short":            raw[:128],
		"long":             raw + "00",
		"recovery id 2":    raw[:128] + "1d",
		"recovery id 0x7f": raw[:128] + "7f",
		"r is zero":        zeroR,
		"empty":            "",
	} {
		if got, err := recoverSigner(vectorMsgA, sig); err == nil {
			t.Errorf("%s: recovered %s, want an error", name, got)
		}
	}
}

// Only SignerAddress may sign an update; a valid signature by anyone else is
// refused and the refusal names who signed.
func TestVerifySignature_onlyTheReleaseKey(t *testing.T) {
	err := verifySignature(vectorMsgA, vectorSigA)
	if err == nil {
		t.Fatal("an update signed by a key that is not SignerAddress verified")
	}
	if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(vectorSigner)) {
		t.Errorf("refusal %q does not name the signer", err)
	}
	if err := verifySignature(vectorMsgA, "0x00"); err == nil {
		t.Error("a malformed signature verified")
	}
}

func TestPersonalSignHash_isTheEthereumPrefixedKeccak(t *testing.T) {
	// keccak256("\x19Ethereum Signed Message:\n5hello"), as eth_sign/personal_sign compute it.
	const want = "50b2c43fd39106bafbba0da34fc430e1f91e3c96ea2acee2bc34119f92b37750"
	if got := hex.EncodeToString(personalSignHash("hello")); got != want {
		t.Errorf("personalSignHash(hello) = %s, want %s", got, want)
	}
}
