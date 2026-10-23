//go:build e2e_fleet

package serverless

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestHostDB_tenantTriggers: a namespace schema may keep its invariants with
// triggers on its own tables (bugboard chg-943). One created through
// db_execute_v2 and one in a db_transaction batch both land and fire; a
// trigger on a platform table, or whose body names one (as an identifier or a
// string literal), runs a PRAGMA, creates a trigger, reads the clock, or is
// TEMP, is refused (website/src/docs/developer/functions.mdx#database-rqlite).
func TestHostDB_tenantTriggers(t *testing.T) {
	t.Parallel()
	fx := setup(t)
	const fn = "e2e-dbtrigger"
	deploy(t, fx, fnSpec{name: fn})
	for _, ddl := range []string{
		"CREATE TABLE IF NOT EXISTS e2e_trg_msg (id INTEGER PRIMARY KEY, body TEXT)",
		"CREATE TABLE IF NOT EXISTS e2e_trg_owed (message_id INTEGER)",
		"CREATE TABLE IF NOT EXISTS e2e_trg_log (message_id INTEGER)",
		"DROP TRIGGER IF EXISTS e2e_trg_blanked",
		"DROP TRIGGER IF EXISTS e2e_trg_logged",
		"DELETE FROM e2e_trg_msg", "DELETE FROM e2e_trg_owed", "DELETE FROM e2e_trg_log",
	} {
		if res := call(t, fx, fn, map[string]any{"op": "db_exec", "sql": ddl}); hostCallRefused(res) {
			t.Fatalf("setup %q: %v", ddl, res)
		}
	}

	blanked := `CREATE TRIGGER e2e_trg_blanked AFTER UPDATE OF body ON e2e_trg_msg
	  FOR EACH ROW WHEN new.body IS NULL
	  BEGIN DELETE FROM e2e_trg_owed WHERE message_id = new.id; END`
	if res := call(t, fx, fn, map[string]any{"op": "db_exec", "sql": blanked}); hostCallRefused(res) {
		t.Fatalf("db_execute_v2 refused a tenant trigger: %v", res)
	}
	logged := map[string]any{"ops": []map[string]any{{"kind": "exec",
		"sql": "CREATE TRIGGER e2e_trg_logged AFTER INSERT ON e2e_trg_msg BEGIN INSERT INTO e2e_trg_log(message_id) VALUES (new.id); END"}}}
	if res := call(t, fx, fn, map[string]any{"op": "db_tx", "ops": logged}); res["committed"] != true {
		t.Fatalf("db_transaction refused a tenant trigger: %v", trim(res))
	}

	for _, sql := range []string{
		"INSERT INTO e2e_trg_msg (id, body) VALUES (1, 'hi')",
		"INSERT INTO e2e_trg_owed (message_id) VALUES (1)",
		"UPDATE e2e_trg_msg SET body = NULL WHERE id = 1",
	} {
		if res := call(t, fx, fn, map[string]any{"op": "db_exec", "sql": sql}); hostCallRefused(res) {
			t.Fatalf("%q: %v", sql, res)
		}
	}
	rows := call(t, fx, fn, map[string]any{"op": "db_query",
		"sql": "SELECT (SELECT COUNT(*) FROM e2e_trg_owed) AS owed, (SELECT COUNT(*) FROM e2e_trg_log) AS logged"})
	if got := fmt.Sprint(rows["rows"]); !strings.Contains(got, "owed:0") || !strings.Contains(got, "logged:1") {
		t.Errorf("the triggers did not fire: %v", rows)
	}

	const head = "CREATE TRIGGER e2e_trg_bad AFTER INSERT ON e2e_trg_msg BEGIN "
	for _, sql := range []string{
		"CREATE TRIGGER e2e_trg_bad AFTER INSERT ON api_keys BEGIN DELETE FROM e2e_trg_log; END",
		head + "UPDATE grants SET role = 'owner'; END",
		head + "INSERT INTO e2e_trg_log SELECT * FROM 'function_secrets'; END",
		head + "PRAGMA writable_schema = 1; END",
		head + "CREATE TRIGGER e2e_trg_inner AFTER INSERT ON e2e_trg_log BEGIN SELECT 1; END; END",
		head + "INSERT INTO e2e_trg_log(message_id) VALUES (unixepoch()); END",
		"CREATE TEMP TRIGGER e2e_trg_bad AFTER INSERT ON e2e_trg_msg BEGIN DELETE FROM e2e_trg_log; END",
		head + "DELETE FROM e2e_trg_log; END; DELETE FROM e2e_trg_msg",
	} {
		if res := call(t, fx, fn, map[string]any{"op": "db_exec", "sql": sql}); !hostCallRefused(res) {
			t.Errorf("%q was not refused: %v", sql, res)
		}
		ops := map[string]any{"ops": []map[string]any{{"kind": "exec", "sql": sql}}}
		if res := call(t, fx, fn, map[string]any{"op": "db_tx", "ops": ops}); res != nil && res["committed"] == true {
			t.Errorf("db_transaction committed %q", sql)
		}
	}
}

// TestHostDB_linearizableRead: db_query_batch serves consistency
// "linearizable" and the read sees a write committed before it; an unknown
// level ("strong") and a freshness bound with linearizable are refused, never
// served at another level (website/src/docs/developer/functions.mdx#database-transactions).
func TestHostDB_linearizableRead(t *testing.T) {
	t.Parallel()
	fx := setup(t)
	const fn = "e2e-dblinear"
	deploy(t, fx, fnSpec{name: fn})
	call(t, fx, fn, map[string]any{"op": "db_exec", "sql": "CREATE TABLE IF NOT EXISTS e2e_lin (v TEXT)"})
	marker := fmt.Sprintf("lin-%s", t.Name())
	if res := call(t, fx, fn, map[string]any{"op": "db_exec", "sql": "INSERT INTO e2e_lin (v) VALUES (?)", "args": []any{marker}}); hostCallRefused(res) {
		t.Fatalf("insert: %v", res)
	}
	read := func(extra map[string]any) map[string]any {
		ops := map[string]any{"ops": []map[string]any{{"sql": "SELECT v FROM e2e_lin WHERE v = ?", "args": []any{marker}}}}
		for k, v := range extra {
			ops[k] = v
		}
		return call(t, fx, fn, map[string]any{"op": "db_batch", "ops": ops})
	}
	if res := read(map[string]any{"consistency": "linearizable"}); !strings.Contains(fmt.Sprint(res["results"]), marker) {
		t.Errorf("a linearizable read missed the committed write: %v", res)
	}
	for name, extra := range map[string]map[string]any{
		"an unknown level":            {"consistency": "strong"},
		"freshness with linearizable": {"consistency": "linearizable", "freshness": "2s"},
	} {
		// A refused batch read is the rejection envelope: an error and no rows.
		if res := read(extra); res == nil || res["error"] == nil || strings.Contains(fmt.Sprint(res["results"]), marker) {
			t.Errorf("%s was not refused: %v", name, res)
		}
	}
}

// TestHostDB_transactionReadsItsOwnWrites: a db_transaction query runs inside
// the transaction, after the ops before it (feat-950). Commits that each
// allocate a sequence number and read it back get one value each, the one they
// wrote, however many land at once; a failing query rolls the batch back
// (website/src/docs/developer/functions.mdx#database-transactions).
func TestHostDB_transactionReadsItsOwnWrites(t *testing.T) {
	t.Parallel()
	fx := setup(t)
	const fn = "e2e-dbtxread"
	deploy(t, fx, fnSpec{name: fn})
	topic := fmt.Sprintf("seq-%d", time.Now().UnixNano())
	for _, sql := range []string{
		"CREATE TABLE IF NOT EXISTS e2e_txseq (topic TEXT PRIMARY KEY, next_seq INTEGER NOT NULL)",
		"INSERT INTO e2e_txseq (topic, next_seq) VALUES ('" + topic + "', 0)",
	} {
		if res := call(t, fx, fn, map[string]any{"op": "db_exec", "sql": sql}); hostCallRefused(res) {
			t.Fatalf("setup %q: %v", sql, res)
		}
	}
	allocate := map[string]any{"ops": []map[string]any{
		{"kind": "exec", "sql": "UPDATE e2e_txseq SET next_seq = next_seq + 1 WHERE topic = ?", "args": []any{topic}},
		{"kind": "query", "sql": "SELECT next_seq FROM e2e_txseq WHERE topic = ?", "args": []any{topic}},
	}}

	const commits = 12
	var mu sync.Mutex
	got := map[string]int{}
	t.Run("concurrent", func(t *testing.T) {
		for i := range commits {
			t.Run(fmt.Sprint(i), func(t *testing.T) {
				t.Parallel()
				res := call(t, fx, fn, map[string]any{"op": "db_tx", "ops": allocate})
				results, _ := res["results"].([]any)
				if res["committed"] != true || len(results) != 2 {
					t.Fatalf("allocation did not commit: %v", trim(res))
				}
				read, _ := results[1].(map[string]any)
				mu.Lock()
				got[fmt.Sprint(read["rows"])]++
				mu.Unlock()
			})
		}
	})
	for seq := 1; seq <= commits; seq++ {
		if key := fmt.Sprintf("[map[next_seq:%d]]", seq); got[key] != 1 {
			t.Errorf("seq %d read back by %d commits, want exactly one; reads: %v", seq, got[key], got)
		}
	}

	rollback := map[string]any{"ops": []map[string]any{
		{"kind": "exec", "sql": "UPDATE e2e_txseq SET next_seq = 1000 WHERE topic = ?", "args": []any{topic}},
		{"kind": "query", "sql": "SELECT nosuch FROM e2e_txseq"},
	}}
	if res := call(t, fx, fn, map[string]any{"op": "db_tx", "ops": rollback}); res["committed"] != false || fmt.Sprint(res["failed_index"]) != "1" {
		t.Errorf("a failing query did not roll the batch back at op 1: %v", trim(res))
	}
	after := call(t, fx, fn, map[string]any{"op": "db_query",
		"sql": "SELECT next_seq FROM e2e_txseq WHERE topic = '" + topic + "'"})
	if rows := fmt.Sprint(after["rows"]); !strings.Contains(rows, fmt.Sprintf("next_seq:%d", commits)) {
		t.Errorf("after the rollback next_seq rows = %s, want %d", rows, commits)
	}
}
