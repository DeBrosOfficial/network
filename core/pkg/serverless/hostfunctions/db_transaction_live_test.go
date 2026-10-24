package hostfunctions

import (
	"encoding/json"
	"testing"

	"github.com/Orama-Network/orama/core/pkg/rqlite"
	"github.com/Orama-Network/orama/core/pkg/rqlite/rqlitetest"
)

// feat-950 against a real rqlite: what a guest's db_transaction returns when a
// query reads back the transaction's own write, and when a statement fails.

func transact(t *testing.T, h *HostFunctions, opsJSON string) rqlite.BatchResult {
	t.Helper()
	out, err := h.DBTransaction(nsCtx(), []byte(opsJSON))
	if err != nil {
		t.Fatalf("DBTransaction: Go error %v; SQL failures belong in the envelope", err)
	}
	var res rqlite.BatchResult
	if err := json.Unmarshal(out, &res); err != nil {
		t.Fatalf("decode %s: %v", out, err)
	}
	return res
}

func TestDBTransaction_liveQueryReadsTheTransactionsOwnWrite(t *testing.T) {
	h := newHFWithDB(rqlitetest.Start(t))
	if res := transact(t, h, `{"ops":[
		{"kind":"exec","sql":"CREATE TABLE event_seq (topic TEXT PRIMARY KEY, next_seq INTEGER NOT NULL)"},
		{"kind":"exec","sql":"INSERT INTO event_seq VALUES ('t', 1)"}]}`); !res.Committed {
		t.Fatalf("setup: %+v", res)
	}

	res := transact(t, h, `{"ops":[
		{"kind":"exec","sql":"UPDATE event_seq SET next_seq = next_seq + 1 WHERE topic = ?","args":["t"]},
		{"kind":"query","sql":"SELECT next_seq FROM event_seq WHERE topic = ?","args":["t"]}]}`)
	if !res.Committed || len(res.Results) != 2 {
		t.Fatalf("result = %+v, want a committed two-op result", res)
	}
	if rows := res.Results[1].Rows; len(rows) != 1 || rows[0]["next_seq"] != float64(2) {
		t.Errorf("query rows = %v, want next_seq 2: the value this transaction wrote", rows)
	}

	failed := transact(t, h, `{"ops":[
		{"kind":"exec","sql":"UPDATE event_seq SET next_seq = next_seq + 1 WHERE topic = 't'"},
		{"kind":"exec","sql":"INSERT INTO event_seq VALUES ('t', 5)"}]}`)
	if failed.Committed || failed.FailedIndex != 1 || failed.Error != "" {
		t.Fatalf("result = %+v, want a rollback failing at op 1 with no batch-level error", failed)
	}
	if op := failed.Results[1]; op.Code != rqlite.BatchCodeConstraintViolation || op.Error == "" {
		t.Errorf("failing op = %+v, want %s with SQLite's reason", op, rqlite.BatchCodeConstraintViolation)
	}

	after := transact(t, h, `{"ops":[{"kind":"query","sql":"SELECT next_seq FROM event_seq WHERE topic = 't'"}]}`)
	if rows := after.Results[0].Rows; len(rows) != 1 || rows[0]["next_seq"] != float64(2) {
		t.Errorf("after the rollback next_seq rows = %v, want 2: the failed transaction's update was kept", rows)
	}
}
