// Package attest serves device attestation (feat-941): a device's challenge,
// its iOS or Android proof, and the namespace's attestation policy.
//
//	POST /v1/attest/challenge             a single-use challenge for the caller's device
//	POST /v1/attest/ios                   an App Attest attestation over it
//	POST /v1/attest/android               a key-attestation chain over it
//	GET  /v1/namespace/attestation-policy the namespace's policy
//	PUT  /v1/namespace/attestation-policy set it (namespace write)
//
// The three attest routes take the caller's session token: a proof is bound to
// the device the session is bound to (its `did`), so the device signs in with
// its device key first. An accepted proof answers with a fresh access token
// carrying the attestation, which the namespace's policy then admits. Checking
// a proof is package attest, recording it package gateway/auth; this package
// reads requests and answers them.
package attest

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	authsvc "github.com/Orama-Network/orama/core/pkg/gateway/auth"
	"github.com/Orama-Network/orama/core/pkg/gateway/ctxkeys"
	"github.com/Orama-Network/orama/core/pkg/logging"
	"go.uber.org/zap"
)

const (
	// maxAttestBody bounds an attestation request. An App Attest object is a
	// few kilobytes and an Android chain is four or five certificates of about
	// one each; 64 KiB is several times either.
	maxAttestBody = 64 << 10
	// maxPolicyBody bounds a policy PUT: a handful of names and digests.
	maxPolicyBody = 16 << 10
	// maxChainLength is the most certificates an Android chain may have. Real
	// chains are four or five (leaf, intermediates, root); a longer one is
	// refused before any is parsed.
	maxChainLength = 10
)

// Service is what the handlers need of the auth service.
type Service interface {
	IssueAttestationChallenge(ctx context.Context, namespace, deviceID string) (string, time.Time, error)
	AttestIOS(ctx context.Context, namespace, deviceID, challenge string, keyID, attestationObject []byte) (*authsvc.AttestationClaim, error)
	AttestAndroid(ctx context.Context, namespace, deviceID, challenge string, chain [][]byte) (*authsvc.AttestationClaim, error)
	ReissueAttested(claims *authsvc.JWTClaims, att *authsvc.AttestationClaim) (string, int64, error)
	AttestationPolicyOf(ctx context.Context, namespace string) (authsvc.AttestationPolicy, error)
	SetAttestationPolicy(ctx context.Context, namespace string, policy authsvc.AttestationPolicy, actor string) error
}

// Auditor records a policy change. *authsvc.AuditLog is the one the gateway
// uses.
type Auditor interface {
	RecordFromRequest(ctx context.Context, r *http.Request, event authsvc.AuditEvent)
}

// Handlers serves the attestation routes.
type Handlers struct {
	service Service
	audit   Auditor
	logger  *logging.ColoredLogger
}

// NewHandlers builds the handlers.
func NewHandlers(service Service, audit Auditor, logger *logging.ColoredLogger) *Handlers {
	return &Handlers{service: service, audit: audit, logger: logger}
}

// session is the caller's verified token and namespace, or an answered
// request.
func (h *Handlers) session(w http.ResponseWriter, r *http.Request) (*authsvc.JWTClaims, string, bool) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "", "POST only")
		return nil, "", false
	}
	claims, _ := r.Context().Value(ctxkeys.JWT).(*authsvc.JWTClaims)
	if claims == nil || authsvc.IsAPIKeySubject(claims.Sub) {
		writeError(w, http.StatusUnauthorized, CodeAttestationNeedsDevice,
			"attestation is made by a signed-in device: send its session token, not an API key")
		return nil, "", false
	}
	if strings.TrimSpace(claims.Did) == "" {
		writeError(w, http.StatusForbidden, CodeAttestationNeedsDevice, authsvc.ErrAttestationNeedsDevice.Error())
		return nil, "", false
	}
	namespace, _ := r.Context().Value(ctxkeys.NamespaceOverride).(string)
	if namespace == "" {
		namespace = claims.Namespace
	}
	return claims, namespace, true
}

// ChallengeHandler issues a challenge for the caller's device.
//
// Response: {"challenge", "expires_at", "device_id"}. The device's platform
// signs over SHA-256(challenge ‖ device_id): App Attest's clientDataHash on
// iOS, the key's attestationChallenge on Android.
func (h *Handlers) ChallengeHandler(w http.ResponseWriter, r *http.Request) {
	claims, namespace, ok := h.session(w, r)
	if !ok {
		return
	}
	challenge, expires, err := h.service.IssueAttestationChallenge(r.Context(), namespace, claims.Did)
	if err != nil {
		h.writeServiceError(w, "issue an attestation challenge", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"challenge":  challenge,
		"expires_at": expires.UTC().Format(time.RFC3339),
		"device_id":  claims.Did,
	})
}

// IOSRequest is an App Attest attestation: the key id DCAppAttestService
// returned and the attestation object, both standard base64.
type IOSRequest struct {
	Challenge   string `json:"challenge"`
	KeyID       string `json:"key_id"`
	Attestation string `json:"attestation"`
}

// IOSHandler checks an App Attest attestation and records the device's key.
func (h *Handlers) IOSHandler(w http.ResponseWriter, r *http.Request) {
	claims, namespace, ok := h.session(w, r)
	if !ok {
		return
	}
	var req IOSRequest
	if !decode(w, r, maxAttestBody, &req) {
		return
	}
	keyID, err1 := base64.StdEncoding.DecodeString(req.KeyID)
	object, err2 := base64.StdEncoding.DecodeString(req.Attestation)
	if req.Challenge == "" || err1 != nil || err2 != nil || len(keyID) == 0 || len(object) == 0 {
		writeError(w, http.StatusBadRequest, CodeAttestationMalformed,
			`send {"challenge", "key_id", "attestation"}, key_id and attestation in standard base64`)
		return
	}
	att, err := h.service.AttestIOS(r.Context(), namespace, claims.Did, req.Challenge, keyID, object)
	h.answerAttestation(w, claims, att, err)
}

// AndroidRequest is a key-attestation certificate chain, leaf first, each
// certificate DER in standard base64.
type AndroidRequest struct {
	Challenge string   `json:"challenge"`
	Chain     []string `json:"chain"`
}

// AndroidHandler checks an Android key-attestation chain.
func (h *Handlers) AndroidHandler(w http.ResponseWriter, r *http.Request) {
	claims, namespace, ok := h.session(w, r)
	if !ok {
		return
	}
	var req AndroidRequest
	if !decode(w, r, maxAttestBody, &req) {
		return
	}
	if req.Challenge == "" || len(req.Chain) == 0 || len(req.Chain) > maxChainLength {
		writeError(w, http.StatusBadRequest, CodeAttestationMalformed,
			`send {"challenge", "chain"}: the key-attestation chain, leaf first, at most 10 certificates`)
		return
	}
	chain := make([][]byte, 0, len(req.Chain))
	for _, cert := range req.Chain {
		der, err := base64.StdEncoding.DecodeString(cert)
		if err != nil || len(der) == 0 {
			writeError(w, http.StatusBadRequest, CodeAttestationMalformed, "every certificate of the chain is DER in standard base64")
			return
		}
		chain = append(chain, der)
	}
	att, err := h.service.AttestAndroid(r.Context(), namespace, claims.Did, req.Challenge, chain)
	h.answerAttestation(w, claims, att, err)
}

// answerAttestation answers an attestation with a token carrying it, or with
// why it was refused.
func (h *Handlers) answerAttestation(w http.ResponseWriter, claims *authsvc.JWTClaims, att *authsvc.AttestationClaim, err error) {
	if err != nil {
		h.writeServiceError(w, "verify the attestation", err)
		return
	}
	token, exp, err := h.service.ReissueAttested(claims, att)
	if err != nil {
		h.writeServiceError(w, "mint the attested token", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token": token,
		"token_type":   "Bearer",
		"expires_in":   max(exp-time.Now().Unix(), 0),
		"attestation":  attestationBody(att),
	})
}

// attestationBody is what an attestation proved, as the API shows it.
func attestationBody(att *authsvc.AttestationClaim) map[string]any {
	return map[string]any{
		"platform":    att.Platform,
		"app_id":      att.AppID,
		"app_version": att.AppVersion,
		"attested_at": time.Unix(att.AttestedAt, 0).UTC().Format(time.RFC3339),
	}
}

// writeServiceError answers an error from the auth service: a refusal by its
// name, the registry's failure as a retry.
func (h *Handlers) writeServiceError(w http.ResponseWriter, what string, err error) {
	if refusal, ok := refusalFor(err); ok {
		writeError(w, refusal.status, refusal.code, refusal.message(err))
		return
	}
	if h.logger != nil {
		h.logger.ComponentWarn(logging.ComponentGeneral, "could not "+what, zap.Error(err))
	}
	writeError(w, http.StatusServiceUnavailable, "", "could not "+what+"; retry")
}

// decode reads a JSON body of at most limit bytes into v, or answers.
func decode(w http.ResponseWriter, r *http.Request, limit int64, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeError(w, http.StatusRequestEntityTooLarge, CodeAttestationMalformed, "the request body is too large")
			return false
		}
		writeError(w, http.StatusBadRequest, CodeAttestationMalformed, "invalid json body")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes an error body, with its code when it has one.
func writeError(w http.ResponseWriter, status int, code, message string) {
	body := map[string]any{"error": message}
	if code != "" {
		body["code"] = code
	}
	writeJSON(w, status, body)
}
