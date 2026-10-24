package migrations_test

// Migration 088 makes revoked_tokens.jti unique and gives capability
// revocations a namespace (bug-750, bug-436). The table it runs over may hold
// the duplicates concurrent revokes wrote, which the UNIQUE index cannot be
// built over, and capability rows written before the column, which the
// namespace ceiling must count.

import (
	"testing"

	"github.com/Orama-Network/orama/core/migrations"
	"github.com/Orama-Network/orama/core/pkg/rqlite"
	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"
)

func TestMigration088_dedupesThenMakesJTIUniqueAndBackfillsNamespaces(t *testing.T) {
	db := openRoundtripDB(t)
	if err := rqlite.ApplyEmbeddedMigrations(t.Context(), db, migrationsBefore(t, "088"), zap.NewNop()); err != nil {
		t.Fatalf("apply migrations before 088: %v", err)
	}
	for _, row := range []struct {
		jti     any
		subject any
		expires int64
	}{
		{"cap:anchat:a1", nil, 100},
		{"cap:anchat:a1", nil, 300}, // the one that lasts longest is kept
		{"cap:anchat:a1", nil, 200},
		{"jti-session", nil, 50},
		{"jti-session", nil, 50}, // equals: the newest is kept
		{nil, "0xwallet", 400},
		{nil, "0xwallet", 400}, // subject rows carry no jti and are left alone
	} {
		if _, err := db.Exec(`INSERT INTO revoked_tokens(jti, subject, expires_at) VALUES (?, ?, ?)`,
			row.jti, row.subject, row.expires); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	if err := rqlite.ApplyEmbeddedMigrations(t.Context(), db, migrations.FS, zap.NewNop()); err != nil {
		t.Fatalf("apply 088 over duplicates: %v", err)
	}

	var n int
	var expires int64
	var namespace string
	if err := db.QueryRow(`SELECT COUNT(*), MAX(expires_at), MAX(namespace) FROM revoked_tokens WHERE jti = 'cap:anchat:a1'`).
		Scan(&n, &expires, &namespace); err != nil {
		t.Fatal(err)
	}
	if n != 1 || expires != 300 {
		t.Errorf("cap:anchat:a1: %d rows, expires %d; want 1 row expiring at 300", n, expires)
	}
	if namespace != "anchat" {
		t.Errorf("the capability row's namespace is %q, want anchat", namespace)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM revoked_tokens WHERE jti = 'jti-session' AND namespace IS NULL`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("jti-session: %d rows with no namespace, want 1", n)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM revoked_tokens WHERE subject = '0xwallet'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("subject rows: %d, want both kept", n)
	}

	if _, err := db.Exec(`INSERT INTO revoked_tokens(jti, expires_at) VALUES ('jti-session', 60)`); err == nil {
		t.Error("a second row for one jti was accepted after 088")
	}
}

// A runner that dies after applying the file but before recording it applies
// it again on the next boot.
func TestMigration088_survivesBeingReapplied(t *testing.T) {
	db := openRoundtripDB(t)
	if err := rqlite.ApplyEmbeddedMigrations(t.Context(), db, migrations.FS, zap.NewNop()); err != nil {
		t.Fatalf("first apply: %v", err)
	}
	if _, err := db.Exec(`DELETE FROM schema_migrations WHERE version = 88`); err != nil {
		t.Fatal(err)
	}
	if err := rqlite.ApplyEmbeddedMigrations(t.Context(), db, migrations.FS, zap.NewNop()); err != nil {
		t.Fatalf("re-apply 088: %v", err)
	}
}
