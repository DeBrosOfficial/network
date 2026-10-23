package fleetattest

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newKey(t *testing.T) ed25519.PrivateKey {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return priv
}

func pub(k ed25519.PrivateKey) ed25519.PublicKey { return k.Public().(ed25519.PublicKey) }

// signedRun is an artifact dir with a report.json and its attestation.
func signedRun(t *testing.T, key ed25519.PrivateKey) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ReportFile), []byte(`{"run_id":"abcd1234"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := Statement{RunID: "abcd1234", Commit: strings.Repeat("a", 40), Verdict: VerdictPass, SignedAt: time.Date(2026, 10, 23, 0, 0, 0, 0, time.UTC)}
	if err := Sign(t.Context(), dir, key, s); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestVerify_acceptsTheStatementAnAllowedKeySigned(t *testing.T) {
	key := newKey(t)
	dir := signedRun(t, key)
	s, err := Verify(t.Context(), dir, []ed25519.PublicKey{pub(newKey(t)), pub(key)})
	if err != nil {
		t.Fatal(err)
	}
	if s.Schema != Schema || s.RunID != "abcd1234" || s.Verdict != VerdictPass || len(s.ReportSHA256) != 64 {
		t.Fatalf("statement %+v", s)
	}
}

func TestVerify_refusesAKeyNotOnTheList(t *testing.T) {
	dir := signedRun(t, newKey(t))
	if _, err := Verify(t.Context(), dir, []ed25519.PublicKey{pub(newKey(t))}); err == nil || !strings.Contains(err.Error(), "not signed by an allowed runner key") {
		t.Fatalf("err = %v", err)
	}
}

func TestVerify_refusesAnEmptyAllowList(t *testing.T) {
	dir := signedRun(t, newKey(t))
	if _, err := Verify(t.Context(), dir, nil); err == nil || !strings.Contains(err.Error(), "no runner key") {
		t.Fatalf("err = %v", err)
	}
}

func TestVerify_refusesAReportEditedAfterSigning(t *testing.T) {
	key := newKey(t)
	dir := signedRun(t, key)
	if err := os.WriteFile(filepath.Join(dir, ReportFile), []byte(`{"run_id":"abcd1234","edited":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(t.Context(), dir, []ed25519.PublicKey{pub(key)}); err == nil || !strings.Contains(err.Error(), "changed after the runner signed it") {
		t.Fatalf("err = %v", err)
	}
}

func TestVerify_refusesATamperedStatement(t *testing.T) {
	key := newKey(t)
	dir := signedRun(t, key)
	path := filepath.Join(dir, FileName)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var env map[string]any
	if err := json.Unmarshal(data, &env); err != nil {
		t.Fatal(err)
	}
	body, err := base64.StdEncoding.DecodeString(env["payload"].(string))
	if err != nil {
		t.Fatal(err)
	}
	env["payload"] = base64.StdEncoding.EncodeToString([]byte(strings.Replace(string(body), `"verdict":"PASS"`, `"verdict":"FAIL"`, 1)))
	forged, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, forged, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(t.Context(), dir, []ed25519.PublicKey{pub(key)}); err == nil {
		t.Fatal("a payload swapped under the signature verified")
	}
}

func TestVerify_refusesAMissingAttestation(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ReportFile), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(t.Context(), dir, []ed25519.PublicKey{pub(newKey(t))}); err == nil {
		t.Fatal("a report without an attestation verified")
	}
}

func TestRemove_dropsTheAttestationAndToleratesNone(t *testing.T) {
	dir := signedRun(t, newKey(t))
	if err := Remove(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, FileName)); !os.IsNotExist(err) {
		t.Fatalf("stat after Remove: %v", err)
	}
	if err := Remove(dir); err != nil {
		t.Fatalf("a second Remove: %v", err)
	}
}

func TestParsePrivateKey_readsPKCS8PEM(t *testing.T) {
	key := newKey(t)
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ParsePrivateKey(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
	if err != nil || !got.Equal(key) {
		t.Fatalf("ParsePrivateKey = %v, %v", got, err)
	}
	if _, err := ParsePrivateKey([]byte("not a key")); err == nil {
		t.Fatal("garbage parsed as a key")
	}
}

func TestPublicKey_roundTripsAndRefusesTheWrongLength(t *testing.T) {
	p := pub(newKey(t))
	got, err := ParsePublicKey(FormatPublicKey(p))
	if err != nil || !got.Equal(p) {
		t.Fatalf("round trip = %v, %v", got, err)
	}
	if _, err := ParsePublicKey("AAAA"); err == nil {
		t.Fatal("a 3-byte key parsed")
	}
}
