package auth

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Reading a row back.
//
// The same query returns different Go types depending on who ran it: the rqlite
// client hands back strings for everything, and go-sqlite3 — which is what the
// tests run against — parses a TIMESTAMP column into a time.Time and a numeric
// one into an int64. Code that reads a row has to accept both, or it passes its
// tests and misreads production.

// getStringVal reads a column as text.
func getStringVal(v any) string {
	switch value := v.(type) {
	case nil:
		return ""
	case string:
		return value
	case []byte:
		return string(value)
	default:
		return fmt.Sprintf("%v", value)
	}
}

// timestampLayouts are the shapes a TIMESTAMP column comes back in.
var timestampLayouts = []string{
	sqliteTime,
	time.RFC3339,
	"2006-01-02 15:04:05Z07:00",
	"2006-01-02 15:04:05 -0700 MST",
}

// parseTimestamp reads a TIMESTAMP column, and says whether it could.
//
// The three answers are genuinely different and callers act on all three: NULL
// (no value was written), a value, and a value nobody can read. Collapsing the
// third into either of the others is how a column that says "retired" or
// "expired" ends up meaning "live".
func parseTimestamp(cell any) (at time.Time, present, readable bool) {
	switch value := cell.(type) {
	case nil:
		return time.Time{}, false, true
	case time.Time:
		return value.UTC(), true, true
	}

	text := strings.TrimSpace(getStringVal(cell))
	if text == "" {
		return time.Time{}, false, true
	}
	for _, layout := range timestampLayouts {
		if t, err := time.Parse(layout, text); err == nil {
			return t.UTC(), true, true
		}
	}
	return time.Time{}, true, false
}

// parseIntCell reads an INTEGER column: an int64 from go-sqlite3, a JSON
// number from rqlite, or its text. A value that is none of them is an error,
// not a zero.
func parseIntCell(cell any) (int, error) {
	switch value := cell.(type) {
	case int64:
		return int(value), nil
	case int:
		return value, nil
	case float64:
		if value != float64(int(value)) {
			return 0, fmt.Errorf("%v is not an integer", value)
		}
		return int(value), nil
	}
	text := strings.TrimSpace(getStringVal(cell))
	n, err := strconv.Atoi(text)
	if err != nil {
		return 0, fmt.Errorf("%q is not an integer", text)
	}
	return n, nil
}
