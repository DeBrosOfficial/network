package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Orama-Network/orama/core/migrations"
	"github.com/Orama-Network/orama/core/pkg/client"
	"github.com/Orama-Network/orama/core/pkg/rqlite"
	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"
)

// These run the revocation writes against the real schema on real SQLite: what
// they are about is the UNIQUE index and the ceiling, which a fake that answers
// INSERT with success would pass whatever the statement said.

func realRevocations(t *testing.T) (*RevocationList, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	// One connection: each :memory: connection is its own database. The
	// concurrent revokes still each run their INSERT, one after another,
	// which is what a duplicate row came from.
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	if err := rqlite.ApplyEmbeddedMigrations(t.Context(), db, migrations.FS, zap.NewNop()); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	wrapped := &sqliteDatabase{db: db}
	return NewRevocationList(func() client.DatabaseClient { return wrapped }, nil), db
}

func countRevocationRows(t *testing.T, db *sql.DB, where string, args ...any) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM revoked_tokens WHERE `+where, args...).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	return n
}

// Two revokes of one token used to be a check and then an INSERT, so both
// passed the check and wrote a row each.
func TestRevocationList_concurrentRevokesOfOneTokenWriteOneRow(t *testing.T) {
	list, db := realRevocations(t)
	exp := time.Now().Add(time.Hour).Unix()

	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- list.RevokeToken(context.Background(), "jti-1", exp, "session ended")
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("a revoke of a token already revoked failed: %v", err)
		}
	}
	if n := countRevocationRows(t, db, `jti = 'jti-1'`); n != 1 {
		t.Errorf("%d rows for one token, want 1", n)
	}
	if !mustDenies(list, &JWTClaims{Jti: "jti-1"}, nil) {
		t.Error("the token is not refused")
	}
}

// One namespace looping mint and revoke stops at the ceiling; another
// namespace, and a capability already revoked, are not affected by it.
func TestRevocationList_capabilityRevocationsStopAtTheNamespaceCeiling(t *testing.T) {
	list, db := realRevocations(t)
	ctx := context.Background()
	exp := time.Now().Add(time.Hour).Unix()

	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	for n := range MaxLiveCapabilityRevocations {
		if _, err := tx.Exec(`INSERT INTO revoked_tokens(jti, namespace, expires_at) VALUES (?, 'greedy', ?)`,
			fmt.Sprintf("cap:greedy:%d", n), exp); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	err = list.RevokeCapability(ctx, "greedy", "cap:greedy:one-more", exp, "capability revoked")
	if !errors.Is(err, ErrCapabilityRevocationLimit) {
		t.Fatalf("revocation past the ceiling: %v, want ErrCapabilityRevocationLimit", err)
	}
	if n := countRevocationRows(t, db, `namespace = 'greedy'`); n != MaxLiveCapabilityRevocations {
		t.Errorf("%d rows for the namespace, want the ceiling %d", n, MaxLiveCapabilityRevocations)
	}

	if err := list.RevokeCapability(ctx, "greedy", "cap:greedy:0", exp, "capability revoked"); err != nil {
		t.Errorf("revoking a capability already revoked, at the ceiling: %v", err)
	}
	if err := list.RevokeCapability(ctx, "quiet", "cap:quiet:1", exp, "capability revoked"); err != nil {
		t.Fatalf("another namespace was refused by the greedy one's ceiling: %v", err)
	}
	if !mustDenies(list, &JWTClaims{Jti: "cap:quiet:1"}, nil) {
		t.Error("the other namespace's capability is not refused")
	}
}

// A revocation stops counting the moment the capability it names expires, so
// the ceiling is on what is in force, not on what was ever written.
func TestRevocationList_expiredCapabilityRevocationsDoNotCount(t *testing.T) {
	list, db := realRevocations(t)
	past := time.Now().Add(-time.Minute).Unix()

	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	for n := range MaxLiveCapabilityRevocations {
		if _, err := tx.Exec(`INSERT INTO revoked_tokens(jti, namespace, expires_at) VALUES (?, 'busy', ?)`,
			fmt.Sprintf("cap:busy:%d", n), past); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	if err := list.RevokeCapability(context.Background(), "busy", "cap:busy:new",
		time.Now().Add(time.Hour).Unix(), "capability revoked"); err != nil {
		t.Fatalf("expired revocations counted against the ceiling: %v", err)
	}
}
