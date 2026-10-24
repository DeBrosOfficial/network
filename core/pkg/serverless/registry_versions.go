package serverless

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/Orama-Network/orama/core/pkg/rqlite"
	"go.uber.org/zap"
)

// MaxRetainedFunctionVersions bounds how many versions of one function the
// registry keeps. A deploy past the bound removes the oldest version, so a
// function redeployed on every commit does not grow its table without limit.
// Ten is deep enough to pin a rollback target across a release train and small
// enough that the rows of a busy function stay in the tens.
const MaxRetainedFunctionVersions = 10

// triggerTables are the tables whose rows say "run THIS function when X
// happens". They follow the current version: a deploy moves them to the new
// row, so a trigger fires the code that was just shipped and a version row is
// never the owner of a trigger the next deploy would orphan.
var triggerTables = []string{
	"function_cron_triggers",
	"function_pubsub_triggers",
	"function_db_triggers",
	"function_timers",
}

// historyTables hold what happened while a version was current. They stay with
// the version that produced them until that version is pruned, when they move to
// the oldest version kept: logs, invocations and jobs are read by function name,
// so the history of a function is not cut short by retention, and no row is left
// pointing at a deleted version (rqlite runs without foreign keys, so ON DELETE
// CASCADE never fires).
var historyTables = []string{
	"function_invocations",
	"function_logs",
	"function_jobs",
}

// versionIDRow scans one function row id.
type versionIDRow struct {
	ID string `db:"id"`
}

// versionStatusRow scans one function row's id and status.
type versionStatusRow struct {
	ID     string `db:"id"`
	Status string `db:"status"`
}

// adoptVersionState finishes a deploy of a new version: it makes newID the
// current version, then removes the versions beyond
// MaxRetainedFunctionVersions. One transaction, so a function is never left with
// its triggers on a row about to be removed.
func (r *Registry) adoptVersionState(ctx context.Context, namespace, name, newID string) error {
	err := r.db.Tx(ctx, func(tx rqlite.Tx) error {
		if err := makeCurrent(ctx, tx, namespace, name, newID); err != nil {
			return err
		}
		return pruneOldVersions(ctx, tx, namespace, name)
	})
	if err != nil {
		return fmt.Errorf("failed to finish deploy of %s/%s: %w", namespace, name, err)
	}
	r.logger.Debug("Function version state adopted",
		zap.String("namespace", namespace), zap.String("name", name), zap.String("id", newID))
	return nil
}

// makeCurrent makes the row id the one current version of the function: every
// other version that could run (active, or disabled) becomes superseded, so
// name@N of a replaced version is refused rather than served, and the triggers
// the other versions owned move onto id, so a trigger fires the code that is
// current.
func makeCurrent(ctx context.Context, tx rqlite.Tx, namespace, name, id string) error {
	if _, err := tx.Exec(ctx,
		`UPDATE functions SET status = ?, updated_at = ? WHERE namespace = ? AND name = ? AND id != ? AND status IN (?, ?)`,
		string(FunctionStatusSuperseded), time.Now(), namespace, name, id,
		string(FunctionStatusActive), string(FunctionStatusInactive)); err != nil {
		return fmt.Errorf("failed to supersede the earlier versions: %w", err)
	}
	for _, table := range triggerTables {
		q := `UPDATE ` + table + ` SET function_id = ? WHERE function_id IN (
			SELECT id FROM functions WHERE namespace = ? AND name = ? AND id != ?)`
		if _, err := tx.Exec(ctx, q, id, namespace, name, id); err != nil {
			return fmt.Errorf("failed to move %s to the current version: %w", table, err)
		}
	}
	return nil
}

// Rollback makes version the current version of the function: it becomes
// active, the version it replaces is superseded, and the triggers move to it.
// Only the current version is invocable, so this — not name@N — is how an
// earlier version is run again. A version that is pruned or deleted cannot be
// rolled back to (ErrVersionNotFound); rolling back to the current version
// enables it if it was disabled.
func (r *Registry) Rollback(ctx context.Context, namespace, name string, version int) error {
	namespace = strings.TrimSpace(namespace)
	name = strings.TrimSpace(name)
	if namespace == "" || name == "" || version <= 0 {
		return &ValidationError{Field: "version", Message: "a namespace, a name and a version number are required"}
	}
	err := r.db.Tx(ctx, func(tx rqlite.Tx) error {
		var target []versionStatusRow
		if err := tx.Query(ctx, &target,
			`SELECT id, status FROM functions WHERE namespace = ? AND name = ? AND version = ?`,
			namespace, name, version); err != nil {
			return fmt.Errorf("failed to read the version to roll back to: %w", err)
		}
		if len(target) == 0 || FunctionStatus(target[0].Status) == FunctionStatusDeleted {
			return ErrVersionNotFound
		}
		if err := makeCurrent(ctx, tx, namespace, name, target[0].ID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE functions SET status = ?, updated_at = ? WHERE id = ?`,
			string(FunctionStatusActive), time.Now(), target[0].ID); err != nil {
			return fmt.Errorf("failed to make the version current: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	r.invalidateFn(namespace, name)
	r.logger.Info("Function rolled back",
		zap.String("namespace", namespace), zap.String("name", name), zap.Int("version", version))
	return nil
}

const (
	// dbChangeTrackingTable holds the cursor of each database trigger; its
	// rows are keyed by trigger, not by function.
	dbChangeTrackingTable = "function_db_change_tracking"
	// envVarsTable holds each version's environment variables.
	envVarsTable = "function_env_vars"
)

// Purge removes a function from the namespace for good: its version rows and
// everything keyed to them — environment variables, triggers (and the cursors
// of its database triggers), invocations, logs and jobs. If version is 0 every
// version goes; otherwise only that one, and purging the current version
// leaves the function with none until a rollback. One transaction, so a purge
// is never half done. Secrets are the namespace's, not the function's, and
// stay. The WASM pins stay too: function WASM is not in the cluster reference
// index, so nothing can tell whether another namespace deployed the same bytes
// (bugboard #947).
//
// Every statement selects by (namespace, name[, version]) through the indexed
// function_id columns, so its cost is the rows the function holds, not the
// size of the namespace.
func (r *Registry) Purge(ctx context.Context, namespace, name string, version int) error {
	namespace = strings.TrimSpace(namespace)
	name = strings.TrimSpace(name)
	if namespace == "" || name == "" {
		return &ValidationError{Field: "name", Message: "a namespace and a name are required"}
	}
	versions := `SELECT id FROM functions WHERE namespace = ? AND name = ?`
	args := []any{namespace, name}
	if version != 0 {
		versions += ` AND version = ?`
		args = append(args, version)
	}
	var ids []versionIDRow
	err := r.db.Tx(ctx, func(tx rqlite.Tx) error {
		if err := tx.Query(ctx, &ids, versions, args...); err != nil {
			return fmt.Errorf("failed to read the versions to purge: %w", err)
		}
		if len(ids) == 0 {
			if version == 0 {
				return ErrFunctionNotFound
			}
			return ErrVersionNotFound
		}
		if _, err := tx.Exec(ctx, `DELETE FROM `+dbChangeTrackingTable+` WHERE trigger_id IN (
			SELECT id FROM function_db_triggers WHERE function_id IN (`+versions+`))`, args...); err != nil {
			return fmt.Errorf("failed to purge %s: %w", dbChangeTrackingTable, err)
		}
		for _, table := range slices.Concat(triggerTables, historyTables, []string{envVarsTable}) {
			if _, err := tx.Exec(ctx, `DELETE FROM `+table+` WHERE function_id IN (`+versions+`)`, args...); err != nil {
				return fmt.Errorf("failed to purge %s: %w", table, err)
			}
		}
		del := `DELETE FROM functions WHERE namespace = ? AND name = ?`
		if version != 0 {
			del += ` AND version = ?`
		}
		if _, err := tx.Exec(ctx, del, args...); err != nil {
			return fmt.Errorf("failed to purge the function rows: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	r.invalidateFn(namespace, name)
	for _, id := range ids {
		r.invalidateEnv(id.ID)
	}
	r.logger.Info("Function purged",
		zap.String("namespace", namespace), zap.String("name", name), zap.Int("version", version))
	return nil
}

// pruneOldVersions deletes every version of the function older than the newest
// MaxRetainedFunctionVersions, moving their invocation history to the oldest
// version kept and dropping their env vars.
func pruneOldVersions(ctx context.Context, tx rqlite.Tx, namespace, name string) error {
	var kept []versionIDRow
	if err := tx.Query(ctx, &kept, `
		SELECT id FROM functions WHERE namespace = ? AND name = ?
		ORDER BY version DESC LIMIT ?`, namespace, name, MaxRetainedFunctionVersions); err != nil {
		return fmt.Errorf("failed to list retained versions: %w", err)
	}
	if len(kept) < MaxRetainedFunctionVersions {
		return nil
	}
	oldestKept := kept[len(kept)-1].ID

	var pruned []versionIDRow
	if err := tx.Query(ctx, &pruned, `
		SELECT id FROM functions WHERE namespace = ? AND name = ?
		ORDER BY version DESC LIMIT -1 OFFSET ?`, namespace, name, MaxRetainedFunctionVersions); err != nil {
		return fmt.Errorf("failed to list versions to prune: %w", err)
	}
	for _, old := range pruned {
		for _, table := range historyTables {
			if _, err := tx.Exec(ctx, `UPDATE `+table+` SET function_id = ? WHERE function_id = ?`,
				oldestKept, old.ID); err != nil {
				return fmt.Errorf("failed to move %s of pruned version: %w", table, err)
			}
		}
		if _, err := tx.Exec(ctx, `DELETE FROM function_env_vars WHERE function_id = ?`, old.ID); err != nil {
			return fmt.Errorf("failed to delete env vars of pruned version: %w", err)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM functions WHERE id = ?`, old.ID); err != nil {
			return fmt.Errorf("failed to delete pruned version: %w", err)
		}
	}
	return nil
}
