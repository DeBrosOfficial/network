// Package refusal is the error type every device-attestation verifier
// returns when it refuses a proof. It is a leaf package so that the platform
// verifiers (attest/apple, attest/android) and the top-level attest API share
// one type without an import cycle; callers use it through package attest.
package refusal

import (
	"errors"
	"fmt"
)

// Kind is what a refusal means to the client: the gateway maps it to the
// named error code the app sees.
type Kind int

const (
	// KindNone is not a refusal: the error is the server's own (a bad
	// policy, a missing input), and the client did nothing wrong.
	KindNone Kind = iota
	// KindInvalid means the proof does not show a genuine app build on a
	// genuine device (ATTESTATION_INVALID).
	KindInvalid
	// KindAppUpdateRequired means the proof is genuine but the app build is
	// older than the namespace accepts (APP_UPDATE_REQUIRED).
	KindAppUpdateRequired
)

func (k Kind) String() string {
	switch k {
	case KindInvalid:
		return "invalid"
	case KindAppUpdateRequired:
		return "app_update_required"
	default:
		return "none"
	}
}

// Reason is the stable, low-cardinality label of the check that refused a
// proof. Operators count refusals by it, so the set is closed.
type Reason string

const (
	ReasonMalformed           Reason = "malformed"
	ReasonChain               Reason = "chain"
	ReasonRevoked             Reason = "revoked"
	ReasonRevocationUnknown   Reason = "revocation_unknown"
	ReasonNonce               Reason = "nonce"
	ReasonKeyID               Reason = "key_id"
	ReasonAppID               Reason = "app_id"
	ReasonCounter             Reason = "counter"
	ReasonEnvironment         Reason = "environment"
	ReasonSignature           Reason = "signature"
	ReasonValidationCategory  Reason = "validation_category"
	ReasonSecurityLevel       Reason = "security_level"
	ReasonChallenge           Reason = "challenge"
	ReasonPackage             Reason = "package"
	ReasonSigningCert         Reason = "signing_cert"
	ReasonDeviceUnlocked      Reason = "device_unlocked"
	ReasonBootState           Reason = "boot_state"
	ReasonBootKey             Reason = "boot_key"
	ReasonPatchLevel          Reason = "patch_level"
	ReasonAppVersion          Reason = "app_version"
	ReasonAppVersionMalformed Reason = "app_version_malformed"
)

// Error is a refused proof: what it means (Kind), which check refused it
// (Reason), and the detail for the log (Err).
type Error struct {
	Kind   Kind
	Reason Reason
	Err    error
}

func (e *Error) Error() string {
	return fmt.Sprintf("attestation refused (%s, %s): %v", e.Kind, e.Reason, e.Err)
}

func (e *Error) Unwrap() error { return e.Err }

// Invalid builds a KindInvalid refusal.
func Invalid(reason Reason, format string, args ...any) error {
	return &Error{Kind: KindInvalid, Reason: reason, Err: fmt.Errorf(format, args...)}
}

// UpdateRequired builds a KindAppUpdateRequired refusal.
func UpdateRequired(format string, args ...any) error {
	return &Error{Kind: KindAppUpdateRequired, Reason: ReasonAppVersion, Err: fmt.Errorf(format, args...)}
}

// As returns the refusal inside err, if there is one.
func As(err error) (*Error, bool) {
	var r *Error
	if errors.As(err, &r) {
		return r, true
	}
	return nil, false
}
