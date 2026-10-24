package android

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"math/big"
	"testing"
	"time"

	"github.com/Orama-Network/orama/core/pkg/attest/refusal"
)

var (
	genNow       = time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	genChallenge = []byte("server-challenge")
	genPackage   = "io.orama.anchat"
	genDigest    = []byte("0123456789abcdef0123456789abcdef")
	genBootKey   = []byte("fedcba9876543210fedcba9876543210")
)

// desc describes a generated key description; the zero value is a genuine
// TEE key of genPackage on a locked, Verified device.
type desc struct {
	attLevel, kmLevel asn1.Enumerated
	levelsSet         bool
	challenge         []byte
	packages          []string
	digests           [][]byte
	bootState         asn1.Enumerated
	unlocked          bool
	rotInSoftware     bool
	patch             int64
	trailing          []byte
}

type pkgInfo struct {
	Name    []byte
	Version int64
}

type appID struct {
	Packages []pkgInfo `asn1:"set"`
	Digests  [][]byte  `asn1:"set"`
}

type rootOfTrust struct {
	Key    []byte
	Locked bool
	State  asn1.Enumerated
	Hash   []byte
}

func mustDER(t *testing.T, v any) []byte {
	t.Helper()
	b, err := asn1.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// explicit encodes [tag] EXPLICIT content, in the high-tag-number form for
// tags past 30.
func explicit(tag int, content []byte) []byte {
	var out []byte
	if tag < 0x1f {
		out = []byte{0xa0 | byte(tag)}
	} else {
		var digits []byte
		for n := tag; n > 0; n >>= 7 {
			digits = append([]byte{byte(n & 0x7f)}, digits...)
		}
		for i := range digits[:len(digits)-1] {
			digits[i] |= 0x80
		}
		out = append([]byte{0xbf}, digits...)
	}
	return append(append(out, derLength(len(content))...), content...)
}

func derLength(n int) []byte {
	if n < 0x80 {
		return []byte{byte(n)}
	}
	var b []byte
	for ; n > 0; n >>= 8 {
		b = append([]byte{byte(n)}, b...)
	}
	return append([]byte{0x80 | byte(len(b))}, b...)
}

func (d desc) der(t *testing.T) []byte {
	t.Helper()
	att, km := asn1.Enumerated(SecurityTEE), asn1.Enumerated(SecurityTEE)
	if d.levelsSet {
		att, km = d.attLevel, d.kmLevel
	}
	challenge := d.challenge
	if challenge == nil {
		challenge = genChallenge
	}
	pkgs := d.packages
	if pkgs == nil {
		pkgs = []string{genPackage}
	}
	digests := d.digests
	if digests == nil {
		digests = [][]byte{genDigest}
	}
	app := appID{Digests: digests}
	for _, p := range pkgs {
		app.Packages = append(app.Packages, pkgInfo{Name: []byte(p), Version: 42})
	}
	rot := explicit(tagRootOfTrust, mustDER(t, rootOfTrust{Key: genBootKey, Locked: !d.unlocked, State: d.bootState, Hash: make([]byte, 32)}))
	patch := d.patch
	if patch == 0 {
		patch = 202609
	}
	sw := explicit(tagAttestationApplicationID, mustDER(t, mustDER(t, app)))
	hw := append(append([]byte{}, rot...), explicit(tagOSPatchLevel, mustDER(t, patch))...)
	if d.rotInSoftware {
		sw = append(rot, sw...)
		hw = explicit(tagOSPatchLevel, mustDER(t, patch))
	}
	body := append([]byte{}, mustDER(t, int64(400))...)
	body = append(body, mustDER(t, att)...)
	body = append(body, mustDER(t, int64(400))...)
	body = append(body, mustDER(t, km)...)
	body = append(body, mustDER(t, challenge)...)
	body = append(body, mustDER(t, []byte{})...)
	body = append(body, mustDER(t, asn1.RawValue{Class: asn1.ClassUniversal, Tag: asn1.TagSequence, IsCompound: true, Bytes: sw})...)
	body = append(body, mustDER(t, asn1.RawValue{Class: asn1.ClassUniversal, Tag: asn1.TagSequence, IsCompound: true, Bytes: hw})...)
	body = append(body, d.trailing...)
	return mustDER(t, asn1.RawValue{Class: asn1.ClassUniversal, Tag: asn1.TagSequence, IsCompound: true, Bytes: body})
}

// pki is a generated root, RKP-style intermediate and attestation key.
type pki struct {
	t                    *testing.T
	rootKey, interKey    *ecdsa.PrivateKey
	attKey               *ecdsa.PrivateKey
	root, inter, attCert *x509.Certificate
}

func genKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func sign(t *testing.T, tmpl, parent *x509.Certificate, pub *ecdsa.PublicKey, signer *ecdsa.PrivateKey) *x509.Certificate {
	t.Helper()
	der, err := x509.CreateCertificate(rand.Reader, tmpl, parent, pub, signer)
	if err != nil {
		t.Fatal(err)
	}
	c, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func ca(serial int64, cn string, notBefore time.Time, ext ...pkix.Extension) *x509.Certificate {
	return &x509.Certificate{
		SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: cn, Organization: []string{"Test"}},
		NotBefore: notBefore, NotAfter: genNow.Add(24 * time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
		ExtraExtensions: ext,
	}
}

func newPKI(t *testing.T) *pki {
	p := &pki{t: t, rootKey: genKey(t), interKey: genKey(t), attKey: genKey(t)}
	rootTmpl := ca(1, "Test Attestation Root", genNow.Add(-time.Hour))
	p.root = sign(t, rootTmpl, rootTmpl, &p.rootKey.PublicKey, p.rootKey)
	p.inter = sign(t, ca(2, "Droid CA2", genNow.Add(-time.Hour)), p.root, &p.interKey.PublicKey, p.rootKey)
	p.attCert = sign(t, ca(3, "TEE attestation key", genNow.Add(-time.Hour)), p.inter, &p.attKey.PublicKey, p.interKey)
	return p
}

func (p *pki) leaf(ext []byte, signer *ecdsa.PrivateKey) []byte {
	key := genKey(p.t)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(4), Subject: pkix.Name{CommonName: "Android Keystore Key"},
		NotBefore: time.Unix(0, 0), NotAfter: genNow.Add(24 * time.Hour),
	}
	if ext != nil {
		tmpl.ExtraExtensions = []pkix.Extension{{Id: oidKeyDescription, Value: ext}}
	}
	// A parent with the attestation certificate's name but the signer's key,
	// so a forged signature can be made under the genuine issuer name.
	parent := *p.attCert
	parent.PublicKey = &signer.PublicKey
	return sign(p.t, tmpl, &parent, &key.PublicKey, signer).Raw
}

func (p *pki) chain(d desc) [][]byte {
	return [][]byte{p.leaf(d.der(p.t), p.attKey), p.attCert.Raw, p.inter.Raw, p.root.Raw}
}

func (p *pki) verifier(selfSigned keySet) *Verifier {
	return &Verifier{
		RootKeys:       [][]byte{p.root.RawSubjectPublicKeyInfo},
		Now:            func() time.Time { return genNow },
		Revocation:     fakeRevocations{},
		SelfSignedKeys: selfSigned,
	}
}

func genExpect() Expect {
	return Expect{Challenge: genChallenge, Package: genPackage, SigningDigests: map[string]bool{string(genDigest): true}}
}

func TestVerify_generatedGenuineAndGrapheneOS(t *testing.T) {
	p := newPKI(t)
	got, err := p.verifier(keySet{}).Verify(p.chain(desc{}), genExpect())
	if err != nil {
		t.Fatal(err)
	}
	if got.VersionCode != 42 || got.SelfSignedKeyListed || got.OSPatchLevel != 202609 {
		t.Fatalf("%+v", got)
	}

	listed := keySet{string(genBootKey): true}
	got, err = p.verifier(listed).Verify(p.chain(desc{bootState: asn1.Enumerated(BootSelfSigned)}), genExpect())
	if err != nil {
		t.Fatal(err)
	}
	if got.BootState != BootSelfSigned || !got.SelfSignedKeyListed {
		t.Fatalf("GrapheneOS verdict %+v", got)
	}
}

func TestVerify_generatedRefusals(t *testing.T) {
	p := newPKI(t)
	unpinned := genExpect()
	unpinned.SigningDigests = map[string]bool{string(genDigest): true}
	cases := []struct {
		name   string
		d      desc
		reason refusal.Reason
	}{
		{"SelfSigned with a key not on the list", desc{bootState: asn1.Enumerated(BootSelfSigned)}, refusal.ReasonBootKey},
		{"Unverified boot", desc{bootState: asn1.Enumerated(BootUnverified)}, refusal.ReasonBootState},
		{"Failed boot", desc{bootState: asn1.Enumerated(BootFailed)}, refusal.ReasonBootState},
		{"unlocked", desc{unlocked: true}, refusal.ReasonDeviceUnlocked},
		{"root of trust only software-enforced", desc{rotInSoftware: true}, refusal.ReasonBootState},
		{"software key", desc{levelsSet: true, attLevel: 0, kmLevel: 0}, refusal.ReasonSecurityLevel},
		{"StrongBox attestation of a TEE key", desc{levelsSet: true, attLevel: 2, kmLevel: 1}, refusal.ReasonSecurityLevel},
		{"unknown security level", desc{levelsSet: true, attLevel: 3, kmLevel: 3}, refusal.ReasonMalformed},
		{"another challenge", desc{challenge: []byte("replayed")}, refusal.ReasonChallenge},
		{"a shared UID with two packages", desc{packages: []string{genPackage, "io.evil.app"}}, refusal.ReasonPackage},
		{"no signing digest", desc{digests: [][]byte{}}, refusal.ReasonSigningCert},
		{"a second, unpinned signer", desc{digests: [][]byte{genDigest, []byte("another-signer-digest-32-bytes!!")}}, refusal.ReasonSigningCert},
		{"bytes after the key description", desc{trailing: []byte{0x05, 0x00}}, refusal.ReasonMalformed},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := p.verifier(keySet{}).Verify(p.chain(c.d), genExpect())
			wantReason(t, err, c.reason)
		})
	}

	t.Run("patch level not YYYYMM under a window", func(t *testing.T) {
		want := genExpect()
		want.MaxPatchAgeMonths = 12
		_, err := p.verifier(keySet{}).Verify(p.chain(desc{patch: 2026}), want)
		wantReason(t, err, refusal.ReasonPatchLevel)
	})
	t.Run("patch level inside the window", func(t *testing.T) {
		want := genExpect()
		want.MaxPatchAgeMonths = 1
		if _, err := p.verifier(keySet{}).Verify(p.chain(desc{patch: 202609}), want); err != nil {
			t.Fatal(err)
		}
	})
}

func TestVerify_generatedChainAttacks(t *testing.T) {
	p := newPKI(t)
	good := desc{}.der(t)
	v := p.verifier(keySet{})

	t.Run("a chain extended below a genuine attestation key", func(t *testing.T) {
		// The attacker's "leaf" is signed by a key they control, which
		// itself carries the genuine key description.
		attackerKey := genKey(t)
		fakeAtt := sign(t, &x509.Certificate{
			SerialNumber: big.NewInt(9), Subject: pkix.Name{CommonName: "Android Keystore Key"},
			NotBefore: genNow.Add(-time.Hour), NotAfter: genNow.Add(time.Hour),
			IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
			ExtraExtensions: []pkix.Extension{{Id: oidKeyDescription, Value: good}},
		}, p.attCert, &attackerKey.PublicKey, p.attKey)
		leaf := sign(t, &x509.Certificate{
			SerialNumber: big.NewInt(10), Subject: pkix.Name{CommonName: "forged"},
			NotBefore: genNow.Add(-time.Hour), NotAfter: genNow.Add(time.Hour),
			ExtraExtensions: []pkix.Extension{{Id: oidKeyDescription, Value: good}},
		}, fakeAtt, &genKey(t).PublicKey, attackerKey)
		_, err := v.Verify([][]byte{leaf.Raw, fakeAtt.Raw, p.attCert.Raw, p.inter.Raw, p.root.Raw}, genExpect())
		wantReason(t, err, refusal.ReasonChain)
	})
	t.Run("a leaf signed by another key under the same issuer name", func(t *testing.T) {
		_, err := v.Verify([][]byte{p.leaf(good, genKey(t)), p.attCert.Raw, p.inter.Raw, p.root.Raw}, genExpect())
		wantReason(t, err, refusal.ReasonChain)
	})
	t.Run("a root that is not pinned", func(t *testing.T) {
		other := newPKI(t)
		_, err := v.Verify(other.chain(desc{}), genExpect())
		wantReason(t, err, refusal.ReasonChain)
	})
	t.Run("an intermediate not yet valid", func(t *testing.T) {
		q := newPKI(t)
		q.inter = sign(t, ca(2, "Droid CA2", genNow.Add(time.Hour)), q.root, &q.interKey.PublicKey, q.rootKey)
		q.attCert = sign(t, ca(3, "TEE attestation key", genNow.Add(-time.Hour)), q.inter, &q.attKey.PublicKey, q.interKey)
		_, err := q.verifier(keySet{}).Verify(q.chain(desc{}), genExpect())
		wantReason(t, err, refusal.ReasonChain)
	})
	t.Run("a leaf without a key description", func(t *testing.T) {
		_, err := v.Verify([][]byte{p.leaf(nil, p.attKey), p.attCert.Raw, p.inter.Raw, p.root.Raw}, genExpect())
		wantReason(t, err, refusal.ReasonMalformed)
	})
	t.Run("garbage instead of a certificate", func(t *testing.T) {
		_, err := v.Verify([][]byte{{0x30, 0x00}, p.attCert.Raw, p.root.Raw}, genExpect())
		wantReason(t, err, refusal.ReasonMalformed)
	})
	t.Run("a revoked attestation certificate", func(t *testing.T) {
		rv := p.verifier(keySet{})
		rv.Revocation = fakeRevocations{"3": true}
		_, err := rv.Verify(p.chain(desc{}), genExpect())
		wantReason(t, err, refusal.ReasonRevoked)
	})
}

func TestReadExplicit_refusesNonMinimalAndForeignIdentifiers(t *testing.T) {
	for name, in := range map[string][]byte{
		"universal identifier":          {0x30, 0x00},
		"primitive context identifier":  {0x81, 0x00},
		"high tag with a leading 0x80":  {0xbf, 0x80, 0x01, 0x00},
		"high-tag form for a low tag":   {0xbf, 0x05, 0x00},
		"long-form length below 128":    {0xa1, 0x81, 0x05, 0, 0, 0, 0, 0},
		"length with a leading zero":    {0xa1, 0x82, 0x00, 0x80},
		"element longer than its list":  {0xa1, 0x05, 0x00},
		"high tag with too many digits": {0xbf, 0x81, 0x81, 0x81, 0x81, 0x81, 0x01, 0x00},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseAuthorizationList(in); err == nil {
				t.Fatal("accepted")
			}
		})
	}
}
