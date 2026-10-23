// Package apple verifies Apple App Attest proofs: the attestation that binds
// a new key to a genuine build of an app on a genuine Apple device, and the
// assertions that key signs afterwards.
//
// It follows Apple's "Validating apps that connect to your server" step by
// step, including the iOS 27 extensions (apple_bundle_version_01,
// apple_validation_category_01) signed into the authenticator data. The
// certificate chain ends at the Apple App Attestation Root CA, compiled in.
package apple

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"crypto/x509"
	_ "embed"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/cryptobyte"
	cbasn1 "golang.org/x/crypto/cryptobyte/asn1"

	"github.com/Orama-Network/orama/core/pkg/attest/refusal"
)

// formatAppAttest is the attestation statement format App Attest uses.
const formatAppAttest = "apple-appattest"

// p256PointLen is an X9.62 uncompressed P-256 point: 0x04, X, Y.
const p256PointLen = 1 + 2*p256CoordLen

// p256Curve is the curve App Attest keys are on.
var p256Curve = elliptic.P256()

// oidNonce is the credential certificate extension holding the nonce.
var oidNonce = []int{1, 2, 840, 113635, 100, 8, 2}

//go:embed apple_app_attestation_root_ca.pem
var rootPEM []byte

// Verifier verifies App Attest proofs. The zero value is not usable; build
// one with New, or set Roots and Now directly in tests.
type Verifier struct {
	// Roots are the trusted roots: the Apple App Attestation Root CA.
	Roots *x509.CertPool
	// Now is the time certificates must be valid at.
	Now func() time.Time
}

// New returns a Verifier trusting the pinned Apple App Attestation Root CA.
func New() *Verifier {
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(rootPEM) {
		panic("apple: the compiled-in App Attestation root does not parse")
	}
	return &Verifier{Roots: pool, Now: time.Now}
}

// Expect is what a proof must show about the app.
type Expect struct {
	// AppID is "TEAMID.bundle.id".
	AppID string
	// AllowDevelopment accepts keys from the development environment.
	AllowDevelopment bool
}

// Attestation is what a verified attestation proves.
type Attestation struct {
	// PublicKey is the attested P-256 key, X9.62 uncompressed.
	PublicKey []byte
	// Development is true for a development-environment key.
	Development bool
	// Receipt is Apple's receipt, for a later fraud-metric request.
	Receipt []byte
	Extensions
}

// attestationObject is the CBOR object DCAppAttestService returns.
type attestationObject struct {
	Fmt      string  `cbor:"fmt"`
	AttStmt  attStmt `cbor:"attStmt"`
	AuthData []byte  `cbor:"authData"`
}

type attStmt struct {
	X5C     [][]byte `cbor:"x5c"`
	Receipt []byte   `cbor:"receipt"`
}

// VerifyAttestation verifies an attestation object for keyID, made over
// clientDataHash: the bytes the app passed to attestKey, which Apple
// specifies as the SHA-256 of the client data.
func (v *Verifier) VerifyAttestation(want Expect, keyID, object []byte, clientDataHash []byte) (Attestation, error) {
	var out Attestation
	var obj attestationObject
	if err := strict.Unmarshal(object, &obj); err != nil {
		return out, refusal.Invalid(refusal.ReasonMalformed, "attestation object: %w", err)
	}
	if obj.Fmt != formatAppAttest {
		return out, refusal.Invalid(refusal.ReasonMalformed, "attestation format %q, want %q", obj.Fmt, formatAppAttest)
	}
	leaf, err := v.verifyChain(obj.AttStmt.X5C)
	if err != nil {
		return out, err
	}

	nonce := sha256.Sum256(append(append([]byte{}, obj.AuthData...), clientDataHash...))
	certNonce, err := extractNonce(leaf)
	if err != nil {
		return out, refusal.Invalid(refusal.ReasonMalformed, "credential certificate: %w", err)
	}
	if !bytes.Equal(certNonce, nonce[:]) {
		return out, refusal.Invalid(refusal.ReasonNonce, "the certificate's nonce does not cover this authenticator data and client data")
	}

	point, err := p256Point(leaf.PublicKey)
	if err != nil {
		return out, refusal.Invalid(refusal.ReasonMalformed, "credential certificate: %w", err)
	}
	if h := sha256.Sum256(point); !bytes.Equal(h[:], keyID) {
		return out, refusal.Invalid(refusal.ReasonKeyID, "the certificate's key is not the key id the app sent")
	}

	ad, err := parseAttestationAuthData(obj.AuthData)
	if err != nil {
		return out, refusal.Invalid(refusal.ReasonMalformed, "authenticator data: %w", err)
	}
	if appHash := sha256.Sum256([]byte(want.AppID)); ad.rpIDHash != appHash {
		return out, refusal.Invalid(refusal.ReasonAppID, "the key belongs to another app than %s", want.AppID)
	}
	if ad.counter != 0 {
		return out, refusal.Invalid(refusal.ReasonCounter, "attestation counter is %d, want 0", ad.counter)
	}
	switch ad.aaguid {
	case aaguidProduction:
	case aaguidDevelopment:
		if !want.AllowDevelopment {
			return out, refusal.Invalid(refusal.ReasonEnvironment, "the key is from the App Attest development environment")
		}
		out.Development = true
	default:
		return out, refusal.Invalid(refusal.ReasonEnvironment, "aaguid %x is neither App Attest environment", ad.aaguid)
	}
	if !bytes.Equal(ad.credentialID, keyID) {
		return out, refusal.Invalid(refusal.ReasonKeyID, "the credential id is not the key id the app sent")
	}
	if err := coseKeyMatches(ad.coseKey, point); err != nil {
		return out, refusal.Invalid(refusal.ReasonKeyID, "%w", err)
	}

	out.PublicKey = point
	out.Receipt = obj.AttStmt.Receipt
	out.Extensions = ad.ext
	return out, nil
}

// verifyChain verifies x5c (credential certificate first) up to the roots and
// returns the credential certificate.
func (v *Verifier) verifyChain(x5c [][]byte) (*x509.Certificate, error) {
	if len(x5c) < 2 {
		return nil, refusal.Invalid(refusal.ReasonChain, "x5c has %d certificates, want the credential certificate and an intermediate", len(x5c))
	}
	certs := make([]*x509.Certificate, len(x5c))
	for i, der := range x5c {
		c, err := x509.ParseCertificate(der)
		if err != nil {
			return nil, refusal.Invalid(refusal.ReasonMalformed, "x5c[%d]: %w", i, err)
		}
		certs[i] = c
	}
	intermediates := x509.NewCertPool()
	for _, c := range certs[1:] {
		intermediates.AddCert(c)
	}
	_, err := certs[0].Verify(x509.VerifyOptions{
		Roots:         v.Roots,
		Intermediates: intermediates,
		CurrentTime:   v.Now(),
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
	})
	if err != nil {
		return nil, refusal.Invalid(refusal.ReasonChain, "credential certificate does not chain to the App Attestation root: %w", err)
	}
	return certs[0], nil
}

// extractNonce reads the nonce extension: SEQUENCE { [1] EXPLICIT OCTET STRING }.
func extractNonce(c *x509.Certificate) ([]byte, error) {
	for _, ext := range c.Extensions {
		if !ext.Id.Equal(oidNonce) {
			continue
		}
		in := cryptobyte.String(ext.Value)
		var seq, tagged, nonce cryptobyte.String
		if !in.ReadASN1(&seq, cbasn1.SEQUENCE) || !in.Empty() ||
			!seq.ReadASN1(&tagged, cbasn1.Tag(1).Constructed().ContextSpecific()) || !seq.Empty() ||
			!tagged.ReadASN1(&nonce, cbasn1.OCTET_STRING) || !tagged.Empty() {
			return nil, errors.New("nonce extension is not SEQUENCE { [1] OCTET STRING }")
		}
		return nonce, nil
	}
	return nil, errors.New("no nonce extension")
}

// p256Point returns a P-256 public key as an X9.62 uncompressed point.
func p256Point(pub any) ([]byte, error) {
	k, ok := pub.(*ecdsa.PublicKey)
	if !ok || k.Curve != p256Curve {
		return nil, fmt.Errorf("public key is %T, not a P-256 key", pub)
	}
	point, err := k.Bytes()
	if err != nil {
		return nil, err
	}
	return point, nil
}
