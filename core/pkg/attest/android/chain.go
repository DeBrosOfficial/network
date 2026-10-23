package android

import (
	"bytes"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"fmt"
	"time"

	"github.com/Orama-Network/orama/core/pkg/attest/refusal"
)

// minChainLen is the shortest hardware chain: the key's certificate, the
// attestation certificate, and the root.
const minChainLen = 3

var (
	// oidKeyDescription is the key attestation extension.
	oidKeyDescription = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 11129, 2, 1, 17}
	// oidSerialNumber is the X.520 serialNumber attribute, which names
	// factory-provisioned intermediates.
	oidSerialNumber = asn1.ObjectIdentifier{2, 5, 4, 5}
)

// verifyChain checks chain (leaf first, root last) the way Google's
// reference verifier (github.com/android/keyattestation) does:
//   - the root's public key is one of the pinned roots;
//   - each certificate names its parent as issuer and is signed by it;
//   - every certificate but the leaf is within its validity period at now,
//     except that expiry is ignored on a factory-provisioned chain, whose
//     keys cannot be rotated (the leaf's dates are set by the device);
//   - only the leaf carries the key attestation extension, so a chain
//     extended below a genuine attestation key is refused.
//
// It returns the parsed certificates.
func verifyChain(chain [][]byte, pinned [][]byte, now time.Time) ([]*x509.Certificate, error) {
	if len(chain) < minChainLen {
		return nil, refusal.Invalid(refusal.ReasonChain, "chain has %d certificates, want at least %d", len(chain), minChainLen)
	}
	certs := make([]*x509.Certificate, len(chain))
	for i, der := range chain {
		c, err := x509.ParseCertificate(der)
		if err != nil {
			return nil, refusal.Invalid(refusal.ReasonMalformed, "certificate %d: %w", i, err)
		}
		certs[i] = c
	}
	root := certs[len(certs)-1]
	if !isPinned(root.RawSubjectPublicKeyInfo, pinned) {
		return nil, refusal.Invalid(refusal.ReasonChain, "chain ends at %q, not a Google attestation root", root.Subject)
	}
	factory := hasSerialNumberAttribute(certs[len(certs)-2].Subject)
	for i := len(certs) - 2; i >= 0; i-- {
		c, parent := certs[i], certs[i+1]
		if !bytes.Equal(c.RawIssuer, parent.RawSubject) {
			return nil, refusal.Invalid(refusal.ReasonChain, "certificate %d is issued by %q, not by the next certificate %q", i, c.Issuer, parent.Subject)
		}
		if err := parent.CheckSignature(c.SignatureAlgorithm, c.RawTBSCertificate, c.Signature); err != nil {
			return nil, refusal.Invalid(refusal.ReasonChain, "certificate %d is not signed by its issuer: %w", i, err)
		}
		if i > 0 {
			if now.Before(c.NotBefore) {
				return nil, refusal.Invalid(refusal.ReasonChain, "certificate %d is not valid before %s", i, c.NotBefore.Format(time.RFC3339))
			}
			if now.After(c.NotAfter) && !factory {
				return nil, refusal.Invalid(refusal.ReasonChain, "certificate %d expired at %s", i, c.NotAfter.Format(time.RFC3339))
			}
		}
		if i > 0 && hasKeyDescription(c) {
			return nil, refusal.Invalid(refusal.ReasonChain, "certificate %d above the leaf carries a key attestation extension", i)
		}
	}
	if !hasKeyDescription(certs[0]) {
		return nil, refusal.Invalid(refusal.ReasonMalformed, "the leaf certificate has no key attestation extension")
	}
	return certs, nil
}

func isPinned(spki []byte, pinned [][]byte) bool {
	for _, p := range pinned {
		if bytes.Equal(spki, p) {
			return true
		}
	}
	return false
}

func hasSerialNumberAttribute(name pkix.Name) bool {
	for _, a := range name.Names {
		if a.Type.Equal(oidSerialNumber) {
			return true
		}
	}
	return false
}

func hasKeyDescription(c *x509.Certificate) bool {
	_, ok := keyDescriptionExtension(c)
	return ok
}

func keyDescriptionExtension(c *x509.Certificate) ([]byte, bool) {
	for _, ext := range c.Extensions {
		if ext.Id.Equal(oidKeyDescription) {
			return ext.Value, true
		}
	}
	return nil, false
}

// serialHex is a certificate serial as Google's revocation list keys it:
// lowercase hex without leading zeros.
func serialHex(c *x509.Certificate) string {
	return fmt.Sprintf("%x", c.SerialNumber)
}
