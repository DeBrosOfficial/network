package sqlguard

import (
	"errors"
	"strings"
	"testing"
)

// The shape of the triggers a tenant keeps its invariants with (bugboard
// chg-943: AnChat's server schema holds 30 of them): when a row is blanked,
// deleted or superseded, the rows that only existed for it go in the same
// statement. Each is on a tenant table and touches only tenant tables.
var tenantTriggers = []string{
	`CREATE TRIGGER IF NOT EXISTS message_blanked AFTER UPDATE OF body ON messages
	 FOR EACH ROW WHEN new.body IS NULL AND old.body IS NOT NULL
	 BEGIN
	   DELETE FROM body_owed WHERE message_id = new.id;
	   DELETE FROM delivery_unopened WHERE message_id = new.id;
	 END`,
	"CREATE TRIGGER message_gone AFTER DELETE ON messages BEGIN DELETE FROM body_owed WHERE message_id = old.id; END;",
	"create trigger main.membership_ended after delete on members begin delete from device_mail where member_id = old.id; end",
	// CASE ... END inside a statement does not end the body.
	"CREATE TRIGGER count_down AFTER DELETE ON messages BEGIN UPDATE counts SET n = CASE WHEN n > 0 THEN n - 1 ELSE 0 END WHERE id = old.id; END",
	// A ';' or END inside a string literal or a comment is data.
	"CREATE TRIGGER log_it AFTER INSERT ON messages BEGIN INSERT INTO log(msg) VALUES ('a; END; b'); -- END;\n END",
	"CREATE TRIGGER guard_it BEFORE INSERT ON messages BEGIN SELECT RAISE(ABORT, 'negative') WHERE new.id < 0; END",
	"CREATE TRIGGER v_ins INSTEAD OF INSERT ON message_view BEGIN INSERT INTO messages(id, body) VALUES (new.id, new.body); END",
	"CREATE TRIGGER resend AFTER UPDATE ON messages BEGIN REPLACE INTO message_resends(message_id) VALUES (new.id); UPDATE counts SET n = n + 1 WHERE id = new.id; END",
	// A column named like a date function is not a call of it.
	"CREATE TRIGGER stamp AFTER INSERT ON messages BEGIN INSERT INTO log(date) VALUES (new.date); END",
	// A column called begin, qualified, does not open the body.
	"CREATE TRIGGER b AFTER INSERT ON spans WHEN new.begin > 0 BEGIN DELETE FROM log WHERE id = new.id; END",
	// A pragma function on the allowlist reads the tenant's own schema.
	"CREATE TRIGGER cols AFTER INSERT ON spans BEGIN INSERT INTO log(msg) SELECT name FROM pragma_table_info('spans'); END",
}

func TestCheck_acceptsTriggersOnTenantTables(t *testing.T) {
	for _, query := range tenantTriggers {
		if err := Check(query); err != nil {
			t.Errorf("Check(%q) = %v, want nil", query, err)
		}
	}
}

// A trigger is refused when anything it can ever run would be refused on its
// own, and the reason names the body statement that broke the rule.
func TestCheck_refusesATriggerBodyByTheRulesOfADirectStatement(t *testing.T) {
	const head = "CREATE TRIGGER t AFTER INSERT ON messages BEGIN "
	for _, tc := range []struct{ query, want string }{
		{head + "DELETE FROM api_keys; END", "trigger body statement 1 (DELETE FROM api_keys): the name api_keys is reserved"},
		{head + "DELETE FROM log; INSERT INTO log SELECT * FROM 'grants'; END", "trigger body statement 2 (INSERT INTO log SELECT * FROM 'grants')"},
		{head + `UPDATE "function_secrets" SET value = 'x'; END`, "function_secrets"},
		{head + "SELECT * FROM sqlite_dbpage; END", "sqlite_dbpage"},
		{head + "PRAGMA writable_schema = 1; END", "trigger body statement 1 (PRAGMA writable_schema = 1): PRAGMA is not available in a trigger body"},
		{head + "INSERT INTO log SELECT file FROM pragma_database_list; END", "trigger body statement 1 (INSERT INTO log SELECT file FROM pragma_database_list): pragma_database_list runs PRAGMA database_list"},
		{head + "INSERT INTO log SELECT * FROM 'pragma_compile_options'; END", "pragma_compile_options runs PRAGMA compile_options"},
		{"CREATE TRIGGER t AFTER INSERT ON messages WHEN EXISTS (SELECT 1 FROM pragma_database_list) BEGIN DELETE FROM log; END", "the trigger's definition: pragma_database_list runs PRAGMA database_list"},
		{head + "ATTACH DATABASE '/x.db' AS x; END", "ATTACH is not available in a trigger body"},
		{head + "VACUUM INTO '/x.db'; END", "VACUUM is not available in a trigger body"},
		{head + "DETACH DATABASE x; END", "DETACH is not available in a trigger body"},
		{head + "CREATE TRIGGER u AFTER INSERT ON log BEGIN SELECT 1; END; END", "CREATE is not available in a trigger body"},
		{head + "WITH x AS (SELECT 1) SELECT * FROM x; END", "WITH is not available in a trigger body"},
		{head + "\"api\"; END", "is not available in a trigger body"},
	} {
		err := Check(tc.query)
		var refused *ErrNotAllowed
		if !errors.As(err, &refused) || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("Check(%q) = %v, want a refusal containing %q", tc.query, err, tc.want)
		}
	}
}

func TestCheck_refusesATriggerOnOrNamingAPlatformTable(t *testing.T) {
	for _, query := range []string{
		"CREATE TRIGGER t AFTER INSERT ON api_keys BEGIN SELECT 1; END",
		"CREATE TRIGGER t AFTER UPDATE ON \"grants\" BEGIN DELETE FROM log; END",
		"CREATE TRIGGER t AFTER INSERT ON messages WHEN (SELECT COUNT(*) FROM principals) > 0 BEGIN DELETE FROM log; END",
		"CREATE TRIGGER namespaces AFTER INSERT ON messages BEGIN DELETE FROM log; END",
	} {
		err := Check(query)
		if err == nil || !strings.Contains(err.Error(), "the trigger's definition") {
			t.Errorf("Check(%q) = %v, want a refusal of the trigger's definition", query, err)
		}
	}
}

func TestCheck_refusesTemporaryTriggers(t *testing.T) {
	for _, query := range []string{
		"CREATE TEMP TRIGGER t AFTER INSERT ON messages BEGIN DELETE FROM log; END",
		"create temporary trigger t after insert on messages begin delete from log; end",
		"CREATE TRIGGER temp.t AFTER INSERT ON messages BEGIN DELETE FROM log; END",
		`CREATE TRIGGER "TEMP".t AFTER INSERT ON messages BEGIN DELETE FROM log; END`,
		"CREATE TRIGGER IF NOT EXISTS other.t AFTER INSERT ON messages BEGIN DELETE FROM log; END",
	} {
		var refused *ErrNotAllowed
		if err := Check(query); !errors.As(err, &refused) {
			t.Errorf("Check(%q) = %v, want a refusal", query, err)
		}
	}
}

// Every node runs a trigger on its own, and rqlite rewrites the clock and
// randomness only in the statement it receives, so a trigger that reads them
// would leave the replicas with different rows.
func TestCheck_refusesNondeterministicTriggers(t *testing.T) {
	const head = "CREATE TRIGGER t AFTER INSERT ON messages "
	for _, query := range []string{
		head + "BEGIN INSERT INTO log(v) VALUES (random()); END",
		head + "BEGIN INSERT INTO log(v) VALUES (RANDOMBLOB(4)); END",
		head + "BEGIN UPDATE messages SET at = datetime('now') WHERE id = new.id; END",
		head + "BEGIN UPDATE messages SET at = CURRENT_TIMESTAMP WHERE id = new.id; END",
		head + "BEGIN UPDATE messages SET at = strftime('%s') WHERE id = new.id; END",
		head + "BEGIN UPDATE messages SET at = julianday(new.at) WHERE id = new.id; END",
		head + `BEGIN INSERT INTO log(v) VALUES ("random"()); END`,
		head + "WHEN unixepoch() > new.expires BEGIN DELETE FROM log; END",
		head + "WHEN CURRENT_DATE > new.day BEGIN DELETE FROM log; END",
	} {
		err := Check(query)
		if err == nil || !strings.Contains(err.Error(), "replicas would diverge") {
			t.Errorf("Check(%q) = %v, want a nondeterminism refusal", query, err)
		}
	}
}

// One call runs one statement, a trigger included: nothing may follow its END.
func TestCheck_refusesAStatementAfterATrigger(t *testing.T) {
	for _, query := range []string{
		"CREATE TRIGGER t AFTER INSERT ON messages BEGIN DELETE FROM log; END; DELETE FROM messages",
		"CREATE TRIGGER t AFTER INSERT ON messages BEGIN DELETE FROM log; END; PRAGMA writable_schema = 1",
		"CREATE TRIGGER t AFTER INSERT ON messages; DELETE FROM log; BEGIN DELETE FROM log; END",
	} {
		err := Check(query)
		if err == nil || !strings.Contains(err.Error(), "one statement") {
			t.Errorf("Check(%q) = %v, want a one-statement refusal", query, err)
		}
	}
}

func TestCheck_refusesMalformedTriggers(t *testing.T) {
	for _, tc := range []struct{ query, want string }{
		{"CREATE TRIGGER t AFTER INSERT ON messages DELETE FROM log", "BEGIN ... END"},
		{"CREATE TRIGGER t AFTER INSERT ON messages BEGIN DELETE FROM log END", "no END"},
		{"CREATE TRIGGER t AFTER INSERT ON messages BEGIN DELETE FROM log;", "no END"},
		{"CREATE TRIGGER", "needs a name"},
		{"CREATE TRIGGER t AFTER INSERT ON messages BEGIN INSERT INTO log(v) VALUES ('" +
			strings.Repeat("x", maxTriggerSQLBytes) + "'); END", "at most"},
	} {
		err := Check(tc.query)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("Check(%.80q) = %v, want a refusal containing %q", tc.query, err, tc.want)
		}
	}
}

func TestCheck_triggerRefusalQuotesAShortenedStatement(t *testing.T) {
	long := "DELETE FROM api_keys WHERE id IN (" + strings.Repeat("1, ", 100) + "1)"
	err := Check("CREATE TRIGGER t AFTER INSERT ON messages BEGIN " + long + "; END")
	if err == nil || !strings.Contains(err.Error(), long[:maxQuotedStatement]+"...") || strings.Contains(err.Error(), long) {
		t.Errorf("refusal %v does not quote the statement shortened to %d bytes", err, maxQuotedStatement)
	}
}

// A scoped handle runs data manipulation only; the trigger allowance is for
// tenant SQL and does not reach it.
func TestCheckScope_stillRefusesTriggers(t *testing.T) {
	known := map[string]bool{"messages": true, "log": true}
	err := CheckScope(tenantTriggers[1], known, known)
	if err == nil || !strings.Contains(err.Error(), "CREATE TRIGGER") {
		t.Errorf("CheckScope accepted a trigger: %v", err)
	}
}

func TestCheck_wordsThatMentionTriggersAreNotTriggers(t *testing.T) {
	for _, query := range []string{
		"CREATE TABLE triggers (id INTEGER)",
		"CREATE INDEX trigger_idx ON messages(body)",
		"SELECT trigger FROM messages",
	} {
		if err := Check(query); err != nil {
			t.Errorf("refused %q: %v", query, err)
		}
	}
}
