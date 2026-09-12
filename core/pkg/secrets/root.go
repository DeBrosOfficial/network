package secrets

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	// FileName is the IKM file next to cluster-secret.
	FileName = "encryption-root"
	// FileIDName holds the current generation id.
	FileIDName = "encryption-root.id"
	// FilePrevName is the previous IKM while a rotate is in flight.
	FilePrevName = "encryption-root.prev"
	// FilePrevIDName is the previous generation id.
	FilePrevIDName = "encryption-root.prev.id"

	// FirstID is the generation assigned when the root is materialised from
	// the cluster secret. Existing ciphertext was derived from that IKM.
	FirstID = "1"

	rootTable = "encryption_roots"
)

// Store is the slice of the registry the root is persisted in.
type Store interface {
	Query(ctx context.Context, dest any, query string, args ...any) error
	Exec(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// LoadOrMaterialize returns the cluster's encryption root.
//
// Order: registry (source of truth) → files on disk → copy of clusterSecret.
// A missing file on an existing cluster is materialised by copying the cluster
// secret, so HKDF(encryption-root, purpose) equals today's keys. The file is
// never regenerated because it is unreadable or the wrong length: that is how
// IPFS-Cluster was silently partitioned, and it would orphan every stored
// secret here.
func LoadOrMaterialize(ctx context.Context, store Store, secretsDir, clusterSecret string) (Root, error) {
	clusterSecret = strings.TrimSpace(clusterSecret)

	if store != nil {
		if r, err := loadFromRegistry(ctx, store); err == nil && r.CurrentIKM != "" {
			_ = writeFiles(secretsDir, r)
			return r, nil
		}
	}

	r, ferr := loadFromFiles(secretsDir)
	if ferr == nil && r.CurrentIKM != "" {
		if store != nil {
			_ = saveToRegistry(ctx, store, r)
		}
		return r, nil
	}
	if ferr != nil && !os.IsNotExist(ferr) {
		return Root{}, ferr
	}

	if clusterSecret == "" {
		if ferr != nil {
			return Root{}, ferr
		}
		return Root{}, fmt.Errorf("no encryption root: no registry row, no %s file, and no cluster secret to copy", FileName)
	}

	r = Root{CurrentID: FirstID, CurrentIKM: clusterSecret}
	if err := writeFiles(secretsDir, r); err != nil {
		return Root{}, err
	}
	if store != nil {
		// The table may not exist yet: schema apply runs in the readiness
		// loop, after the secrets manager is constructed. The next persist
		// (operator rotate, or a later boot) writes the row.
		_ = saveToRegistry(ctx, store, r)
	}
	return r, nil
}

type rootRow struct {
	Slot           string `db:"slot"`
	KeyID          string `db:"key_id"`
	IKM            string `db:"ikm"`
	WriteVersioned int    `db:"write_versioned"`
}

func loadFromRegistry(ctx context.Context, store Store) (Root, error) {
	var rows []rootRow
	if err := store.Query(ctx, &rows, `SELECT slot, key_id, ikm, write_versioned FROM `+rootTable); err != nil {
		return Root{}, err
	}
	var r Root
	for _, row := range rows {
		switch row.Slot {
		case "current":
			r.CurrentID = row.KeyID
			r.CurrentIKM = strings.TrimSpace(row.IKM)
			r.WriteVersioned = row.WriteVersioned != 0
		case "previous":
			r.PreviousID = row.KeyID
			r.PreviousIKM = strings.TrimSpace(row.IKM)
		}
	}
	if r.CurrentIKM == "" {
		return Root{}, fmt.Errorf("registry encryption_roots has no current row")
	}
	return r, nil
}

func saveToRegistry(ctx context.Context, store Store, r Root) error {
	if _, err := store.Exec(ctx,
		`INSERT INTO `+rootTable+` (slot, key_id, ikm, write_versioned, updated_at)
		 VALUES ('current', ?, ?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(slot) DO UPDATE SET
			key_id = excluded.key_id,
			ikm = excluded.ikm,
			write_versioned = excluded.write_versioned,
			updated_at = excluded.updated_at`,
		r.CurrentID, r.CurrentIKM, boolToInt(r.WriteVersioned)); err != nil {
		return err
	}
	if r.PreviousIKM == "" {
		_, err := store.Exec(ctx, `DELETE FROM `+rootTable+` WHERE slot = 'previous'`)
		return err
	}
	_, err := store.Exec(ctx,
		`INSERT INTO `+rootTable+` (slot, key_id, ikm, write_versioned, updated_at)
		 VALUES ('previous', ?, ?, 0, CURRENT_TIMESTAMP)
		 ON CONFLICT(slot) DO UPDATE SET
			key_id = excluded.key_id,
			ikm = excluded.ikm,
			updated_at = excluded.updated_at`,
		r.PreviousID, r.PreviousIKM)
	return err
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func loadFromFiles(dir string) (Root, error) {
	ikm, err := readSecretFile(filepath.Join(dir, FileName))
	if err != nil {
		return Root{}, err
	}
	id := FirstID
	if s, err := readSecretFile(filepath.Join(dir, FileIDName)); err == nil && s != "" {
		id = s
	}
	r := Root{CurrentID: id, CurrentIKM: ikm}
	if prev, err := readSecretFile(filepath.Join(dir, FilePrevName)); err == nil && prev != "" {
		r.PreviousIKM = prev
		r.PreviousID = FirstID
		if s, err := readSecretFile(filepath.Join(dir, FilePrevIDName)); err == nil && s != "" {
			r.PreviousID = s
		}
	}
	return r, nil
}

func writeFiles(dir string, r Root) error {
	if dir == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("encryption-root dir: %w", err)
	}
	if err := os.Chmod(dir, 0700); err != nil {
		return fmt.Errorf("encryption-root dir perms: %w", err)
	}
	if err := writeSecretFile(filepath.Join(dir, FileName), r.CurrentIKM); err != nil {
		return err
	}
	if err := writeSecretFile(filepath.Join(dir, FileIDName), r.CurrentID); err != nil {
		return err
	}
	if r.PreviousIKM == "" {
		_ = os.Remove(filepath.Join(dir, FilePrevName))
		_ = os.Remove(filepath.Join(dir, FilePrevIDName))
		return nil
	}
	if err := writeSecretFile(filepath.Join(dir, FilePrevName), r.PreviousIKM); err != nil {
		return err
	}
	return writeSecretFile(filepath.Join(dir, FilePrevIDName), r.PreviousID)
}

func readSecretFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	v := strings.TrimSpace(string(data))
	if v == "" {
		return "", fmt.Errorf("%s is empty; refusing to invent a replacement", path)
	}
	return v, nil
}

func writeSecretFile(path, value string) error {
	if err := os.WriteFile(path, []byte(value), 0600); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := os.Chmod(path, 0600); err != nil {
		return fmt.Errorf("chmod %s: %w", path, err)
	}
	return nil
}

// Rotate generates a new IKM, demotes the current root to previous, and
// persists both. Existing ciphertext stays readable under previous until the
// walker re-encrypts it.
func Rotate(ctx context.Context, store Store, secretsDir string, current Root) (Root, error) {
	next, err := nextID(current.CurrentID)
	if err != nil {
		return Root{}, err
	}
	ikm, err := randomIKM()
	if err != nil {
		return Root{}, err
	}
	r := Root{
		CurrentID:      next,
		CurrentIKM:     ikm,
		PreviousID:     current.CurrentID,
		PreviousIKM:    current.CurrentIKM,
		WriteVersioned: true,
	}
	if err := writeFiles(secretsDir, r); err != nil {
		return Root{}, err
	}
	if store != nil {
		if err := saveToRegistry(ctx, store, r); err != nil {
			return Root{}, fmt.Errorf("persist rotated encryption root: %w", err)
		}
	}
	return r, nil
}

// ForgetPrevious drops the previous generation after the walker has finished.
func ForgetPrevious(ctx context.Context, store Store, secretsDir string, current Root) (Root, error) {
	current.PreviousID = ""
	current.PreviousIKM = ""
	if err := writeFiles(secretsDir, current); err != nil {
		return Root{}, err
	}
	if store != nil {
		if err := saveToRegistry(ctx, store, current); err != nil {
			return Root{}, err
		}
	}
	return current, nil
}

// EnableVersionedWrites flips new Encrypts to the versioned envelope without
// changing the IKM. Used by the format-only rewrite so a mixed-version
// rollout is finished before anything writes enc:v1:.
func EnableVersionedWrites(ctx context.Context, store Store, secretsDir string, current Root) (Root, error) {
	current.WriteVersioned = true
	if err := writeFiles(secretsDir, current); err != nil {
		return Root{}, err
	}
	if store != nil {
		if err := saveToRegistry(ctx, store, current); err != nil {
			return Root{}, err
		}
	}
	return current, nil
}

func nextID(current string) (string, error) {
	n, err := strconv.Atoi(strings.TrimSpace(current))
	if err != nil || n < 1 {
		return "", fmt.Errorf("encryption-root id %q is not a positive integer", current)
	}
	return strconv.Itoa(n + 1), nil
}

func randomIKM() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate encryption root: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// SecretsDir is <dataDir>/secrets, the same directory as cluster-secret.
func SecretsDir(dataDir string) string {
	return filepath.Join(dataDir, "secrets")
}

// Persist writes the root to the secrets directory. Used by a namespace
// gateway that received a rotated root from the index.
func Persist(secretsDir string, r Root) error {
	return writeFiles(secretsDir, r)
}
