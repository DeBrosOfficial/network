package sqlguard

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// tenantSchema is the schema the tenant triggers in trigger_test.go are written
// against.
var tenantSchema = []string{
	"CREATE TABLE messages (id INTEGER PRIMARY KEY, body TEXT, at TEXT, date TEXT)",
	"CREATE TABLE members (id INTEGER PRIMARY KEY)",
	"CREATE TABLE spans (id INTEGER PRIMARY KEY, \"begin\" INTEGER)",
	"CREATE TABLE body_owed (message_id INTEGER)",
	"CREATE TABLE delivery_unopened (message_id INTEGER)",
	"CREATE TABLE device_mail (member_id INTEGER)",
	"CREATE TABLE message_resends (message_id INTEGER PRIMARY KEY)",
	"CREATE TABLE counts (id INTEGER PRIMARY KEY, n INTEGER)",
	"CREATE TABLE log (id INTEGER PRIMARY KEY, msg TEXT, v, date TEXT)",
	"CREATE VIEW message_view AS SELECT id, body FROM messages",
	// What a statement hidden after a trigger would destroy.
	"CREATE TABLE sentinel (x INTEGER)",
}

func openTenantDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	for _, ddl := range tenantSchema {
		if _, err := db.Exec(ddl); err != nil {
			t.Fatalf("%s: %v", ddl, err)
		}
	}
	return db
}

func triggerCount(t *testing.T, db *sql.DB) int {
	t.Helper()
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type = 'trigger'").Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// Every trigger the guard accepts is one SQLite creates as one trigger, and
// nothing else: the guard read the statement where SQLite reads it.
func TestCheck_acceptedTriggersAreWhatSQLiteCreates(t *testing.T) {
	for _, query := range tenantTriggers {
		db := openTenantDB(t)
		if _, err := db.Exec(query); err != nil {
			t.Errorf("SQLite refused an accepted trigger %q: %v", query, err)
			continue
		}
		if n := triggerCount(t, db); n != 1 {
			t.Errorf("%q made %d triggers, want 1", query, n)
		}
	}
}

// The cleanup a tenant keeps in a trigger happens in the statement that fires
// it.
func TestCheck_anAcceptedTriggerKeepsItsInvariant(t *testing.T) {
	db := openTenantDB(t)
	if err := Check(tenantTriggers[0]); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		tenantTriggers[0],
		"INSERT INTO messages (id, body) VALUES (1, 'hi')",
		"INSERT INTO body_owed VALUES (1)",
		"INSERT INTO delivery_unopened VALUES (1)",
		"UPDATE messages SET body = NULL WHERE id = 1",
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	var owed, unopened int
	if err := db.QueryRow("SELECT (SELECT COUNT(*) FROM body_owed), (SELECT COUNT(*) FROM delivery_unopened)").Scan(&owed, &unopened); err != nil {
		t.Fatal(err)
	}
	if owed != 0 || unopened != 0 {
		t.Errorf("after blanking the message: body_owed=%d delivery_unopened=%d, want 0 and 0", owed, unopened)
	}
}

// Wherever the guard and SQLite could disagree on where a trigger ends, a
// statement SQLite would run after it must not get through. Bodies are
// followed by nothing, a semicolon, or a statement that drops the sentinel
// table; whatever the guard accepts is run, and the sentinel must survive.
func TestCheck_noStatementHidesAfterATrigger(t *testing.T) {
	bodies := []string{
		"SELECT 1; END",
		"SELECT CASE WHEN 1 THEN 1 END; END",
		"SELECT 'END;'; END",
		"SELECT 1 /* ; END */; END",
		"SELECT 1; -- ; END\nEND",
		"SELECT 1; END END",
		"SELECT 1; \"END\"",
		"SELECT 1; [END]",
		"SELECT 1; END",
		"SELECT 1;; END",
	}
	suffixes := []string{"", ";", "; DROP TABLE sentinel", ";DROP TABLE sentinel;", " DROP TABLE sentinel", ";\nDROP TABLE sentinel", "; END; DROP TABLE sentinel"}
	heads := []string{
		"CREATE TRIGGER t AFTER INSERT ON messages BEGIN ",
		"CREATE TRIGGER t AFTER INSERT ON spans WHEN new.begin BEGIN ",
		"CREATE TRIGGER t AFTER INSERT ON messages WHEN (SELECT 1) BEGIN ",
	}
	accepted := 0
	for _, head := range heads {
		for _, body := range bodies {
			for _, suffix := range suffixes {
				query := head + body + suffix
				if Check(query) != nil {
					continue
				}
				accepted++
				db := openTenantDB(t)
				_, _ = db.Exec(query)
				var n int
				if err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE name = 'sentinel'").Scan(&n); err != nil || n != 1 {
					t.Errorf("the guard accepted %q and SQLite ran a statement after the trigger", query)
				}
			}
		}
	}
	if accepted == 0 {
		t.Fatal("the guard accepted none of the probes; the test is not testing anything")
	}
}

// A trigger SQLite stores is read back by the backup image check and the scrub
// through Check, from the text in sqlite_master: that text must be judged the
// same as the statement that created it.
func TestCheck_aStoredTriggerIsJudgedLikeItsCreate(t *testing.T) {
	db := openTenantDB(t)
	for _, q := range tenantTriggers {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		var stored string
		if err := db.QueryRow("SELECT sql FROM sqlite_master WHERE type = 'trigger' ORDER BY rowid DESC LIMIT 1").Scan(&stored); err != nil {
			t.Fatal(err)
		}
		if err := Check(stored); err != nil {
			t.Errorf("stored trigger %q refused: %v", stored, err)
		}
	}
}
