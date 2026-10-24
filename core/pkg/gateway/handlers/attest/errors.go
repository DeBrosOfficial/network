package attest

import (
	"errors"
	"net/http"

	"github.com/Orama-Network/orama/core/pkg/attest"
	authsvc "github.com/Orama-Network/orama/core/pkg/gateway/auth"
)

// The refusals of the attestation routes. A refused proof says only what it
// means to the app — invalid, or the build is too old — and which check
// refused it goes to the log, never to the caller: telling a forger which
// check it failed is a step-by-step guide.
const (
	// CodeAttestationInvalid: the proof does not show the namespace's own app
	// on a genuine device (or the device's pinned OS key changed).
	CodeAttestationInvalid = "ATTESTATION_INVALID"
	// CodeAppUpdateRequired: the proof is genuine and the build is older than
	// the namespace admits.
	CodeAppUpdateRequired = "APP_UPDATE_REQUIRED"
	// CodeAttestationChallengeInvalid: the challenge was never issued to this
	// device, was already used, or expired. Ask for a new one.
	CodeAttestationChallengeInvalid = "ATTESTATION_CHALLENGE_INVALID"
	// CodeAttestationNotConfigured: the namespace's attestation mode is off;
	// there is no app to attest against.
	CodeAttestationNotConfigured = "ATTESTATION_NOT_CONFIGURED"
	// CodeAttestationNeedsDevice: the session is bound to no device, or the
	// caller is a key. A proof is bound to a device.
	CodeAttestationNeedsDevice = "ATTESTATION_NEEDS_DEVICE"
	// CodeAttestationMalformed: the request is not one the route reads.
	CodeAttestationMalformed = "ATTESTATION_MALFORMED"
	// CodeAttestationPolicyInvalid: a policy PUT that cannot be enforced as
	// written (an incomplete platform, a malformed digest or version).
	CodeAttestationPolicyInvalid = "ATTESTATION_POLICY_INVALID"
	// CodeAttestationChallengeLimit: the device holds as many unanswered
	// challenges as it may. Answer one, or wait for them to expire.
	CodeAttestationChallengeLimit = "ATTESTATION_CHALLENGE_LIMIT"
)

// refusal is how one kind of error is answered.
type refusal struct {
	status int
	code   string
	// text is what the caller reads; empty means the error's own text.
	text string
}

func (r refusal) message(err error) string {
	if r.text != "" {
		return r.text
	}
	return err.Error()
}

// refusalFor maps an error from the auth service to its answer, or false for
// an error that is the server's own (answered 503).
func refusalFor(err error) (refusal, bool) {
	switch attest.KindOf(err) {
	case attest.KindInvalid:
		return refusal{http.StatusForbidden, CodeAttestationInvalid,
			"this proof does not show this namespace's app on a genuine device"}, true
	case attest.KindAppUpdateRequired:
		return refusal{http.StatusForbidden, CodeAppUpdateRequired,
			"this build is older than the namespace admits: update the app"}, true
	}
	var tooMany *authsvc.ErrTooManyOutstandingNonces
	switch {
	case errors.Is(err, authsvc.ErrNonceInvalid):
		return refusal{http.StatusUnauthorized, CodeAttestationChallengeInvalid,
			"the challenge is unknown, used or expired: ask for a new one"}, true
	case errors.Is(err, authsvc.ErrAttestationNotConfigured):
		return refusal{http.StatusConflict, CodeAttestationNotConfigured, ""}, true
	case errors.Is(err, authsvc.ErrAttestationNeedsDevice):
		return refusal{http.StatusForbidden, CodeAttestationNeedsDevice, ""}, true
	case errors.As(err, &tooMany):
		return refusal{http.StatusTooManyRequests, CodeAttestationChallengeLimit,
			"this device already holds as many unanswered challenges as it may; answer one or wait for them to expire"}, true
	}
	return refusal{}, false
}
