package attest

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	authsvc "github.com/Orama-Network/orama/core/pkg/gateway/auth"
	"github.com/Orama-Network/orama/core/pkg/gateway/ctxkeys"
)

// GET /v1/namespace/attestation-policy   the namespace's attestation policy
// PUT /v1/namespace/attestation-policy   change it
//
// A PUT carries the fields it changes and leaves the rest as they are:
//
//	{"mode": "off"|"monitor"|"enforce",
//	 "ios": {"team_id", "bundle_id", "allow_development", "min_bundle_version"},
//	 "android": {"package", "signing_cert_digests", "allow_grapheneos",
//	             "min_version_code", "max_patch_age_months"},
//	 "reattest_interval": "168h"}
//
// The result is validated whole before it is written. The route policy
// requires the namespace-write permission; the namespace is the caller's
// credential's, never the request's.

// policyWire is a policy as the API reads and writes it.
type policyWire struct {
	authsvc.AttestationPolicy
	ReattestInterval string `json:"reattest_interval"`
}

func toWire(p authsvc.AttestationPolicy) policyWire {
	if p.Android.SigningCertDigests == nil {
		p.Android.SigningCertDigests = []string{}
	}
	return policyWire{AttestationPolicy: p, ReattestInterval: p.ReattestInterval.String()}
}

// PolicyHandler reads or sets the calling namespace's attestation policy.
func (h *Handlers) PolicyHandler(w http.ResponseWriter, r *http.Request) {
	namespace, _ := r.Context().Value(ctxkeys.NamespaceOverride).(string)
	if authsvc.IsLobbyNamespace(namespace) || namespace == "" {
		writeError(w, http.StatusForbidden, "", authsvc.ErrLobbyHasNoAttestationPolicy.Error())
		return
	}
	switch r.Method {
	case http.MethodGet:
		policy, err := h.service.AttestationPolicyOf(r.Context(), namespace)
		if err != nil {
			h.writeServiceError(w, "read the attestation policy", err)
			return
		}
		h.writePolicy(w, namespace, policy)
	case http.MethodPut:
		h.setPolicy(w, r, namespace)
	default:
		writeError(w, http.StatusMethodNotAllowed, "", "GET to read, PUT to set")
	}
}

func (h *Handlers) setPolicy(w http.ResponseWriter, r *http.Request, namespace string) {
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxPolicyBody))
	if err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, CodeAttestationMalformed, "the request body is too large")
		return
	}
	current, err := h.service.AttestationPolicyOf(r.Context(), namespace)
	if err != nil {
		h.writeServiceError(w, "read the attestation policy", err)
		return
	}
	// Decoding onto the current policy is what leaves out a field the request
	// does not name. Unknown fields are refused: a misspelt field changing
	// nothing would read as success.
	wire := toWire(current)
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&wire); err != nil {
		writeError(w, http.StatusBadRequest, CodeAttestationMalformed, "invalid policy: "+err.Error())
		return
	}
	policy := wire.AttestationPolicy
	if policy.ReattestInterval, err = time.ParseDuration(wire.ReattestInterval); err != nil {
		writeError(w, http.StatusBadRequest, CodeAttestationMalformed, `reattest_interval is a duration such as "168h"`)
		return
	}
	actor := authsvc.ActorFromRequest(r)
	if err := h.service.SetAttestationPolicy(r.Context(), namespace, policy, actor); err != nil {
		if errors.Is(err, authsvc.ErrInvalidAttestationPolicy) {
			writeError(w, http.StatusBadRequest, CodeAttestationPolicyInvalid, err.Error())
			return
		}
		h.writeServiceError(w, "record the attestation policy", err)
		return
	}
	if h.audit != nil {
		h.audit.RecordFromRequest(r.Context(), r, authsvc.AuditEvent{
			Namespace: namespace,
			Actor:     actor,
			Action:    authsvc.AuditAttestationPolicySet,
			Result:    authsvc.AuditSuccess,
			Resource:  string(policy.Normalize().Mode),
		})
	}
	stored, err := h.service.AttestationPolicyOf(r.Context(), namespace)
	if err != nil {
		h.writeServiceError(w, "read the attestation policy back", err)
		return
	}
	h.writePolicy(w, namespace, stored)
}

func (h *Handlers) writePolicy(w http.ResponseWriter, namespace string, policy authsvc.AttestationPolicy) {
	writeJSON(w, http.StatusOK, struct {
		Namespace string `json:"namespace"`
		policyWire
	}{namespace, toWire(policy)})
}
