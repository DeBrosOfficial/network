package serverless

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
)

// versionID is the row id of version of fn in namespace "ns", whatever its status.
func versionID(t *testing.T, r *Registry, fn string, version int) string {
	t.Helper()
	versions, err := r.ListVersions(context.Background(), "ns", fn)
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	for _, v := range versions {
		if v.Version == version {
			return v.ID
		}
	}
	t.Fatalf("%s has no version %d", fn, version)
	return ""
}

// bug-939/bug-940: a deploy supersedes the version before it, and name@N of
// a superseded version is refused with its own error, not served.
func TestGet_supersededVersionIsRefused(t *testing.T) {
	r, _ := newVersionedRegistry(t)
	ctx := context.Background()
	deployN(t, r, "fn", 3)
	for v := 1; v <= 2; v++ {
		if _, err := r.Get(ctx, "ns", "fn", v); !errors.Is(err, ErrVersionSuperseded) {
			t.Errorf("Get fn@%d after v3 shipped = %v; want ErrVersionSuperseded", v, err)
		}
	}
	if got, err := r.Get(ctx, "ns", "fn", 3); err != nil || got.Version != 3 {
		t.Errorf("Get fn@3 (current) = %v, %v; want version 3", got, err)
	}
	if !IsNotFound(ErrVersionSuperseded) {
		t.Error("a superseded version must read as not found to every caller that maps not-found to 404")
	}
}

// A gateway older than the superseded status deploys without marking the
// version it replaced, so two rows can be active during a rolling upgrade.
// The lower one is still refused.
func TestGet_activeRowBelowAnotherActiveRowIsSuperseded(t *testing.T) {
	r, db := newVersionedRegistry(t)
	ctx := context.Background()
	deployN(t, r, "fn", 2)
	if _, err := db.Exec(`UPDATE functions SET status = 'active' WHERE namespace = 'ns' AND name = 'fn'`); err != nil {
		t.Fatalf("seed two active rows: %v", err)
	}
	if _, err := r.Get(ctx, "ns", "fn", 1); !errors.Is(err, ErrVersionSuperseded) {
		t.Errorf("Get fn@1 beside an active v2 = %v; want ErrVersionSuperseded", err)
	}
}

func TestGet_disabledOrDeletedVersionIsNotFound(t *testing.T) {
	r, _ := newVersionedRegistry(t)
	ctx := context.Background()
	deployN(t, r, "fn", 2)
	if err := r.SetEnabled(ctx, "ns", "fn", false); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if _, err := r.Get(ctx, "ns", "fn", 2); !errors.Is(err, ErrVersionNotFound) {
		t.Errorf("Get fn@2 of a disabled function = %v; want ErrVersionNotFound", err)
	}
	if err := r.Delete(ctx, "ns", "fn", 0); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := r.Get(ctx, "ns", "fn", 1); !errors.Is(err, ErrVersionNotFound) {
		t.Errorf("Get fn@1 after delete = %v; want ErrVersionNotFound", err)
	}
}

// Disabling and enabling act on the current version only: enabling never
// brings a superseded version back.
func TestSetEnabled_doesNotReviveASupersededVersion(t *testing.T) {
	r, _ := newVersionedRegistry(t)
	ctx := context.Background()
	deployN(t, r, "fn", 2)
	if err := r.SetEnabled(ctx, "ns", "fn", false); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if err := r.SetEnabled(ctx, "ns", "fn", true); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if err := r.SetEnabled(ctx, "ns", "fn", true); err != nil {
		t.Fatalf("enabling an enabled function must succeed: %v", err)
	}
	if _, err := r.Get(ctx, "ns", "fn", 1); !errors.Is(err, ErrVersionSuperseded) {
		t.Errorf("Get fn@1 after disable and enable = %v; want ErrVersionSuperseded", err)
	}
	if got, err := r.Get(ctx, "ns", "fn", 0); err != nil || got.Version != 2 {
		t.Errorf("current after enable = %v, %v; want version 2", got, err)
	}
}

// A rollback makes an earlier version the current one: it runs at the bare
// name and at its number, the version it replaced is superseded, the triggers
// move to it, and the next deploy is current again with the next number.
func TestRollback_makesTheVersionCurrent(t *testing.T) {
	r, db := newVersionedRegistry(t)
	ctx := context.Background()
	deployN(t, r, "fn", 3)
	if _, err := db.Exec(`INSERT INTO function_pubsub_triggers (id, function_id, topic, topic_pattern, enabled) VALUES ('t1', ?, 'a', 'a', 1)`,
		versionID(t, r, "fn", 3)); err != nil {
		t.Fatalf("seed trigger: %v", err)
	}

	if err := r.Rollback(ctx, "ns", "fn", 1); err != nil {
		t.Fatalf("Rollback to 1: %v", err)
	}
	if got, err := r.Get(ctx, "ns", "fn", 0); err != nil || got.Version != 1 {
		t.Fatalf("current after rollback = %v, %v; want version 1", got, err)
	}
	if _, err := r.Get(ctx, "ns", "fn", 1); err != nil {
		t.Errorf("Get fn@1 after rolling back to it: %v", err)
	}
	if _, err := r.Get(ctx, "ns", "fn", 3); !errors.Is(err, ErrVersionSuperseded) {
		t.Errorf("Get fn@3 after rolling back from it = %v; want ErrVersionSuperseded", err)
	}
	var owner string
	if err := db.QueryRow(`SELECT function_id FROM function_pubsub_triggers WHERE id = 't1'`).Scan(&owner); err != nil {
		t.Fatalf("read trigger: %v", err)
	}
	if v1 := versionID(t, r, "fn", 1); owner != v1 {
		t.Errorf("trigger owned by %s after rollback; want version 1 %s", owner, v1)
	}

	deployN(t, r, "fn", 1)
	if got, err := r.Get(ctx, "ns", "fn", 0); err != nil || got.Version != 4 {
		t.Fatalf("deploy after rollback = %v, %v; want version 4 current", got, err)
	}
	if _, err := r.Get(ctx, "ns", "fn", 1); !errors.Is(err, ErrVersionSuperseded) {
		t.Errorf("Get fn@1 after the next deploy = %v; want ErrVersionSuperseded", err)
	}
}

func TestRollback_ofADisabledFunctionServesTheTarget(t *testing.T) {
	r, _ := newVersionedRegistry(t)
	ctx := context.Background()
	deployN(t, r, "fn", 2)
	if err := r.SetEnabled(ctx, "ns", "fn", false); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if err := r.Rollback(ctx, "ns", "fn", 1); err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	if got, err := r.Get(ctx, "ns", "fn", 0); err != nil || got.Version != 1 {
		t.Fatalf("current after rolling back a disabled function = %v, %v; want version 1", got, err)
	}
	if _, err := r.Get(ctx, "ns", "fn", 2); !errors.Is(err, ErrVersionSuperseded) {
		t.Errorf("the disabled version 2 after rollback = %v; want ErrVersionSuperseded", err)
	}
}

func TestRollback_toAVersionThatCannotRunIsVersionNotFound(t *testing.T) {
	r, _ := newVersionedRegistry(t)
	ctx := context.Background()
	deployN(t, r, "fn", 3)
	if err := r.Delete(ctx, "ns", "fn", 1); err != nil {
		t.Fatalf("Delete v1: %v", err)
	}
	for _, v := range []int{1, 9} {
		if err := r.Rollback(ctx, "ns", "fn", v); !errors.Is(err, ErrVersionNotFound) {
			t.Errorf("Rollback to %d = %v; want ErrVersionNotFound", v, err)
		}
	}
	if got, err := r.Get(ctx, "ns", "fn", 0); err != nil || got.Version != 3 {
		t.Errorf("a refused rollback changed the current version: %v, %v", got, err)
	}
	var verr *ValidationError
	if err := r.Rollback(ctx, "ns", "fn", 0); !errors.As(err, &verr) {
		t.Errorf("Rollback to version 0 = %v; want a ValidationError", err)
	}
}

// countRows runs a COUNT query.
func countRows(t *testing.T, db *sql.DB, query string, args ...any) int {
	t.Helper()
	var n int
	if err := db.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return n
}

// seedVersionState writes one row of everything keyed to a version id.
func seedVersionState(t *testing.T, db *sql.DB, id, tag string) {
	t.Helper()
	for _, q := range []string{
		`INSERT INTO function_env_vars (id, function_id, key, value) VALUES ('e-` + tag + `', ?, 'K', 'V')`,
		`INSERT INTO function_pubsub_triggers (id, function_id, topic, topic_pattern, enabled) VALUES ('p-` + tag + `', ?, 'a', 'a', 1)`,
		`INSERT INTO function_cron_triggers (id, function_id, cron_expression) VALUES ('c-` + tag + `', ?, '* * * * *')`,
		`INSERT INTO function_db_triggers (id, function_id, table_name, operation) VALUES ('d-` + tag + `', ?, 't', 'INSERT')`,
		`INSERT INTO function_timers (id, function_id, run_at) VALUES ('m-` + tag + `', ?, CURRENT_TIMESTAMP)`,
		`INSERT INTO function_invocations (id, function_id, request_id, trigger_type, status, started_at) VALUES ('i-` + tag + `', ?, 'r', 'http', 'success', CURRENT_TIMESTAMP)`,
		`INSERT INTO function_logs (id, function_id, invocation_id, level, message) VALUES ('l-` + tag + `', ?, 'i-` + tag + `', 'info', 'hello')`,
		`INSERT INTO function_jobs (id, function_id, status) VALUES ('j-` + tag + `', ?, 'pending')`,
	} {
		if _, err := db.Exec(q, id); err != nil {
			t.Fatalf("seed %q: %v", q, err)
		}
	}
	if _, err := db.Exec(`INSERT INTO function_db_change_tracking (id, trigger_id) VALUES ('ct-` + tag + `', 'd-` + tag + `')`); err != nil {
		t.Fatalf("seed change tracking: %v", err)
	}
}

// keyedTables is every table Purge clears by function id.
var keyedTables = []string{
	"function_env_vars", "function_pubsub_triggers", "function_cron_triggers", "function_db_triggers",
	"function_timers", "function_invocations", "function_logs", "function_jobs",
}

// bug-940: a purge removes the rows of every version, whatever their status,
// and everything keyed to them, so nothing of the function is left to list or
// invoke; the next deploy starts again at version 1.
func TestPurge_removesEveryVersionAndWhatIsKeyedToIt(t *testing.T) {
	r, db := newVersionedRegistry(t)
	ctx := context.Background()
	deployN(t, r, "fn", 3)
	deployN(t, r, "keep", 1)
	if err := r.Delete(ctx, "ns", "fn", 1); err != nil {
		t.Fatalf("Delete v1: %v", err)
	}
	for v := 1; v <= 3; v++ {
		seedVersionState(t, db, versionID(t, r, "fn", v), fmt.Sprintf("fn%d", v))
	}
	seedVersionState(t, db, versionID(t, r, "keep", 1), "keep")

	if err := r.Purge(ctx, "ns", "fn", 0); err != nil {
		t.Fatalf("Purge: %v", err)
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM functions WHERE namespace = 'ns' AND name = 'fn'`); n != 0 {
		t.Errorf("%d function rows left after purge", n)
	}
	for _, table := range keyedTables {
		if n := countRows(t, db, `SELECT COUNT(*) FROM `+table+` WHERE function_id NOT IN (SELECT id FROM functions)`); n != 0 {
			t.Errorf("%d rows of %s point at a purged version", n, table)
		}
		if n := countRows(t, db, `SELECT COUNT(*) FROM `+table+` WHERE function_id = ?`, versionID(t, r, "keep", 1)); n != 1 {
			t.Errorf("purging fn touched %s of another function: %d rows left, want 1", table, n)
		}
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM function_db_change_tracking`); n != 1 {
		t.Errorf("%d database-trigger cursors left; want only the other function's", n)
	}
	all, err := r.ListAll(ctx, "ns")
	if err != nil || len(all) != 1 || all[0].Name != "keep" {
		t.Errorf("ListAll after purge = %d rows, %v; want only keep", len(all), err)
	}
	if _, err := r.Get(ctx, "ns", "fn", 0); !errors.Is(err, ErrFunctionNotFound) {
		t.Errorf("Get after purge = %v; want ErrFunctionNotFound", err)
	}
	deployN(t, r, "fn", 1)
	if got, err := r.Get(ctx, "ns", "fn", 0); err != nil || got.Version != 1 {
		t.Errorf("deploy after purge = %v, %v; want version 1", got, err)
	}
}

func TestPurge_oneVersionLeavesTheOthers(t *testing.T) {
	r, db := newVersionedRegistry(t)
	ctx := context.Background()
	deployN(t, r, "fn", 2)
	seedVersionState(t, db, versionID(t, r, "fn", 1), "v1")
	if err := r.Purge(ctx, "ns", "fn", 1); err != nil {
		t.Fatalf("Purge v1: %v", err)
	}
	versions, err := r.ListVersions(ctx, "ns", "fn")
	if err != nil || len(versions) != 1 || versions[0].Version != 2 {
		t.Fatalf("versions after purging v1 = %d rows, %v; want [2]", len(versions), err)
	}
	for _, table := range keyedTables {
		if n := countRows(t, db, `SELECT COUNT(*) FROM `+table+` WHERE function_id NOT IN (SELECT id FROM functions)`); n != 0 {
			t.Errorf("%d rows of %s point at the purged version", n, table)
		}
	}
	if got, err := r.Get(ctx, "ns", "fn", 0); err != nil || got.Version != 2 {
		t.Errorf("current after purging v1 = %v, %v; want version 2", got, err)
	}
}

func TestPurge_unknownFunctionOrVersionIsNotFound(t *testing.T) {
	r, _ := newVersionedRegistry(t)
	ctx := context.Background()
	if err := r.Purge(ctx, "ns", "nope", 0); !errors.Is(err, ErrFunctionNotFound) {
		t.Errorf("Purge of an unknown function = %v; want ErrFunctionNotFound", err)
	}
	deployN(t, r, "fn", 1)
	if err := r.Purge(ctx, "ns", "fn", 7); !errors.Is(err, ErrVersionNotFound) {
		t.Errorf("Purge of an unknown version = %v; want ErrVersionNotFound", err)
	}
}

// ListAll is the owner's view of every row it holds, whatever its status.
func TestListAll_showsEveryStatus(t *testing.T) {
	r, _ := newVersionedRegistry(t)
	ctx := context.Background()
	deployN(t, r, "a", 2)
	deployN(t, r, "b", 1)
	if err := r.SetEnabled(ctx, "ns", "b", false); err != nil {
		t.Fatalf("disable b: %v", err)
	}
	deployN(t, r, "c", 1)
	if err := r.Delete(ctx, "ns", "c", 0); err != nil {
		t.Fatalf("delete c: %v", err)
	}
	all, err := r.ListAll(ctx, "ns")
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	got := make([]string, 0, len(all))
	for _, fn := range all {
		got = append(got, fmt.Sprintf("%s@%d:%s", fn.Name, fn.Version, fn.Status))
	}
	want := []string{"a@2:active", "a@1:superseded", "b@1:inactive", "c@1:deleted"}
	if len(got) != len(want) {
		t.Fatalf("ListAll = %v; want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("ListAll[%d] = %s; want %s", i, got[i], want[i])
		}
	}
	if listed, err := r.List(ctx, "ns"); err != nil || len(listed) != 1 || listed[0].Name != "a" {
		t.Errorf("List must still show only current, enabled functions: %d rows, %v", len(listed), err)
	}
}
