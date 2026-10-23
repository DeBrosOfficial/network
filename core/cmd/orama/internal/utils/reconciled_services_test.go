package utils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Orama-Network/orama/core/pkg/systemd"
)

// bug-386: the shared TURN unit matches no orama-namespace-* name and is not
// in GetProductionServices (the lifecycle must not start or stop it), so the
// status surfaces list it from GetReconciledServices. It is there whenever its
// unit file is installed, wanted only while the node holds a TURN allocation.
func TestReconciledServices_listTheSharedTURNUnit(t *testing.T) {
	unitDir := t.TempDir()
	turnConfig := filepath.Join(t.TempDir(), "turn.yaml")

	if got := reconciledServices(unitDir, turnConfig); len(got) != 0 {
		t.Fatalf("no unit file installed: %+v, want none", got)
	}

	if err := os.WriteFile(filepath.Join(unitDir, systemd.HostTURNServiceName), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	got := reconciledServices(unitDir, turnConfig)
	if len(got) != 1 || got[0].Name != "orama-turn" || got[0].Wanted {
		t.Fatalf("unit installed, no TURN allocation: %+v, want orama-turn not wanted", got)
	}

	if err := os.WriteFile(turnConfig, []byte("tenants: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got = reconciledServices(unitDir, turnConfig)
	if len(got) != 1 || got[0].Name != "orama-turn" || !got[0].Wanted {
		t.Fatalf("unit installed, a TURN allocation: %+v, want orama-turn wanted", got)
	}
}

// The lifecycle list stays without it: start, stop, restart and upgrade
// enumerate GetProductionServices, and the TURN reconciler owns the unit.
func TestGetProductionServices_leavesTheSharedTURNUnitToItsReconciler(t *testing.T) {
	for _, svc := range GetProductionServices() {
		if svc+".service" == systemd.HostTURNServiceName {
			t.Fatalf("GetProductionServices lists %s, which the TURN reconciler owns", svc)
		}
	}
}
