package apple

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/Orama-Network/orama/core/pkg/attest/refusal"
)

func b64(t *testing.T, s string) []byte {
	t.Helper()
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func at(when time.Time) *Verifier {
	v := New()
	v.Now = func() time.Time { return when }
	return v
}

func wantReason(t *testing.T, err error, reason refusal.Reason) {
	t.Helper()
	r, ok := refusal.As(err)
	if !ok {
		t.Fatalf("want a %s refusal, got %v", reason, err)
	}
	if r.Kind != refusal.KindInvalid || r.Reason != reason {
		t.Fatalf("want an invalid/%s refusal, got %s/%s: %v", reason, r.Kind, r.Reason, err)
	}
}

// Apple's Attestation Object Validation Guide sample: an iOS 27 attestation
// with the signed extensions. Apple's sample passes the raw challenge as
// clientDataHash, and its expected values are the ones asserted here.
var (
	guideAppID     = "1234567890.com.example.myapp"
	guideKeyID     = "zgSY9YSD+7TaDXssY6WlOPVS1K3Lmk+pFhlcSWE+ZV0="
	guideChallenge = []byte("example_server_challenge")
	guideNow       = time.Date(2026, 4, 21, 18, 13, 12, 0, time.UTC)
)

func guideObject(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/apple_guide_attestation.cbor")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestVerifyAttestation_acceptsApplesValidationGuideSample(t *testing.T) {
	got, err := at(guideNow).VerifyAttestation(Expect{AppID: guideAppID}, b64(t, guideKeyID), guideObject(t), guideChallenge)
	if err != nil {
		t.Fatal(err)
	}
	if got.Development {
		t.Fatal("the guide's sample is a production key")
	}
	if h := sha256.Sum256(got.PublicKey); base64.StdEncoding.EncodeToString(h[:]) != guideKeyID {
		t.Fatalf("public key hash %x is not the key id", h)
	}
	// The guide's prose says "1.0"; the sample it ships signs "1".
	if got.BundleVersion != "1" || got.ValidationCategory != 1 {
		t.Fatalf("extensions %+v, want bundle version 1 and category 1", got.Extensions)
	}
	if len(got.Receipt) == 0 {
		t.Fatal("no receipt")
	}
}

func TestVerifyAttestation_refusesApplesSampleForAnythingElse(t *testing.T) {
	keyID := b64(t, guideKeyID)
	cases := []struct {
		name      string
		v         *Verifier
		want      Expect
		keyID     []byte
		challenge []byte
		reason    refusal.Reason
	}{
		{"another challenge", at(guideNow), Expect{AppID: guideAppID}, keyID, []byte("another_challenge"), refusal.ReasonNonce},
		{"another app", at(guideNow), Expect{AppID: "1234567890.com.example.other"}, keyID, guideChallenge, refusal.ReasonAppID},
		{"another key id", at(guideNow), Expect{AppID: guideAppID}, make([]byte, 32), guideChallenge, refusal.ReasonKeyID},
		{"an expired credential certificate", at(guideNow.AddDate(0, 0, 3)), Expect{AppID: guideAppID}, keyID, guideChallenge, refusal.ReasonChain},
		{"another root", newFixture(t).verifier(), Expect{AppID: guideAppID}, keyID, guideChallenge, refusal.ReasonChain},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.name == "another root" {
				c.v.Now = func() time.Time { return guideNow }
			}
			_, err := c.v.VerifyAttestation(c.want, c.keyID, guideObject(t), c.challenge)
			wantReason(t, err, c.reason)
		})
	}
}

// A real iOS 14.4 device in the development environment, with the
// assertion it made next (see testdata/ios14_4.json for the source).
type ios14Vectors struct {
	Attestation struct {
		AttestationBase64 string
		KeyIDBase64       string `json:"keyIdBase64"`
		ClientDataHash    string `json:"clientDataHashSha256Base64"`
		TeamIdentifier    string
		BundleIdentifier  string
		Timestamp         string
	}
	Assertion struct {
		AssertionBase64 string
		ClientDataHash  string `json:"clientDataHashSha256Base64"`
	}
}

func ios14(t *testing.T) (ios14Vectors, *Verifier, Expect) {
	t.Helper()
	raw, err := os.ReadFile("testdata/ios14_4.json")
	if err != nil {
		t.Fatal(err)
	}
	var v ios14Vectors
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	when, err := time.Parse(time.RFC3339, v.Attestation.Timestamp)
	if err != nil {
		t.Fatal(err)
	}
	return v, at(when), Expect{AppID: v.Attestation.TeamIdentifier + "." + v.Attestation.BundleIdentifier}
}

func TestVerifyAttestation_realIOS14DevelopmentKeyNeedsDevelopmentAllowed(t *testing.T) {
	vec, v, want := ios14(t)
	obj := b64(t, vec.Attestation.AttestationBase64)
	keyID := b64(t, vec.Attestation.KeyIDBase64)
	cdh := b64(t, vec.Attestation.ClientDataHash)

	_, err := v.VerifyAttestation(want, keyID, obj, cdh)
	wantReason(t, err, refusal.ReasonEnvironment)

	want.AllowDevelopment = true
	got, err := v.VerifyAttestation(want, keyID, obj, cdh)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Development || got.BundleVersion != "" || got.ValidationCategory != 0 {
		t.Fatalf("iOS 14 signs no extensions and this is a development key: %+v", got)
	}
}

func TestVerifyAssertion_realIOS14AssertionAndItsReplay(t *testing.T) {
	vec, v, want := ios14(t)
	want.AllowDevelopment = true
	att, err := v.VerifyAttestation(want, b64(t, vec.Attestation.KeyIDBase64), b64(t, vec.Attestation.AttestationBase64), b64(t, vec.Attestation.ClientDataHash))
	if err != nil {
		t.Fatal(err)
	}
	obj := b64(t, vec.Assertion.AssertionBase64)
	cdh := b64(t, vec.Assertion.ClientDataHash)

	got, err := v.VerifyAssertion(want, att.PublicKey, 0, obj, cdh)
	if err != nil {
		t.Fatal(err)
	}
	if got.Counter != 1 {
		t.Fatalf("counter %d, want 1", got.Counter)
	}
	_, err = v.VerifyAssertion(want, att.PublicKey, got.Counter, obj, cdh)
	wantReason(t, err, refusal.ReasonCounter)

	_, err = v.VerifyAssertion(want, att.PublicKey, 0, obj, make([]byte, 32))
	wantReason(t, err, refusal.ReasonSignature)
}

func TestVerifyAttestation_generatedProofsRefusedByEachCheck(t *testing.T) {
	f := newFixture(t)
	cdh := sha256.Sum256([]byte("challenge"))
	dev := aaguidDevelopment
	unknown := [aaguidLen]byte{'x'}
	other := genKey(t, p256Curve)
	otherPoint, _ := other.PublicKey.Bytes()
	otherCOSE := mustCBOR(t, coseEC2Key{Kty: coseKtyEC2, Alg: coseAlgES256, Curve: coseCrvP256, X: otherPoint[1:33], Y: otherPoint[33:]})

	if _, err := f.verifier().VerifyAttestation(Expect{AppID: fixtureAppID}, f.keyID, f.object(attestation{}, cdh[:]), cdh[:]); err != nil {
		t.Fatalf("the genuine generated attestation: %v", err)
	}
	cases := []struct {
		name   string
		a      attestation
		reason refusal.Reason
	}{
		{"counter above zero", attestation{counter: 1}, refusal.ReasonCounter},
		{"development key", attestation{aaguid: &dev}, refusal.ReasonEnvironment},
		{"unknown aaguid", attestation{aaguid: &unknown}, refusal.ReasonEnvironment},
		{"credential id not the key id", attestation{credentialID: make([]byte, 32)}, refusal.ReasonKeyID},
		{"COSE key not the certificate's", attestation{coseKey: otherCOSE}, refusal.ReasonKeyID},
		{"nonce over other data", attestation{nonce: make([]byte, 32)}, refusal.ReasonNonce},
		{"another app", attestation{appID: "ABCDE12345.io.orama.other"}, refusal.ReasonAppID},
		{"no attested credential data flag", attestation{flags: flagExtensions}, refusal.ReasonMalformed},
		{"extensions flagged but absent", attestation{flags: flagAttested | flagExtensions}, refusal.ReasonMalformed},
		{"bytes after the extensions", attestation{extensions: append(mustCBOR(t, map[string]any{extBundleVersion: "2"}), 0)}, refusal.ReasonMalformed},
		{"bundle version not a string", attestation{extensions: mustCBOR(t, map[string]any{extBundleVersion: 2})}, refusal.ReasonMalformed},
		{"category a 3-byte string", attestation{extensions: mustCBOR(t, map[string]any{extValidationCategory: []byte{1, 0, 0}})}, refusal.ReasonMalformed},
		{"category above UInt32", attestation{extensions: mustCBOR(t, map[string]any{extValidationCategory: uint64(1) << 32})}, refusal.ReasonMalformed},
		{"indefinite-length extensions", attestation{extensions: []byte{0xbf, 0xff}}, refusal.ReasonMalformed},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := f.verifier().VerifyAttestation(Expect{AppID: fixtureAppID}, f.keyID, f.object(c.a, cdh[:]), cdh[:])
			wantReason(t, err, c.reason)
		})
	}
}

func TestVerifyAttestation_readsBothExtensionEncodingsAndIgnoresUnknownKeys(t *testing.T) {
	f := newFixture(t)
	cdh := sha256.Sum256([]byte("challenge"))
	for name, ext := range map[string][]byte{
		"UInt32 as an integer":      mustCBOR(t, map[string]any{extBundleVersion: "1.2.3", extValidationCategory: 4, "apple_future_02": true}),
		"UInt32 as 4 LE bytes":      mustCBOR(t, map[string]any{extBundleVersion: "1.2.3", extValidationCategory: []byte{4, 0, 0, 0}}),
		"with the ED flag set, too": mustCBOR(t, map[string]any{extBundleVersion: "1.2.3", extValidationCategory: 4}),
	} {
		t.Run(name, func(t *testing.T) {
			a := attestation{extensions: ext}
			if name == "with the ED flag set, too" {
				a.flags = flagAttested | flagExtensions
			}
			got, err := f.verifier().VerifyAttestation(Expect{AppID: fixtureAppID}, f.keyID, f.object(a, cdh[:]), cdh[:])
			if err != nil {
				t.Fatal(err)
			}
			if got.BundleVersion != "1.2.3" || got.ValidationCategory != 4 {
				t.Fatalf("extensions %+v", got.Extensions)
			}
		})
	}
}

func TestVerifyAttestation_developmentKeyAcceptedWhenAllowed(t *testing.T) {
	f := newFixture(t)
	cdh := sha256.Sum256([]byte("challenge"))
	dev := aaguidDevelopment
	got, err := f.verifier().VerifyAttestation(Expect{AppID: fixtureAppID, AllowDevelopment: true}, f.keyID, f.object(attestation{aaguid: &dev}, cdh[:]), cdh[:])
	if err != nil || !got.Development {
		t.Fatalf("development key with development allowed: %+v, %v", got, err)
	}
}

func TestVerifyAttestation_refusesMalformedObjects(t *testing.T) {
	f := newFixture(t)
	cdh := sha256.Sum256([]byte("challenge"))
	leafOnly := mustCBOR(t, attestationObject{Fmt: formatAppAttest, AttStmt: attStmt{X5C: [][]byte{f.leaf(make([]byte, 32)).Raw}}, AuthData: f.authData(attestation{})})
	cases := map[string]struct {
		obj    []byte
		reason refusal.Reason
	}{
		"not CBOR":         {[]byte{0xff}, refusal.ReasonMalformed},
		"another format":   {mustCBOR(t, attestationObject{Fmt: "packed"}), refusal.ReasonMalformed},
		"no intermediate":  {leafOnly, refusal.ReasonChain},
		"tagged CBOR":      {append([]byte{0xd8, 0x18}, f.object(attestation{}, cdh[:])...), refusal.ReasonMalformed},
		"trailing garbage": {append(f.object(attestation{}, cdh[:]), 0), refusal.ReasonMalformed},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := f.verifier().VerifyAttestation(Expect{AppID: fixtureAppID}, f.keyID, c.obj, cdh[:])
			wantReason(t, err, c.reason)
		})
	}
}
