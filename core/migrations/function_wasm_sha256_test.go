package migrations_test

import (
	"strings"
	"testing"

	"github.com/Orama-Network/orama/core/migrations"
	"github.com/Orama-Network/orama/core/pkg/rqlite"
	"go.uber.org/zap"
)

// wasmSHA256Migration adds functions.wasm_sha256.
const wasmSHA256Migration = "093_function_wasm_sha256.sql"

// Migration 093 (chg-956): a row deployed before it keeps a NULL hash, and the
// column holds only NULL or a 64-character lowercase hex digest. A replay of
// the migrations changes nothing.
func TestWASMSHA256Migration_nullForOldRowsAndOnlyADigest(t *testing.T) {
	db := openRoundtripDB(t)
	if err := rqlite.ApplyEmbeddedMigrations(t.Context(), db, migrationsBefore(t, wasmSHA256Migration), zap.NewNop()); err != nil {
		t.Fatalf("apply migrations before 093: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO functions (id, name, namespace, version, wasm_cid, created_by) VALUES ('old', 'fn', 'ns', 1, 'cid', 'ns')`); err != nil {
		t.Fatalf("seed: %v", err)
	}
	for run := 1; run <= 2; run++ {
		if err := rqlite.ApplyEmbeddedMigrations(t.Context(), db, migrations.FS, zap.NewNop()); err != nil {
			t.Fatalf("apply 093 (run %d): %v", run, err)
		}
	}

	var isNull bool
	if err := db.QueryRow(`SELECT wasm_sha256 IS NULL FROM functions WHERE id = 'old'`).Scan(&isNull); err != nil {
		t.Fatal(err)
	}
	if !isNull {
		t.Error("a row deployed before 093 has a wasm_sha256; want NULL")
	}

	digest := strings.Repeat("0123456789abcdef", 4)
	insert := `INSERT INTO functions (id, name, namespace, version, wasm_cid, wasm_sha256, created_by) VALUES (?, 'fn', 'ns', ?, 'cid', ?, 'ns')`
	if _, err := db.Exec(insert, "ok", 2, digest); err != nil {
		t.Fatalf("a lowercase hex digest was refused: %v", err)
	}
	for i, bad := range []string{"", strings.ToUpper(digest), digest[:63], digest + "0", strings.Repeat("g", 64)} {
		if _, err := db.Exec(insert, "bad", 3+i, bad); err == nil {
			t.Errorf("wasm_sha256 %q was stored", bad)
		}
	}
}
