package android

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Orama-Network/orama/core/pkg/attest/refusal"
)

// The testdata chains are real devices' attestations from Google's
// reference verifier, github.com/android/keyattestation (testdata/,
// Apache-2.0), each with the JSON of its decoded key description.

type fakeRevocations map[string]bool

func (f fakeRevocations) Revoked(serial string) (bool, error) { return f[serial], nil }

type unloadedRevocations struct{}

func (unloadedRevocations) Revoked(string) (bool, error) {
	return false, errors.New("no revocation list loaded yet")
}

type keySet map[string]bool

func (k keySet) Contains(key []byte) bool { return k[string(key)] }

// vector is a real chain and what its JSON says about it.
type vector struct {
	chain     [][]byte
	challenge []byte
	pkg       string
	version   int64
	digests   [][]byte
	created   time.Time
	bootState string
	bootKey   []byte
	locked    bool
	osPatch   int64
}

func loadVector(t *testing.T, name string) vector {
	t.Helper()
	raw, err := os.ReadFile("testdata/" + name + ".pem")
	if err != nil {
		t.Fatal(err)
	}
	var v vector
	for {
		var block *pem.Block
		block, raw = pem.Decode(raw)
		if block == nil {
			break
		}
		v.chain = append(v.chain, block.Bytes)
	}
	js, err := os.ReadFile("testdata/" + name + ".json")
	if err != nil {
		return v
	}
	var lines []string
	for _, l := range strings.Split(string(js), "\n") {
		if !strings.HasPrefix(strings.TrimSpace(l), "//") {
			lines = append(lines, l)
		}
	}
	var d struct {
		AttestationChallenge string
		SoftwareEnforced     struct {
			CreationDateTime         string
			AttestationApplicationID struct {
				Packages   []struct{ Name, Version string }
				Signatures []string
			} `json:"attestationApplicationId"`
		}
		HardwareEnforced struct {
			RootOfTrust struct {
				VerifiedBootKey   string
				DeviceLocked      bool
				VerifiedBootState string
			}
			OSPatchLevel string `json:"osPatchLevel"`
		}
	}
	if err := json.Unmarshal([]byte(strings.Join(lines, "\n")), &d); err != nil {
		t.Fatal(err)
	}
	dec := func(s string) []byte {
		b, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	v.challenge = dec(d.AttestationChallenge)
	app := d.SoftwareEnforced.AttestationApplicationID
	v.pkg = app.Packages[0].Name
	v.version, _ = strconv.ParseInt(app.Packages[0].Version, 10, 64)
	for _, s := range app.Signatures {
		v.digests = append(v.digests, dec(s))
	}
	ms, err := strconv.ParseInt(d.SoftwareEnforced.CreationDateTime, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	v.created = time.UnixMilli(ms)
	rot := d.HardwareEnforced.RootOfTrust
	v.bootState, v.bootKey, v.locked = rot.VerifiedBootState, dec(rot.VerifiedBootKey), rot.DeviceLocked
	v.osPatch, _ = strconv.ParseInt(d.HardwareEnforced.OSPatchLevel, 10, 64)
	return v
}

func (v vector) expect() Expect {
	digests := map[string]bool{}
	for _, d := range v.digests {
		digests[string(d)] = true
	}
	return Expect{Challenge: v.challenge, Package: v.pkg, SigningDigests: digests}
}

func realVerifier(at time.Time, revoked fakeRevocations, selfSigned keySet) *Verifier {
	v := New(revoked, selfSigned)
	v.Now = func() time.Time { return at }
	return v
}

func wantReason(t *testing.T, err error, reason refusal.Reason) {
	t.Helper()
	r, ok := refusal.As(err)
	if !ok {
		t.Fatalf("want a %s refusal, got %v", reason, err)
	}
	if r.Kind != refusal.KindInvalid || r.Reason != reason {
		t.Fatalf("want invalid/%s, got %s/%s: %v", reason, r.Kind, r.Reason, err)
	}
}

func TestVerify_acceptsRealLockedVerifiedDevices(t *testing.T) {
	for _, name := range []string{
		"caiman_sdk36_TEE_EC_RKP",        // Pixel 9 Pro, RKP chain to the RSA root
		"tegu_sdk36_SB_EC_2026_ROOT",     // StrongBox, RKP chain to Key Attestation CA1
		"frankel_sdk37_TEE_EC_2026",      // TEE, chain to Key Attestation CA1
		"sony-xperia10-iii_sdk33_TEE_EC", // a non-Google OEM, factory chain
	} {
		t.Run(name, func(t *testing.T) {
			vec := loadVector(t, name)
			got, err := realVerifier(vec.created, fakeRevocations{}, keySet{}).Verify(vec.chain, vec.expect())
			if err != nil {
				t.Fatal(err)
			}
			if got.BootState != BootVerified || !got.DeviceLocked || string(got.BootKey) != string(vec.bootKey) {
				t.Fatalf("root of trust %v %v %x, want Verified, locked, %x", got.BootState, got.DeviceLocked, got.BootKey, vec.bootKey)
			}
			if got.Package != vec.pkg || got.VersionCode != vec.version || got.OSPatchLevel != vec.osPatch {
				t.Fatalf("verdict %+v disagrees with the vector's JSON", got)
			}
			if got.SecurityLevel < SecurityTEE {
				t.Fatalf("security level %d", got.SecurityLevel)
			}
		})
	}
}

func TestVerify_factoryChainStaysValidAfterItsIntermediatesExpire(t *testing.T) {
	vec := loadVector(t, "sony-xperia10-iii_sdk33_TEE_EC")
	// Its intermediates expired on 2026-05-24.
	after := time.Date(2026, 10, 23, 0, 0, 0, 0, time.UTC)
	if _, err := realVerifier(after, fakeRevocations{}, keySet{}).Verify(vec.chain, vec.expect()); err != nil {
		t.Fatal(err)
	}
}

func TestVerify_refusesRealChainsForWhatTheyShow(t *testing.T) {
	caiman := loadVector(t, "caiman_sdk36_TEE_EC_RKP")
	akita := loadVector(t, "akita_sdk34_TEE_EC_NONE")
	marlin := loadVector(t, "marlin_sdk29_TEE_EC_NONE")

	t.Run("an unlocked bootloader", func(t *testing.T) {
		_, err := realVerifier(akita.created, fakeRevocations{}, keySet{}).Verify(akita.chain, akita.expect())
		wantReason(t, err, refusal.ReasonDeviceUnlocked)
	})
	t.Run("a software attestation root", func(t *testing.T) {
		_, err := realVerifier(marlin.created, fakeRevocations{}, keySet{}).Verify(marlin.chain, marlin.expect())
		wantReason(t, err, refusal.ReasonChain)
	})
	t.Run("an RKP certificate past its expiry", func(t *testing.T) {
		_, err := realVerifier(caiman.created.AddDate(0, 6, 0), fakeRevocations{}, keySet{}).Verify(caiman.chain, caiman.expect())
		wantReason(t, err, refusal.ReasonChain)
	})
	t.Run("a revoked intermediate", func(t *testing.T) {
		// Droid CA3 of this chain.
		revoked := fakeRevocations{"ed74866372b0791cf1478b39fad0f755593ad3": true}
		_, err := realVerifier(caiman.created, revoked, keySet{}).Verify(caiman.chain, caiman.expect())
		wantReason(t, err, refusal.ReasonRevoked)
	})
	t.Run("another challenge", func(t *testing.T) {
		want := caiman.expect()
		want.Challenge = []byte("another")
		_, err := realVerifier(caiman.created, fakeRevocations{}, keySet{}).Verify(caiman.chain, want)
		wantReason(t, err, refusal.ReasonChallenge)
	})
	t.Run("another package", func(t *testing.T) {
		want := caiman.expect()
		want.Package = "io.orama.anchat"
		_, err := realVerifier(caiman.created, fakeRevocations{}, keySet{}).Verify(caiman.chain, want)
		wantReason(t, err, refusal.ReasonPackage)
	})
	t.Run("re-signed: a signing certificate the policy does not pin", func(t *testing.T) {
		want := caiman.expect()
		want.SigningDigests = map[string]bool{string(make([]byte, 32)): true}
		_, err := realVerifier(caiman.created, fakeRevocations{}, keySet{}).Verify(caiman.chain, want)
		wantReason(t, err, refusal.ReasonSigningCert)
	})
	t.Run("an OS patch level older than the window", func(t *testing.T) {
		// Patch level 2023-07, checked in 2026-10.
		vec := loadVector(t, "sony-xperia10-iii_sdk33_TEE_EC")
		want := vec.expect()
		want.MaxPatchAgeMonths = 3
		_, err := realVerifier(time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), fakeRevocations{}, keySet{}).Verify(vec.chain, want)
		wantReason(t, err, refusal.ReasonPatchLevel)
	})
	t.Run("a chain cut short", func(t *testing.T) {
		_, err := realVerifier(caiman.created, fakeRevocations{}, keySet{}).Verify(caiman.chain[:2], caiman.expect())
		wantReason(t, err, refusal.ReasonChain)
	})
	t.Run("a chain missing an intermediate", func(t *testing.T) {
		chain := append([][]byte{caiman.chain[0], caiman.chain[1]}, caiman.chain[3:]...)
		_, err := realVerifier(caiman.created, fakeRevocations{}, keySet{}).Verify(chain, caiman.expect())
		wantReason(t, err, refusal.ReasonChain)
	})
}

func TestVerify_noRevocationListIsTheServersErrorNotARefusal(t *testing.T) {
	vec := loadVector(t, "caiman_sdk36_TEE_EC_RKP")
	v := New(unloadedRevocations{}, keySet{})
	v.Now = func() time.Time { return vec.created }
	_, err := v.Verify(vec.chain, vec.expect())
	if err == nil {
		t.Fatal("verified without a revocation list")
	}
	if _, ok := refusal.As(err); ok {
		t.Fatalf("a missing revocation list is not the client's fault: %v", err)
	}
}

func TestVerify_refusesGooglesMalformedKeyDescriptions(t *testing.T) {
	for name, why := range map[string]string{
		"invalid_tags_not_in_ascending_order": "ascending",
		"invalid_malformed_rot_device_locked": "RootOfTrust",
	} {
		t.Run(name, func(t *testing.T) {
			vec := loadVector(t, name)
			leaf, err := x509.ParseCertificate(vec.chain[0])
			if err != nil {
				t.Fatal(err)
			}
			ext, ok := keyDescriptionExtension(leaf)
			if !ok {
				t.Fatal("no key description")
			}
			if _, err := ParseKeyDescription(ext); err == nil || !strings.Contains(err.Error(), why) {
				t.Fatalf("want a refusal naming %q, got %v", why, err)
			}
		})
	}
}
