package hostfunctions

import (
	"context"
	"errors"
	"fmt"
)

// A capability (feat-264) and a storage fetch capability (bugboard #266) carry
// no credential when they are used: whoever holds one opens what it names. In
// a namespace that enforces device attestation (feat-941) that would be a way
// round it, so the gate is at the mint: only an invocation whose caller is an
// attested device mints one. A caller the attestation gate exempts (an
// operator, a workload, a control-plane key) and a gateway-started invocation
// (cron, a trigger) carry no attestation, and mint none there.

// AttestationEnforcement says whether a namespace enforces device attestation.
// *auth.Service is the one the gateway uses.
type AttestationEnforcement interface {
	AttestationEnforced(namespace string) (bool, error)
}

var errMintNeedsAttestation = errors.New("this namespace admits only its attested app: a capability is minted only for an attested caller")

// SetAttestationEnforcement wires what tells whether a namespace enforces
// attestation. Called once at gateway start, before any function runs.
func (h *HostFunctions) SetAttestationEnforcement(a AttestationEnforcement) {
	h.attestation = a
}

// requireAttestedMint refuses a mint in a namespace that enforces attestation
// for an invocation whose caller is not an attested device.
func (h *HostFunctions) requireAttestedMint(ctx context.Context) error {
	cur := h.currentInvocationContext(ctx)
	if h.attestation == nil || cur == nil || cur.CallerAttestation != nil {
		return nil
	}
	enforced, err := h.attestation.AttestationEnforced(cur.Namespace)
	if err != nil {
		return fmt.Errorf("cannot tell whether this namespace enforces attestation: %w", err)
	}
	if enforced {
		return errMintNeedsAttestation
	}
	return nil
}
