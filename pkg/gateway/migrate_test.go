package gateway

import "testing"

func TestParseMigrationVersion(t *testing.T) {
	cases := map[string]struct{
		name string
		ok   bool
	}{
		"001_init.sql":       {"001_init.sql", true},
		"10foobar.SQL":       {"10foobar.SQL", true},
		"abc.sql":            {"abc.sql", false},
		"":                   {"", false},
		"123_no_ext":         {"123_no_ext", true},
	}
	for _, c := range cases {
		_, ok := parseMigrationVersion(c.name)
		if ok != c.ok {
			t.Fatalf("for %q expected %v got %v", c.name, c.ok, ok)
		}
	}
}

func TestSplitSQLStatements(t *testing.T) {
	in := `-- comment
BEGIN;
CREATE TABLE t (id INTEGER);
-- another
INSERT INTO t VALUES (1); -- inline comment
COMMIT;
`
	out := splitSQLStatements(in)
	if len(out) != 2 {
		t.Fatalf("expected 2 statements, got %d: %#v", len(out), out)
	}
	if out[0] != "CREATE TABLE t (id INTEGER);" {
		t.Fatalf("unexpected first: %q", out[0])
	}
	if out[1] != "INSERT INTO t VALUES (1);" {
		t.Fatalf("unexpected second: %q", out[1])
	}
}
