package report

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Orama-Network/orama/core/pkg/systemd"
)

// bug-386: the node report (what orama status and orama monitor read) carries
// the shared TURN unit when it is installed, NotNeeded on a node with no TURN
// allocation.
func TestCollectHostTURN(t *testing.T) {
	unitDir := t.TempDir()
	turnConfig := filepath.Join(t.TempDir(), "turn.yaml")
	ctx := context.Background()

	if _, ok := collectHostTURN(ctx, unitDir, turnConfig); ok {
		t.Fatal("reported a TURN unit that is not installed")
	}
	if err := os.WriteFile(filepath.Join(unitDir, systemd.HostTURNServiceName), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	info, ok := collectHostTURN(ctx, unitDir, turnConfig)
	if !ok || info.Name != "orama-turn" || !info.NotNeeded {
		t.Fatalf("installed, no allocation: %+v %v, want orama-turn NotNeeded", info, ok)
	}
	if err := os.WriteFile(turnConfig, []byte("tenants: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if info, _ = collectHostTURN(ctx, unitDir, turnConfig); info.NotNeeded {
		t.Fatalf("installed with an allocation: %+v, want it needed", info)
	}
}
