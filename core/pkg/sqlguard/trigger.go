package sqlguard

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// A tenant may keep its schema's invariants with triggers (bugboard chg-943),
// on terms that leave a trigger no more reach than a statement sent directly.
//
// A trigger body is statements SQLite runs later, when a write to the
// trigger's table fires it, and the statement that fires it never names what
// the body does. So the body is judged once, when the trigger is created, and
// the trigger is accepted only if every statement it can ever run would have
// been accepted on its own:
//
//   - Its table, its WHEN clause and every body statement are held to the
//     name rules of Check: no platform table, as an identifier or as a string
//     literal. A trigger on a platform table is refused by the same rule.
//   - Every body statement is INSERT, REPLACE, UPDATE, DELETE or SELECT, the
//     statements SQLite's grammar allows in a trigger. ATTACH, PRAGMA, VACUUM
//     and a nested CREATE TRIGGER are refused here by name, not left to
//     SQLite's parser.
//   - It lives in the main schema. A TEMP trigger lives on the connection, not
//     in the replicated schema: every node that applied the CREATE has it, but
//     a node rebuilt from a Raft snapshot does not, so replicas diverge, and no
//     backup or image check would ever see it.
//   - Nothing in it reads the clock or randomness. rqlite makes a replicated
//     write deterministic by rewriting RANDOM(), RANDOMBLOB() and 'now' in the
//     statement it receives, never inside a trigger, so each node would compute
//     its own value. The date and time functions are refused whatever their
//     arguments: a value read from a row can be 'now'.
//   - Its text is at most maxTriggerSQLBytes.
//
// Recursion stays bounded by SQLite: recursive_triggers is off and PRAGMA is
// refused, so a trigger never fires itself, and a chain of triggers is at most
// as long as the number of triggers.
//
// The trigger is read the way SQLite reads it: the body runs from the first
// BEGIN after the trigger's name to the first END that is a statement of its
// own (`; END`). A CASE ... END inside a statement does not end the body.
// Where the guard and SQLite could disagree (a column named begin in the WHEN
// clause), the guard's reading puts SQL in a body statement that does not start
// with an allowed word, so it refuses; it never accepts more than SQLite runs.

// maxTriggerSQLBytes bounds a trigger's text. The text is stored in the schema
// and parsed again by every connection that opens the database, on every node,
// so one tenant's trigger must not make that parse arbitrarily slow. A trigger
// that keeps an invariant is a few hundred bytes; 64 KiB leaves two orders of
// magnitude of room.
const maxTriggerSQLBytes = 64 << 10

// maxQuotedStatement is how much of a refused body statement a refusal quotes.
const maxQuotedStatement = 120

// triggerBodyStatements are the statement kinds a trigger body may run.
var triggerBodyStatements = map[string]bool{
	"insert": true, "replace": true, "update": true, "delete": true, "select": true,
}

// nondeterministicFunctions are the SQL functions whose value differs from
// node to node when a trigger runs them (see the package notes above).
var nondeterministicFunctions = map[string]bool{
	"random": true, "randomblob": true,
	"date": true, "time": true, "datetime": true, "julianday": true,
	"unixepoch": true, "strftime": true, "timediff": true,
}

// nondeterministicKeywords read the clock without parentheses.
var nondeterministicKeywords = map[string]bool{
	"current_timestamp": true, "current_date": true, "current_time": true,
}

// checkCreateTrigger judges a CREATE TRIGGER statement (isCreateTrigger is
// true for tokens).
func checkCreateTrigger(query string, tokens []token) error {
	if len(query) > maxTriggerSQLBytes {
		return &ErrNotAllowed{Reason: fmt.Sprintf("a trigger's text may be at most %d bytes; this one is %d", maxTriggerSQLBytes, len(query))}
	}
	if isTempKeyword(strings.ToLower(tokens[1].text)) {
		return &ErrNotAllowed{Reason: "CREATE TEMP TRIGGER is not available to tenant SQL: a temporary trigger lives on one node's connection, not in the replicated schema, so replicas would diverge; create it in the main schema"}
	}
	nameAt, err := triggerNameIndex(tokens)
	if err != nil {
		return err
	}
	begin := triggerBodyStart(tokens, nameAt)
	if begin < 0 {
		return &ErrNotAllowed{Reason: "CREATE TRIGGER needs a BEGIN ... END body"}
	}
	for _, tok := range tokens[:begin] {
		if tok.kind == tokenSemicolon {
			return &ErrNotAllowed{Reason: "one call runs one statement; send them one at a time"}
		}
	}
	if err := checkTriggerTokens(tokens[:begin]); err != nil {
		return refusedIn("the trigger's definition", err)
	}
	return checkTriggerBody(query, tokens[begin+1:])
}

// triggerNameIndex returns the index of the trigger's name, past CREATE
// TRIGGER and an optional IF NOT EXISTS, and refuses a name qualified by a
// schema other than main: `CREATE TRIGGER temp.t` is a TEMP trigger by
// another spelling.
func triggerNameIndex(tokens []token) (int, error) {
	at := 2 // past CREATE TRIGGER
	if w := leadingWords(tokens[at:], 3); len(w) == 3 && w[0] == "if" && w[1] == "not" && w[2] == "exists" {
		at += 3
	}
	if at+1 < len(tokens) && tokens[at+1].kind == tokenDot {
		if schema := normalizeName(tokens[at].text); schema != "main" {
			return 0, &ErrNotAllowed{Reason: fmt.Sprintf("a trigger in schema %q is not available to tenant SQL; only the main schema is replicated", schema)}
		}
		at += 2
	}
	if at >= len(tokens) {
		return 0, &ErrNotAllowed{Reason: "CREATE TRIGGER needs a name"}
	}
	return at, nil
}

// triggerBodyStart returns the index of the BEGIN that opens the body: the
// first word BEGIN after the name that is not a qualified column (`new.begin`).
// It returns -1 when there is none.
func triggerBodyStart(tokens []token, nameAt int) int {
	for i := nameAt + 1; i < len(tokens); i++ {
		tok := tokens[i]
		if tok.kind == tokenWord && strings.EqualFold(tok.text, "begin") && tokens[i-1].kind != tokenDot {
			return i
		}
	}
	return -1
}

// checkTriggerBody judges the statements between BEGIN and END, and refuses
// anything after END but semicolons.
func checkTriggerBody(query string, tokens []token) error {
	n := 0
	for start := 0; start < len(tokens); {
		end := start
		for end < len(tokens) && tokens[end].kind != tokenSemicolon {
			end++
		}
		stmt := tokens[start:end]
		next := end + 1
		switch {
		case len(stmt) == 1 && stmt[0].kind == tokenWord && strings.EqualFold(stmt[0].text, "end"):
			if next < len(tokens) && hasMoreContent(tokens[next:]) {
				return &ErrNotAllowed{Reason: "one call runs one statement; send them one at a time"}
			}
			return nil
		case len(stmt) > 0:
			n++
			if err := checkTriggerStatement(stmt); err != nil {
				return refusedIn(fmt.Sprintf("trigger body statement %d (%s)", n, quoteStatement(query, stmt)), err)
			}
		}
		start = next
	}
	return &ErrNotAllowed{Reason: "the trigger body has no END; a body is statements each ending in ';', then END"}
}

// checkTriggerStatement judges one statement of a trigger body.
func checkTriggerStatement(stmt []token) error {
	first := stmt[0]
	if first.kind != tokenWord || !triggerBodyStatements[strings.ToLower(first.text)] {
		return &ErrNotAllowed{Reason: fmt.Sprintf("%s is not available in a trigger body; a body runs INSERT, REPLACE, UPDATE, DELETE or SELECT", strings.ToUpper(first.text))}
	}
	return checkTriggerTokens(stmt)
}

// checkTriggerTokens holds part of a trigger to the name rules of Check and
// refuses what reads the clock or randomness.
func checkTriggerTokens(tokens []token) error {
	for i, tok := range tokens {
		if err := checkName(tok); err != nil {
			return err
		}
		if err := checkDeterministic(tokens, i); err != nil {
			return err
		}
	}
	return nil
}

// checkDeterministic refuses tokens[i] when it reads the clock or randomness:
// one of the clock keywords, or a call (a name followed by a parenthesis) of
// a function whose value differs between nodes.
func checkDeterministic(tokens []token, i int) error {
	tok := tokens[i]
	name := normalizeName(tok.text)
	if tok.kind == tokenWord && nondeterministicKeywords[name] {
		return &ErrNotAllowed{Reason: fmt.Sprintf("%s in a trigger is evaluated by each node on its own clock, so replicas would diverge; set the value in the statement that fires the trigger", strings.ToUpper(name))}
	}
	isName := tok.kind == tokenWord || tok.kind == tokenQuotedIdent
	isCall := i+1 < len(tokens) && tokens[i+1].kind == tokenOther && tokens[i+1].text == "("
	if isName && isCall && nondeterministicFunctions[name] {
		return &ErrNotAllowed{Reason: fmt.Sprintf("%s() in a trigger is evaluated by each node on its own, so replicas would diverge; compute the value in the statement that fires the trigger and store it in a column", name)}
	}
	return nil
}

// refusedIn prefixes a refusal with where in the trigger it was found.
func refusedIn(where string, err error) error {
	return &ErrNotAllowed{Reason: where + ": " + err.Error()}
}

// quoteStatement returns the text of stmt as written, shortened for a message.
func quoteStatement(query string, stmt []token) string {
	text := query[stmt[0].start:stmt[len(stmt)-1].end]
	if len(text) <= maxQuotedStatement {
		return text
	}
	cut := maxQuotedStatement
	for cut > 0 && !utf8.RuneStart(text[cut]) {
		cut--
	}
	return text[:cut] + "..."
}
