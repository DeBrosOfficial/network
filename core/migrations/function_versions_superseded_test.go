package migrations_test

import (
	"io/fs"
	"testing"

	"github.com/Orama-Network/orama/core/migrations"
	"github.com/Orama-Network/orama/core/pkg/rqlite"
	"go.uber.org/zap"
)

// supersededMigration marks every non-current function version superseded.
const supersededMigration = "086_function_versions_superseded.sql"

// Migration 086 (bug-939, bug-940): of each function's versions that can run,
// the highest keeps its status and every lower one becomes superseded, so
// name@N of replaced code stops resolving. Deleted rows are untouched, and a
// disabled function keeps exactly one version to enable.
func TestSupersededMigration_leavesOneCurrentVersionPerFunction(t *testing.T) {
	db := openRoundtripDB(t)
	if err := rqlite.ApplyEmbeddedMigrations(t.Context(), db, migrationsBefore(t, supersededMigration), zap.NewNop()); err != nil {
		t.Fatalf("apply migrations before 086: %v", err)
	}
	seed := []struct {
		id, name, ns string
		version      int
		status       string
	}{
		{"live1", "live", "ns", 1, "active"},
		{"live2", "live", "ns", 2, "active"},
		{"live3", "live", "ns", 3, "active"},
		{"off1", "off", "ns", 1, "inactive"},
		{"off2", "off", "ns", 2, "inactive"},
		{"gone1", "gone", "ns", 1, "deleted"},
		{"gone2", "gone", "ns", 2, "deleted"},
		{"top1", "topdel", "ns", 1, "active"},
		{"top2", "topdel", "ns", 2, "active"},
		{"top3", "topdel", "ns", 3, "deleted"},
		{"other1", "live", "other", 1, "active"},
	}
	for _, r := range seed {
		if _, err := db.Exec(`INSERT INTO functions (id, name, namespace, version, wasm_cid, created_by, status) VALUES (?, ?, ?, ?, 'cid', ?, ?)`,
			r.id, r.name, r.ns, r.version, r.ns, r.status); err != nil {
			t.Fatalf("seed %s: %v", r.id, err)
		}
	}

	for run := 1; run <= 2; run++ {
		if err := rqlite.ApplyEmbeddedMigrations(t.Context(), db, migrations.FS, zap.NewNop()); err != nil {
			t.Fatalf("apply 086 (run %d): %v", run, err)
		}
		if run == 2 {
			// A replay of the file itself, past the runner's bookkeeping.
			data, err := fs.ReadFile(migrations.FS, supersededMigration)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(string(data)); err != nil {
				t.Fatalf("replay 086: %v", err)
			}
		}
		want := map[string]string{
			"live1": "superseded", "live2": "superseded", "live3": "active",
			"off1": "superseded", "off2": "inactive",
			"gone1": "deleted", "gone2": "deleted",
			"top1": "superseded", "top2": "active", "top3": "deleted",
			"other1": "active",
		}
		for id, status := range want {
			var got string
			if err := db.QueryRow(`SELECT status FROM functions WHERE id = ?`, id).Scan(&got); err != nil {
				t.Fatalf("read %s: %v", id, err)
			}
			if got != status {
				t.Errorf("run %d: %s is %q; want %q", run, id, got, status)
			}
		}
	}
}
