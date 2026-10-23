// Package fleetattest is the runner's signed statement about one fleet e2e
// run: which commit it tested, its verdict, and the SHA-256 of the report.json
// it wrote. The e2e-fleet runner signs it with the runner key; the release
// tooling (pkg/releasepub) verifies it before a commit is released.
//
// The statement travels in a DSSE envelope (in-toto's Dead Simple Signing
// Envelope) signed with Ed25519, written beside report.json as
// report.attestation.json. The signature covers the statement, and the
// statement names the report by digest, so a report edited after the run no
// longer matches its attestation.
package fleetattest

import (
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/secure-systems-lab/go-securesystemslib/dsse"
)

// Files in a run's artifact directory.
const (
	// ReportFile is the report the statement names by digest.
	ReportFile = "report.json"
	// FileName is the signed envelope, beside ReportFile.
	FileName = "report.attestation.json"
)

// PayloadType is the DSSE payload type of a statement, and Schema its
// version.
const (
	PayloadType = "application/vnd.orama.fleet-e2e-report+json"
	Schema      = "orama.fleet-e2e-report/v1"
)

// VerdictPass is the verdict of a run in which every feature passed and the
// coverage gate held (every uncovered item waived).
const VerdictPass = "PASS"

// Statement is what the runner signs.
type Statement struct {
	Schema string `json:"schema"`
	RunID  string `json:"run_id"`
	// Commit is the full SHA the run tested, with a "-dirty" suffix when the
	// working tree had uncommitted changes.
	Commit       string    `json:"commit"`
	Verdict      string    `json:"verdict"`
	ReportSHA256 string    `json:"report_sha256"`
	SignedAt     time.Time `json:"signed_at"`
}

// Sign writes the attestation of the report.json in dir: the statement with
// the report's digest, signed with key.
func Sign(ctx context.Context, dir string, key ed25519.PrivateKey, s Statement) error {
	digest, err := reportDigest(dir)
	if err != nil {
		return err
	}
	s.Schema, s.ReportSHA256 = Schema, digest
	body, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("encode the attestation statement: %w", err)
	}
	signer, err := dsse.NewEnvelopeSigner(edSigner{key: key})
	if err != nil {
		return err
	}
	env, err := signer.SignPayload(ctx, PayloadType, body)
	if err != nil {
		return fmt.Errorf("sign the attestation: %w", err)
	}
	data, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		return fmt.Errorf("encode the attestation: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, FileName), append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", FileName, err)
	}
	return nil
}

// Verify reads the attestation in dir, checks that one of allowed signed it
// and that it names the report.json beside it, and returns the statement.
// It does not judge the statement: which commit and verdict are acceptable is
// the caller's policy.
func Verify(ctx context.Context, dir string, allowed []ed25519.PublicKey) (Statement, error) {
	if len(allowed) == 0 {
		return Statement{}, errors.New("no runner key is allowed to sign a fleet report")
	}
	data, err := os.ReadFile(filepath.Join(dir, FileName))
	if err != nil {
		return Statement{}, fmt.Errorf("read the fleet report's attestation: %w", err)
	}
	var env dsse.Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return Statement{}, fmt.Errorf("%s is not a signed envelope: %w", FileName, err)
	}
	if env.PayloadType != PayloadType {
		return Statement{}, fmt.Errorf("%s carries a %q, not a fleet report statement", FileName, env.PayloadType)
	}
	verifiers := make([]dsse.Verifier, 0, len(allowed))
	for _, pub := range allowed {
		verifiers = append(verifiers, edVerifier{pub: pub})
	}
	ev, err := dsse.NewEnvelopeVerifier(verifiers...)
	if err != nil {
		return Statement{}, err
	}
	_, body, err := ev.VerifyAndDecode(ctx, &env)
	if err != nil {
		return Statement{}, fmt.Errorf("%s is not signed by an allowed runner key: %w", FileName, err)
	}
	var s Statement
	if err := json.Unmarshal(body, &s); err != nil {
		return Statement{}, fmt.Errorf("decode the attestation statement: %w", err)
	}
	if s.Schema != Schema {
		return Statement{}, fmt.Errorf("the attestation statement is %q, not %s", s.Schema, Schema)
	}
	digest, err := reportDigest(dir)
	if err != nil {
		return Statement{}, err
	}
	if digest != s.ReportSHA256 {
		return Statement{}, fmt.Errorf("%s changed after the runner signed it (sha256 %s, signed %s)", ReportFile, digest, s.ReportSHA256)
	}
	return s, nil
}

// Remove deletes the attestation in dir, if any: a report.json rewritten
// outside a run is no longer the one the runner signed.
func Remove(dir string) error {
	if err := os.Remove(filepath.Join(dir, FileName)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove the stale %s: %w", FileName, err)
	}
	return nil
}

func reportDigest(dir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(dir, ReportFile))
	if err != nil {
		return "", fmt.Errorf("read the fleet report: %w", err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// ParsePrivateKey reads an Ed25519 private key in PKCS #8 PEM, the form
// `openssl genpkey -algorithm ed25519` writes.
func ParsePrivateKey(data []byte) (ed25519.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil || block.Type != "PRIVATE KEY" {
		return nil, errors.New("the runner signing key is not a PEM \"PRIVATE KEY\" block")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse the runner signing key: %w", err)
	}
	ed, ok := key.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("the runner signing key is a %T, not Ed25519", key)
	}
	return ed, nil
}

// ParsePublicKey reads a runner public key written as the standard base64 of
// its 32 raw bytes (FormatPublicKey).
func ParsePublicKey(s string) (ed25519.PublicKey, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(s))
	if err != nil {
		return nil, fmt.Errorf("runner key %q is not base64: %w", s, err)
	}
	if len(raw) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("runner key %q is %d bytes, not an Ed25519 public key (%d)", s, len(raw), ed25519.PublicKeySize)
	}
	return ed25519.PublicKey(raw), nil
}

// FormatPublicKey is the form ParsePublicKey reads.
func FormatPublicKey(pub ed25519.PublicKey) string {
	return base64.StdEncoding.EncodeToString(pub)
}

// keyID names a key in the envelope: the hex SHA-256 of its raw bytes.
func keyID(pub ed25519.PublicKey) string {
	sum := sha256.Sum256(pub)
	return hex.EncodeToString(sum[:])
}

type edSigner struct{ key ed25519.PrivateKey }

func (s edSigner) Sign(_ context.Context, data []byte) ([]byte, error) {
	return ed25519.Sign(s.key, data), nil
}

func (s edSigner) KeyID() (string, error) {
	return keyID(s.key.Public().(ed25519.PublicKey)), nil
}

type edVerifier struct{ pub ed25519.PublicKey }

func (v edVerifier) Verify(_ context.Context, data, sig []byte) error {
	if !ed25519.Verify(v.pub, data, sig) {
		return errors.New("ed25519 signature does not verify")
	}
	return nil
}

func (v edVerifier) KeyID() (string, error) { return keyID(v.pub), nil }

func (v edVerifier) Public() crypto.PublicKey { return v.pub }
