package traffic

import (
	"testing"
	"time"
)

// Refusals are counted by code, platform and mode, since the recorder started,
// and reported in a stable order.
func TestObserveAttestation_countsByCodePlatformAndMode(t *testing.T) {
	r := New(nil)
	if got := r.Snapshot(time.Now()).Attestation; got != nil {
		t.Fatalf("a fresh recorder reports attestation counts: %+v", got)
	}
	r.ObserveAttestation("ATTESTATION_REQUIRED", "none", "monitor")
	r.ObserveAttestation("ATTESTATION_REQUIRED", "none", "monitor")
	r.ObserveAttestation("APP_UPDATE_REQUIRED", "android", "enforce")

	got := r.Snapshot(time.Now().Add(time.Hour)).Attestation
	if len(got) != 2 {
		t.Fatalf("got %d rows, want 2: %+v", len(got), got)
	}
	if got[0].Mode != "enforce" || got[0].Code != "APP_UPDATE_REQUIRED" || got[0].Platform != "android" || got[0].Count != 1 {
		t.Errorf("first row %+v", got[0])
	}
	if got[1].Mode != "monitor" || got[1].Code != "ATTESTATION_REQUIRED" || got[1].Count != 2 {
		t.Errorf("second row %+v (counts outlive the window)", got[1])
	}
}
