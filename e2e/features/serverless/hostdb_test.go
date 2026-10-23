//go:build e2e_fleet

package serverless

import (
	"fmt"
	"strings"
	"testing"
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
