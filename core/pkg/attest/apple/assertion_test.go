package apple

import (
	"crypto/sha256"
	"testing"

	"github.com/Orama-Network/orama/core/pkg/attest/refusal"
)

func TestVerifyAssertion_generated(t *testing.T) {
	f := newFixture(t)
	v := f.verifier()
	want := Expect{AppID: fixtureAppID}
	cdh := sha256.Sum256([]byte("request"))
	ext := mustCBOR(t, map[string]any{extBundleVersion: "42", extValidationCategory: []byte{4, 0, 0, 0}})

	t.Run("iOS 27 extensions after the header, AT set as App Attest does", func(t *testing.T) {
		got, err := v.VerifyAssertion(want, f.point, 6, f.assertion(assertionAuthData(fixtureAppID, flagAttested, 7, ext), cdh[:]), cdh[:])
		if err != nil {
			t.Fatal(err)
		}
		if got.Counter != 7 || got.BundleVersion != "42" || got.ValidationCategory != 4 {
			t.Fatalf("%+v", got)
		}
	})
	t.Run("pre-iOS 27: the 37-byte header alone", func(t *testing.T) {
		got, err := v.VerifyAssertion(want, f.point, 0, f.assertion(assertionAuthData(fixtureAppID, flagAttested, 1, nil), cdh[:]), cdh[:])
		if err != nil || got.BundleVersion != "" {
			t.Fatalf("%+v, %v", got, err)
		}
	})

	other := cdh
	other[0] ^= 1
	cases := []struct {
		name   string
		obj    []byte
		last   uint32
		cdh    []byte
		reason refusal.Reason
	}{
		{"counter equal to the last", f.assertion(assertionAuthData(fixtureAppID, flagAttested, 7, nil), cdh[:]), 7, cdh[:], refusal.ReasonCounter},
		{"counter below the last", f.assertion(assertionAuthData(fixtureAppID, flagAttested, 3, nil), cdh[:]), 7, cdh[:], refusal.ReasonCounter},
		{"another app", f.assertion(assertionAuthData("ABCDE12345.io.orama.other", flagAttested, 8, nil), cdh[:]), 7, cdh[:], refusal.ReasonAppID},
		{"signed over other client data", f.assertion(assertionAuthData(fixtureAppID, flagAttested, 8, nil), other[:]), 7, cdh[:], refusal.ReasonSignature},
		{"short authenticator data", f.assertion(make([]byte, headerLen-1), cdh[:]), 7, cdh[:], refusal.ReasonMalformed},
		{"garbage after the header", f.assertion(assertionAuthData(fixtureAppID, flagAttested, 8, []byte{1, 2, 3}), cdh[:]), 7, cdh[:], refusal.ReasonMalformed},
		{"not CBOR", []byte{0xff}, 7, cdh[:], refusal.ReasonMalformed},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := v.VerifyAssertion(want, f.point, c.last, c.obj, c.cdh)
			wantReason(t, err, c.reason)
		})
	}

	t.Run("a stored key that is not a P-256 point", func(t *testing.T) {
		_, err := v.VerifyAssertion(want, make([]byte, p256PointLen), 0, f.assertion(assertionAuthData(fixtureAppID, flagAttested, 1, nil), cdh[:]), cdh[:])
		wantReason(t, err, refusal.ReasonMalformed)
	})
}
