package secrets

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/DeBrosOfficial/network/pkg/rqlite"
	_ "github.com/mattn/go-sqlite3"
)

func TestLoadOrMaterialize_copiesClusterSecretAndDoesNotRegenerate(t *testing.T) {
	dir := t.TempDir()
	secretsDir := filepath.Join(dir, "secrets")
	const cs = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	r, err := LoadOrMaterialize(context.Background(), nil, secretsDir, cs)
	if err != nil {
		t.Fatal(err)
	}
	if r.CurrentIKM != cs || r.CurrentID != FirstID {
		t.Fatalf("got %+v", r)
	}

	// A second load reads the file, not a new value.
	r2, err := LoadOrMaterialize(context.Background(), nil, secretsDir, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	if err != nil {
		t.Fatal(err)
	}
	if r2.CurrentIKM != cs {
		t.Fatalf("regenerated or ignored the file: %q", r2.CurrentIKM)
	}
}

func TestLoadOrMaterialize_emptyFileIsFatal(t *testing.T) {
	dir := t.TempDir()
	secretsDir := filepath.Join(dir, "secrets")
	if err := os.MkdirAll(secretsDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(secretsDir, FileName), []byte("\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadOrMaterialize(context.Background(), nil, secretsDir, "cluster"); err == nil {
		t.Fatal("empty file was replaced")
	}
}

func TestRotate_newIKMCannotDeriveFromOld(t *testing.T) {
	dir := t.TempDir()
	secretsDir := filepath.Join(dir, "secrets")
	const cs = "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	cur, err := LoadOrMaterialize(context.Background(), nil, secretsDir, cs)
	if err != nil {
		t.Fatal(err)
	}
	next, err := Rotate(context.Background(), nil, secretsDir, cur)
	if err != nil {
		t.Fatal(err)
	}
	if next.CurrentIKM == cur.CurrentIKM {
		t.Fatal("rotate reused the IKM")
	}
	if next.PreviousIKM != cur.CurrentIKM {
		t.Fatal("previous was not the old current")
	}
	if next.CurrentID != "2" || next.PreviousID != "1" {
		t.Fatalf("ids %+v", next)
	}

	oldKS, _ := cur.Keyset("p")
	newKS, _ := next.Keyset("p")
	ct, err := newKS.Encrypt("after")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := oldKS.Decrypt(ct); err == nil {
		t.Fatal("pre-rotate key opened post-rotate ciphertext")
	}
}

func TestLoadOrMaterialize_registryWins(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec(`CREATE TABLE encryption_roots (
		slot TEXT PRIMARY KEY, key_id TEXT NOT NULL, ikm TEXT NOT NULL,
		write_versioned INTEGER NOT NULL DEFAULT 0, updated_at TIMESTAMP
	)`); err != nil {
		t.Fatal(err)
	}
	store := rqlite.NewClient(db)
	if _, err := store.Exec(context.Background(),
		`INSERT INTO encryption_roots (slot, key_id, ikm, write_versioned) VALUES ('current', '7', 'dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd', 1)`); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	r, err := LoadOrMaterialize(context.Background(), store, filepath.Join(dir, "secrets"), "cluster-secret-should-not-win")
	if err != nil {
		t.Fatal(err)
	}
	if r.CurrentID != "7" || r.CurrentIKM[0] != 'd' || !r.WriteVersioned {
		t.Fatalf("got %+v", r)
	}
}
