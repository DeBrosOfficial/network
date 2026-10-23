package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Orama-Network/orama/core/pkg/attest"
	"github.com/Orama-Network/orama/core/pkg/client"
)

// Device attestation (feat-941): what a namespace requires of its callers.
//
// A namespace may admit to its functions and data plane only the genuine build
// of its app on a genuine device. A bundled API key cannot be that authority —
// anyone can extract it — so the device proves it: iOS with App Attest,
// Android (GrapheneOS included) with hardware key attestation. Verifying a
// proof is package attest; this file is what a namespace accepts.

// AttestationMode is how a namespace holds its callers to attestation.
type AttestationMode string

const (
	// AttestationOff checks nothing. It is what a namespace with no policy
	// has, and how every namespace worked before attestation existed.
	AttestationOff AttestationMode = "off"
	// AttestationMonitor judges every caller and refuses none: what would
	// have been refused is logged and counted, so an operator can see who
	// enforcing would lock out before it does.
	AttestationMonitor AttestationMode = "monitor"
	// AttestationEnforce refuses every caller the policy does not admit.
	AttestationEnforce AttestationMode = "enforce"
)

const (
	// DefaultReattestInterval is how long an Android attestation lasts before
	// the device must attest again. Android fixes the app version in the key
	// when it is generated, so re-attesting is what makes a raised minimum
	// version reach a device that never updated; a week bounds that.
	DefaultReattestInterval = 7 * 24 * time.Hour
	// minReattestInterval and maxReattestInterval bound what a namespace may
	// set: an hour is the shortest a phone can be expected to re-attest on a
	// schedule, a year the longest a proof says anything about the device.
	minReattestInterval = time.Hour
	maxReattestInterval = 365 * 24 * time.Hour
)

// ErrLobbyHasNoAttestationPolicy is a policy set on the lobby, which is no
// tenant's namespace and has no app to attest.
var ErrLobbyHasNoAttestationPolicy = errors.New("the lobby has no attestation policy; set one on a namespace you own")

// ErrInvalidAttestationPolicy is a policy that cannot be enforced as written.
var ErrInvalidAttestationPolicy = errors.New("invalid attestation policy")

// ParseAttestationMode reads a mode name.
func ParseAttestationMode(raw string) (AttestationMode, error) {
	switch m := AttestationMode(strings.ToLower(strings.TrimSpace(raw))); m {
	case AttestationOff, AttestationMonitor, AttestationEnforce:
		return m, nil
	default:
		return "", fmt.Errorf("attestation mode is one of off, monitor, enforce (got %q)", raw)
	}
}

// AttestationPolicy is what one namespace accepts.
type AttestationPolicy struct {
	Mode AttestationMode `json:"mode"`
	IOS  IOSAttestation  `json:"ios"`
	// Android is accepted from any OEM whose device boots verified, its
	// verified-boot key pinned on first use, and from GrapheneOS through its
	// signed list when AllowGrapheneOS is set.
	Android AndroidAttestation `json:"android"`
	// ReattestInterval is how long an Android attestation lasts.
	ReattestInterval time.Duration `json:"-"`
}

// IOSAttestation is the iOS app a namespace accepts. An empty team id accepts
// no iOS build.
type IOSAttestation struct {
	TeamID   string `json:"team_id"`
	BundleID string `json:"bundle_id"`
	// AllowDevelopment accepts the App Attest development environment and
	// development-signed builds: a devnet namespace.
	AllowDevelopment bool `json:"allow_development"`
	// MinBundleVersion is the oldest CFBundleVersion accepted, "" for none.
	// iOS 26 and older sign no version: such an install is still attested as
	// the genuine app, and the minimum cannot apply to it.
	MinBundleVersion string `json:"min_bundle_version"`
}

// AndroidAttestation is the Android app a namespace accepts. An empty package
// accepts no Android build.
type AndroidAttestation struct {
	Package string `json:"package"`
	// SigningCertDigests are the lowercase-hex SHA-256 digests of the signing
	// certificates the app may be signed with. A re-signed APK has another.
	SigningCertDigests []string `json:"signing_cert_digests"`
	// AllowGrapheneOS accepts a device booting GrapheneOS: verified boot
	// with a key on GrapheneOS's signed list rather than an OEM's.
	AllowGrapheneOS bool `json:"allow_grapheneos"`
	// MinVersionCode is the oldest versionCode accepted, 0 for none.
	MinVersionCode int64 `json:"min_version_code"`
	// MaxPatchAgeMonths refuses an OS whose security patch is older, 0 for
	// no limit.
	MaxPatchAgeMonths int `json:"max_patch_age_months"`
}

// AcceptsIOS and AcceptsAndroid report whether the policy names an app on
// that platform.
func (p AttestationPolicy) AcceptsIOS() bool     { return p.IOS.TeamID != "" || p.IOS.BundleID != "" }
func (p AttestationPolicy) AcceptsAndroid() bool { return p.Android.Package != "" }

// Verifier is the policy as package attest checks a proof against it.
func (p AttestationPolicy) Verifier() attest.Policy {
	return attest.Policy{
		AppleTeamID:               p.IOS.TeamID,
		AppleBundleID:             p.IOS.BundleID,
		AllowDevelopment:          p.IOS.AllowDevelopment,
		MinIOSBundleVersion:       p.IOS.MinBundleVersion,
		AndroidPackage:            p.Android.Package,
		AndroidSigningCertDigests: p.Android.SigningCertDigests,
		MinAndroidVersionCode:     p.Android.MinVersionCode,
		MaxPatchAgeMonths:         p.Android.MaxPatchAgeMonths,
	}
}

// Normalize trims what a person typed and lowercases the digests, so the
// stored policy is the one the verifier compares against.
func (p AttestationPolicy) Normalize() AttestationPolicy {
	p.Mode = AttestationMode(strings.ToLower(strings.TrimSpace(string(p.Mode))))
	p.IOS.TeamID = strings.TrimSpace(p.IOS.TeamID)
	p.IOS.BundleID = strings.TrimSpace(p.IOS.BundleID)
	p.IOS.MinBundleVersion = strings.TrimSpace(p.IOS.MinBundleVersion)
	p.Android.Package = strings.TrimSpace(p.Android.Package)
	digests := make([]string, 0, len(p.Android.SigningCertDigests))
	for _, d := range p.Android.SigningCertDigests {
		if d = strings.ToLower(strings.TrimSpace(d)); d != "" {
			digests = append(digests, d)
		}
	}
	p.Android.SigningCertDigests = digests
	if p.ReattestInterval == 0 {
		p.ReattestInterval = DefaultReattestInterval
	}
	return p
}

// Validate reports whether the policy can be enforced as written. A mode
// other than off must name an app on at least one platform, and every
// platform it names must be complete.
func (p AttestationPolicy) Validate() error {
	if _, err := ParseAttestationMode(string(p.Mode)); err != nil {
		return err
	}
	if p.ReattestInterval < minReattestInterval || p.ReattestInterval > maxReattestInterval {
		return fmt.Errorf("the re-attestation interval is between %s and %s (got %s)",
			minReattestInterval, maxReattestInterval, p.ReattestInterval)
	}
	if p.AcceptsIOS() {
		if err := p.Verifier().ValidateIOS(); err != nil {
			return err
		}
	}
	if p.AcceptsAndroid() {
		if err := p.Verifier().ValidateAndroid(); err != nil {
			return err
		}
	} else if p.Android.AllowGrapheneOS || len(p.Android.SigningCertDigests) > 0 || p.Android.MinVersionCode != 0 {
		return errors.New("an Android setting needs the Android package it applies to")
	}
	if p.Mode != AttestationOff && !p.AcceptsIOS() && !p.AcceptsAndroid() {
		return fmt.Errorf("mode %s needs an app to admit: an iOS team and bundle id, an Android package, or both", p.Mode)
	}
	return nil
}

// attestationPolicyColumns is the column list every read selects, in the
// order scanAttestationPolicy reads.
const attestationPolicyColumns = `p.mode, p.ios_team_id, p.ios_bundle_id, p.ios_allow_development,
	p.ios_min_bundle_version, p.android_package, p.android_signing_digests, p.android_allow_graphene,
	p.android_min_version_code, p.android_max_patch_age_months, p.reattest_interval_sec`

// scanAttestationPolicy reads attestationPolicyColumns from a row.
func scanAttestationPolicy(row []interface{}) (AttestationPolicy, error) {
	if len(row) < 11 {
		return AttestationPolicy{}, fmt.Errorf("an attestation policy row has %d columns, want 11", len(row))
	}
	mode, err := ParseAttestationMode(getStringVal(row[0]))
	if err != nil {
		return AttestationPolicy{}, err
	}
	var digests []string
	if raw := getStringVal(row[6]); raw != "" {
		if err := json.Unmarshal([]byte(raw), &digests); err != nil {
			return AttestationPolicy{}, fmt.Errorf("the stored signing-certificate digests are not a JSON list: %w", err)
		}
	}
	return AttestationPolicy{
		Mode: mode,
		IOS: IOSAttestation{
			TeamID:           getStringVal(row[1]),
			BundleID:         getStringVal(row[2]),
			AllowDevelopment: toInt64(row[3]) != 0,
			MinBundleVersion: getStringVal(row[4]),
		},
		Android: AndroidAttestation{
			Package:            getStringVal(row[5]),
			SigningCertDigests: digests,
			AllowGrapheneOS:    toInt64(row[7]) != 0,
			MinVersionCode:     toInt64(row[8]),
			MaxPatchAgeMonths:  int(toInt64(row[9])),
		},
		ReattestInterval: time.Duration(toInt64(row[10])) * time.Second,
	}, nil
}

// offPolicy is the policy of a namespace with no row.
func offPolicy() AttestationPolicy {
	return AttestationPolicy{Mode: AttestationOff, ReattestInterval: DefaultReattestInterval}
}

// AttestationPolicyOf returns a namespace's policy as recorded now. The paths
// that set it, attest a device or show it read this; a request is judged by
// the gateway's copy (AttestationPolicies).
func (s *Service) AttestationPolicyOf(ctx context.Context, namespace string) (AttestationPolicy, error) {
	namespace = sessionNamespace(namespace)
	if IsLobbyNamespace(namespace) {
		return offPolicy(), nil
	}
	db, nsID, err := s.deviceRegistry(ctx, namespace)
	if err != nil {
		return AttestationPolicy{}, err
	}
	res, err := db.Query(client.WithInternalAuth(ctx),
		`SELECT `+attestationPolicyColumns+` FROM namespace_attestation_policy AS p WHERE p.namespace_id = ? LIMIT 1`, nsID)
	if err != nil {
		return AttestationPolicy{}, fmt.Errorf("read the attestation policy of %q: %w", namespace, err)
	}
	if res == nil || len(res.Rows) == 0 {
		return offPolicy(), nil
	}
	return scanAttestationPolicy(res.Rows[0])
}

// SetAttestationPolicy records a namespace's policy after validating it, and
// applies it on this gateway at once; every other gateway applies it at its
// next reload of the policies, within RevocationStaleness.
func (s *Service) SetAttestationPolicy(ctx context.Context, namespace string, policy AttestationPolicy, actor string) error {
	policy = policy.Normalize()
	if err := policy.Validate(); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidAttestationPolicy, err)
	}
	namespace = sessionNamespace(namespace)
	if IsLobbyNamespace(namespace) {
		return ErrLobbyHasNoAttestationPolicy
	}
	if s.db == nil {
		return ErrRotationNotConfigured
	}
	_, nsID, err := s.deviceRegistry(ctx, namespace)
	if err != nil {
		return err
	}
	digests, err := json.Marshal(policy.Android.SigningCertDigests)
	if err != nil {
		return fmt.Errorf("encode the signing-certificate digests: %w", err)
	}
	if _, err := s.db.Exec(client.WithInternalAuth(ctx),
		`INSERT INTO namespace_attestation_policy(namespace_id, mode, ios_team_id, ios_bundle_id,
		   ios_allow_development, ios_min_bundle_version, android_package, android_signing_digests,
		   android_allow_graphene, android_min_version_code, android_max_patch_age_months,
		   reattest_interval_sec, updated_by, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'))
		 ON CONFLICT(namespace_id) DO UPDATE SET
		   mode = excluded.mode,
		   ios_team_id = excluded.ios_team_id,
		   ios_bundle_id = excluded.ios_bundle_id,
		   ios_allow_development = excluded.ios_allow_development,
		   ios_min_bundle_version = excluded.ios_min_bundle_version,
		   android_package = excluded.android_package,
		   android_signing_digests = excluded.android_signing_digests,
		   android_allow_graphene = excluded.android_allow_graphene,
		   android_min_version_code = excluded.android_min_version_code,
		   android_max_patch_age_months = excluded.android_max_patch_age_months,
		   reattest_interval_sec = excluded.reattest_interval_sec,
		   updated_by = excluded.updated_by,
		   updated_at = excluded.updated_at`,
		nsID, string(policy.Mode), policy.IOS.TeamID, policy.IOS.BundleID, boolInt(policy.IOS.AllowDevelopment),
		policy.IOS.MinBundleVersion, policy.Android.Package, string(digests), boolInt(policy.Android.AllowGrapheneOS),
		policy.Android.MinVersionCode, policy.Android.MaxPatchAgeMonths, int64(policy.ReattestInterval/time.Second),
		actor); err != nil {
		return fmt.Errorf("record the attestation policy of %q: %w", namespace, err)
	}
	s.attestationPolicies.recordLocal(namespace, policy)
	return nil
}

// AttestationPolicies returns this gateway's copy of every namespace's policy.
func (s *Service) AttestationPolicies() *AttestationPolicies { return s.attestationPolicies }

// AttestationPolicyFor is the policy a request to namespace is judged by,
// from this gateway's copy. ErrAttestationPoliciesUnavailable means the copy
// is too old and the registry cannot be read: the caller refuses with a
// retryable 503, because an unknown policy is not "off".
func (s *Service) AttestationPolicyFor(namespace string) (AttestationPolicy, error) {
	if IsLobbyNamespace(sessionNamespace(namespace)) {
		return offPolicy(), nil
	}
	return s.attestationPolicies.For(namespace)
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// OperatesNamespace reports whether a role is one that operates its namespace
// (owner, admin, developer). Such a wallet is held neither to the session
// policy nor to attestation: it signs in from the CLI, which is no app build,
// and whoever holds it can change either policy.
func OperatesNamespace(role Role) bool { return isControlPlaneRole(role) }

// HoldsControlPlane reports whether a permission set reaches anything beyond
// the data plane (DataPlanePermissions). A key that does is an operator's or
// a server's, never the kind an app ships; attestation does not judge it.
func HoldsControlPlane(perms PermissionSet) bool {
	for _, p := range perms {
		if !isDataPlanePermission(p) {
			return true
		}
	}
	return false
}

func isDataPlanePermission(p Permission) bool {
	for _, d := range DataPlanePermissions() {
		if p.Domain == d.Domain && (d.Action == ActionAny || p.Action == d.Action) {
			return true
		}
	}
	return false
}

// AttestationEnforced reports whether namespace refuses the callers its
// attestation policy does not admit. A function's capability mints ask it.
func (s *Service) AttestationEnforced(namespace string) (bool, error) {
	policy, err := s.AttestationPolicyFor(namespace)
	if err != nil {
		return false, err
	}
	return policy.Mode == AttestationEnforce, nil
}
