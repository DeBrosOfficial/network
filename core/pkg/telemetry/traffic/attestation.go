package traffic

import (
	"sort"

	"github.com/Orama-Network/orama/core/pkg/telemetry/report"
)

// attestationKey is one row of the attestation refusal counts. Every field
// takes a handful of values the gateway sets — three codes, three platforms
// (ios, android, none), two modes — so the counts stay a few rows whatever
// clients send.
type attestationKey struct {
	code, platform, mode string
}

// ObserveAttestation counts a request device attestation refused, or in
// monitor mode would have refused: its code, the platform the caller's
// attestation named ("none" when it carried none) and the namespace's mode.
// The counts run since the gateway started, like TotalRequests, so a rise is
// visible however seldom it is sampled.
func (r *Recorder) ObserveAttestation(code, platform, mode string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.attestation == nil {
		r.attestation = map[attestationKey]int64{}
	}
	r.attestation[attestationKey{code, platform, mode}]++
}

// attestationReport lists the counts, ordered by mode, code and platform so
// the order is stable between snapshots. The caller holds r.mu.
func (r *Recorder) attestationReport() []report.AttestationRefusals {
	if len(r.attestation) == 0 {
		return nil
	}
	out := make([]report.AttestationRefusals, 0, len(r.attestation))
	for k, n := range r.attestation {
		out = append(out, report.AttestationRefusals{Code: k.code, Platform: k.platform, Mode: k.mode, Count: n})
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Mode != b.Mode {
			return a.Mode < b.Mode
		}
		if a.Code != b.Code {
			return a.Code < b.Code
		}
		return a.Platform < b.Platform
	})
	return out
}
