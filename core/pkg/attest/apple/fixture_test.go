package apple

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/binary"
	"math/big"
	"testing"
	"time"

	"github.com/fxamacker/cbor/v2"
)

// fixtureNow is the time the generated fixture's certificates are valid at.
var fixtureNow = time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)

const fixtureAppID = "ABCDE12345.io.orama.test"

// fixture is a generated App Attest key and CA, so every refusal can be
// exercised with an injected root.
type fixture struct {
	t        *testing.T
	root     *x509.Certificate
	interKey *ecdsa.PrivateKey
	inter    *x509.Certificate
	key      *ecdsa.PrivateKey
	point    []byte
	keyID    []byte
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	rootKey := genKey(t, elliptic.P384())
	root := makeCert(t, &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Test App Attestation Root CA"},
		NotBefore: fixtureNow.Add(-time.Hour), NotAfter: fixtureNow.Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}, nil, rootKey, rootKey)
	interKey := genKey(t, elliptic.P384())
	inter := makeCert(t, &x509.Certificate{
		SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "Test App Attestation CA 1"},
		NotBefore: fixtureNow.Add(-time.Hour), NotAfter: fixtureNow.Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}, root, interKey, rootKey)
	key := genKey(t, elliptic.P256())
	point, err := key.PublicKey.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	id := sha256.Sum256(point)
	return &fixture{t: t, root: root, interKey: interKey, inter: inter, key: key, point: point, keyID: id[:]}
}

func (f *fixture) verifier() *Verifier {
	pool := x509.NewCertPool()
	pool.AddCert(f.root)
	return &Verifier{Roots: pool, Now: func() time.Time { return fixtureNow }}
}

func genKey(t *testing.T, c elliptic.Curve) *ecdsa.PrivateKey {
	t.Helper()
	k, err := ecdsa.GenerateKey(c, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func makeCert(t *testing.T, tmpl, parent *x509.Certificate, key, signer *ecdsa.PrivateKey) *x509.Certificate {
	t.Helper()
	if parent == nil {
		parent = tmpl
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, parent, &key.PublicKey, signer)
	if err != nil {
		t.Fatal(err)
	}
	c, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// attestation describes one generated attestation; zero fields take the
// genuine value.
type attestation struct {
	appID        string
	counter      uint32
	aaguid       *[aaguidLen]byte
	credentialID []byte
	coseKey      []byte
	flags        byte
	extensions   []byte
	nonce        []byte
}

func (f *fixture) coseKey() []byte {
	b, err := cbor.Marshal(coseEC2Key{Kty: coseKtyEC2, Alg: coseAlgES256, Curve: coseCrvP256, X: f.point[1:33], Y: f.point[33:]})
	if err != nil {
		f.t.Fatal(err)
	}
	return b
}

func (f *fixture) authData(a attestation) []byte {
	appID := a.appID
	if appID == "" {
		appID = fixtureAppID
	}
	rp := sha256.Sum256([]byte(appID))
	flags := a.flags
	if flags == 0 {
		flags = flagAttested
	}
	out := append([]byte{}, rp[:]...)
	out = append(out, flags)
	out = binary.BigEndian.AppendUint32(out, a.counter)
	aaguid := aaguidProduction
	if a.aaguid != nil {
		aaguid = *a.aaguid
	}
	out = append(out, aaguid[:]...)
	credID := a.credentialID
	if credID == nil {
		credID = f.keyID
	}
	out = binary.BigEndian.AppendUint16(out, uint16(len(credID)))
	out = append(out, credID...)
	key := a.coseKey
	if key == nil {
		key = f.coseKey()
	}
	out = append(out, key...)
	return append(out, a.extensions...)
}

// object builds the attestation object over clientDataHash.
func (f *fixture) object(a attestation, clientDataHash []byte) []byte {
	ad := f.authData(a)
	nonce := a.nonce
	if nonce == nil {
		n := sha256.Sum256(append(append([]byte{}, ad...), clientDataHash...))
		nonce = n[:]
	}
	leaf := f.leaf(nonce)
	obj, err := cbor.Marshal(attestationObject{
		Fmt:      formatAppAttest,
		AttStmt:  attStmt{X5C: [][]byte{leaf.Raw, f.inter.Raw}, Receipt: []byte("receipt")},
		AuthData: ad,
	})
	if err != nil {
		f.t.Fatal(err)
	}
	return obj
}

func (f *fixture) leaf(nonce []byte) *x509.Certificate {
	ext, err := asn1.Marshal(struct {
		Nonce []byte `asn1:"tag:1,explicit"`
	}{nonce})
	if err != nil {
		f.t.Fatal(err)
	}
	return makeCert(f.t, &x509.Certificate{
		SerialNumber: big.NewInt(3), Subject: pkix.Name{CommonName: "leaf"},
		NotBefore: fixtureNow.Add(-time.Hour), NotAfter: fixtureNow.Add(time.Hour),
		ExtraExtensions: []pkix.Extension{{Id: oidNonce, Value: ext}},
	}, f.inter, f.key, f.interKey)
}

// assertion builds an assertion with authenticator data ad over clientDataHash.
func (f *fixture) assertion(ad, clientDataHash []byte) []byte {
	nonce := sha256.Sum256(append(append([]byte{}, ad...), clientDataHash...))
	digest := sha256.Sum256(nonce[:])
	sig, err := ecdsa.SignASN1(rand.Reader, f.key, digest[:])
	if err != nil {
		f.t.Fatal(err)
	}
	obj, err := cbor.Marshal(assertionObject{Signature: sig, AuthenticatorData: ad})
	if err != nil {
		f.t.Fatal(err)
	}
	return obj
}

func assertionAuthData(appID string, flags byte, counter uint32, ext []byte) []byte {
	rp := sha256.Sum256([]byte(appID))
	out := append([]byte{}, rp[:]...)
	out = append(out, flags)
	out = binary.BigEndian.AppendUint32(out, counter)
	return append(out, ext...)
}

func mustCBOR(t *testing.T, v any) []byte {
	t.Helper()
	b, err := cbor.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
