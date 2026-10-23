package migrations_test

// Migration 084 takes the storage grant off the keys minted from the
// app-runtime profile (bugboard #946): that profile is for the key an app
// ships, and storage on a key reaches unpin.

import (
	"database/sql"
	"testing"

	"github.com/Orama-Network/orama/core/migrations"
	"github.com/Orama-Network/orama/core/pkg/gateway/auth"
	"github.com/Orama-Network/orama/core/pkg/rqlite"
	"go.uber.org/zap"
)

// appRuntimeBefore084 is what every app-runtime mint wrote before 084.
const appRuntimeBefore084 = "invoke,proxy,push,storage,webrtc"

// registryWithAppRuntimeKeys is a registry holding an app-runtime key, a
// revoked one, a function's storage key and a key that holds storage among
// other grants, minted before 084, with 084 then applied.
func registryWithAppRuntimeKeys(t *testing.T) *sql.DB {
	t.Helper()
	db := openRoundtripDB(t)

	if err := rqlite.ApplyEmbeddedMigrations(t.Context(), db, migrationsBefore(t, "084"), zap.NewNop()); err != nil {
		t.Fatalf("apply migrations before 084: %v", err)
	}
	if _, err := db.Exec(`INSERT OR IGNORE INTO namespaces(id, name) VALUES (10, 'anchat')`); err != nil {
		t.Fatalf("seed namespace: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO api_keys(id, key, name, namespace_id, scopes, expires_at, revoked_at) VALUES
		(400, 'hash-bundle', 'web', 10, ?, datetime('now', '+30 days'), NULL),
		(401, 'hash-revoked', 'old-web', 10, ?, datetime('now', '+30 days'), datetime('now', '-1 day')),
		(402, 'hash-fn', 'prune', 10, 'storage', datetime('now', '+30 days'), NULL),
		(403, 'hash-mixed', 'server', 10, 'invoke,storage', datetime('now', '+30 days'), NULL)`,
		appRuntimeBefore084, appRuntimeBefore084); err != nil {
		t.Fatalf("seed keys: %v", err)
	}

	if err := rqlite.ApplyEmbeddedMigrations(t.Context(), db, migrations.FS, zap.NewNop()); err != nil {
		t.Fatalf("apply 084: %v", err)
	}
	return db
}

func scopesOf(t *testing.T, db *sql.DB, id int) string {
	t.Helper()
	var scopes string
	if err := db.QueryRow(`SELECT scopes FROM api_keys WHERE id = ?`, id).Scan(&scopes); err != nil {
		t.Fatalf("read key %d: %v", id, err)
	}
	return scopes
}

// A live app-runtime key ends up holding exactly what the profile mints now,
// so a key minted before the change and one minted after are the same key.
func TestMigration084_appRuntimeKeyLosesStorage(t *testing.T) {
	db := registryWithAppRuntimeKeys(t)

	want, err := auth.NormalizeGrants("app-runtime")
	if err != nil {
		t.Fatalf("normalize app-runtime: %v", err)
	}
	if got := scopesOf(t, db, 400); got != want {
		t.Errorf("an app-runtime key holds %q after 084, want the profile's %q", got, want)
	}
	if auth.ParseScopes(scopesOf(t, db, 400)).Has(auth.ScopeStorage) {
		t.Error("an app-runtime key still holds storage")
	}
}

// Only the profile's exact set moves: a function's storage key is what #151
// unpins with, and an explicit list is what its minter asked for.
func TestMigration084_keepsEveryOtherKey(t *testing.T) {
	db := registryWithAppRuntimeKeys(t)

	for id, want := range map[int]string{
		401: appRuntimeBefore084, // revoked: the record of what it could do
		402: "storage",
		403: "invoke,storage",
	} {
		if got := scopesOf(t, db, id); got != want {
			t.Errorf("key %d holds %q after 084, want %q", id, got, want)
		}
	}
}

func TestMigration084_replayIsANoOp(t *testing.T) {
	db := registryWithAppRuntimeKeys(t)
	before := scopesOf(t, db, 400)

	if _, err := db.Exec(`DELETE FROM schema_migrations WHERE version = 84`); err != nil {
		t.Fatalf("forget 084: %v", err)
	}
	if err := rqlite.ApplyEmbeddedMigrations(t.Context(), db, migrations.FS, zap.NewNop()); err != nil {
		t.Fatalf("replay 084: %v", err)
	}
	if got := scopesOf(t, db, 400); got != before {
		t.Errorf("replaying 084 changed the key: %q, was %q", got, before)
	}
}
