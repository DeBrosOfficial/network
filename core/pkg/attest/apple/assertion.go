package apple

import (
	"crypto/ecdsa"
	"crypto/sha256"

	"github.com/Orama-Network/orama/core/pkg/attest/refusal"
)

// Assertion is what a verified assertion proves.
type Assertion struct {
	// Counter is the assertion's counter, strictly above the stored one.
	Counter uint32
	Extensions
}

// assertionObject is the CBOR object generateAssertion returns.
type assertionObject struct {
	Signature         []byte `cbor:"signature"`
	AuthenticatorData []byte `cbor:"authenticatorData"`
}

// VerifyAssertion verifies an assertion signed by publicKey (X9.62
// uncompressed P-256, from a verified attestation) over clientDataHash. Its
// counter must be strictly greater than lastCounter.
func (v *Verifier) VerifyAssertion(want Expect, publicKey []byte, lastCounter uint32, object []byte, clientDataHash []byte) (Assertion, error) {
	var out Assertion
	if len(publicKey) != p256PointLen {
		return out, refusal.Invalid(refusal.ReasonMalformed, "stored public key is %d bytes, want %d", len(publicKey), p256PointLen)
	}
	pub, err := ecdsa.ParseUncompressedPublicKey(p256Curve, publicKey)
	if err != nil {
		return out, refusal.Invalid(refusal.ReasonMalformed, "stored public key: %w", err)
	}
	var obj assertionObject
	if err := strict.Unmarshal(object, &obj); err != nil {
		return out, refusal.Invalid(refusal.ReasonMalformed, "assertion object: %w", err)
	}

	// The key signs nonce with ECDSA over SHA-256, so the signed digest is
	// SHA-256(nonce).
	nonce := sha256.Sum256(append(append([]byte{}, obj.AuthenticatorData...), clientDataHash...))
	digest := sha256.Sum256(nonce[:])
	if !ecdsa.VerifyASN1(pub, digest[:], obj.Signature) {
		return out, refusal.Invalid(refusal.ReasonSignature, "the assertion is not signed by the attested key over this client data")
	}

	ad, err := parseAssertionAuthData(obj.AuthenticatorData)
	if err != nil {
		return out, refusal.Invalid(refusal.ReasonMalformed, "authenticator data: %w", err)
	}
	if appHash := sha256.Sum256([]byte(want.AppID)); ad.rpIDHash != appHash {
		return out, refusal.Invalid(refusal.ReasonAppID, "the assertion is for another app than %s", want.AppID)
	}
	if ad.counter <= lastCounter {
		return out, refusal.Invalid(refusal.ReasonCounter, "assertion counter %d is not above the last accepted %d", ad.counter, lastCounter)
	}
	out.Counter = ad.counter
	out.Extensions = ad.ext
	return out, nil
}
