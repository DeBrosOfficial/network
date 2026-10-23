package rqlite_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/Orama-Network/orama/core/pkg/rqlite"
	"github.com/Orama-Network/orama/core/pkg/rqlite/rqlitetest"
)

// feat-950: a batch's queries run inside its transaction, in input order, so
// a query sees what the ops before it wrote and never a later commit's state.

// requestRecorder is an rqlite stand-in that records every request's path,
// query string and statements, and answers with body.
type requestRecorder struct {
	mu       sync.Mutex
	paths    []string
	queries  []string
	stmts    [][]string
	response string
}

func (r *requestRecorder) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	raw, _ := io.ReadAll(req.Body)
	var stmts [][]any
	_ = json.Unmarshal(raw, &stmts)
	sqls := make([]string, 0, len(stmts))
	for _, s := range stmts {
		if len(s) > 0 {
			sql, _ := s[0].(string)
			sqls = append(sqls, sql)
		}
	}
	r.mu.Lock()
	r.paths = append(r.paths, req.URL.Path)
	r.queries = append(r.queries, req.URL.RawQuery)
	r.stmts = append(r.stmts, sqls)
	r.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(r.response))
}

func TestBatch_sendsEveryOpInOrderAsOneTransactionalRequest(t *testing.T) {
	rec := &requestRecorder{response: `{"results":[
		{"last_insert_id":7,"rows_affected":1},
		{"columns":["n"],"types":["integer"],"values":[[3]]},
		{"last_insert_id":0,"rows_affected":2}]}`}
	srv := httptest.NewServer(rec)
	defer srv.Close()

	ops := []rqlite.BatchOp{
		{Kind: rqlite.BatchOpExec, SQL: "INSERT INTO t(v) VALUES (1)"},
		{Kind: rqlite.BatchOpQuery, SQL: "SELECT count(*) AS n FROM t"},
		{Kind: rqlite.BatchOpExec, SQL: "UPDATE t SET v = 2"},
	}
	res, err := clientFor(t, srv).Batch(context.Background(), ops)
	if err != nil || !res.Committed {
		t.Fatalf("Batch = %+v, %v; want committed", res, err)
	}

	if len(rec.paths) != 1 || rec.paths[0] != "/db/request" {
		t.Fatalf("requests = %v; want exactly one to /db/request", rec.paths)
	}
	if !strings.Contains("&"+rec.queries[0]+"&", "&transaction&") {
		t.Errorf("query string %q does not ask for a transaction", rec.queries[0])
	}
	for i, op := range ops {
		if i >= len(rec.stmts[0]) || rec.stmts[0][i] != op.SQL {
			t.Fatalf("statements sent = %q; want the ops in input order", rec.stmts[0])
		}
	}

	want := []rqlite.OpResult{
		{Kind: rqlite.BatchOpExec, RowsAffected: 1, LastInsertID: 7},
		{Kind: rqlite.BatchOpQuery, Rows: []map[string]interface{}{{"n": float64(3)}}},
		{Kind: rqlite.BatchOpExec, RowsAffected: 2},
	}
	got, _ := json.Marshal(res.Results)
	exp, _ := json.Marshal(want)
	if string(got) != string(exp) {
		t.Errorf("results = %s\nwant      %s", got, exp)
	}
}

func TestBatch_refusesAnEmptyStatement(t *testing.T) {
	rec := &requestRecorder{response: `{"results":[]}`}
	srv := httptest.NewServer(rec)
	defer srv.Close()

	res, err := clientFor(t, srv).Batch(context.Background(), []rqlite.BatchOp{
		{Kind: rqlite.BatchOpExec, SQL: "INSERT INTO t(v) VALUES (1)"},
		{Kind: rqlite.BatchOpQuery, SQL: "  "},
	})
	if err == nil || res != nil {
		t.Fatalf("Batch = %+v, %v; want a validation error and no result", res, err)
	}
	if len(rec.paths) != 0 {
		t.Errorf("a refused batch reached rqlite: %v", rec.paths)
	}
}

func createCounter(t *testing.T, c rqlite.Client) {
	t.Helper()
	res, err := c.Batch(context.Background(), []rqlite.BatchOp{
		{Kind: rqlite.BatchOpExec, SQL: "CREATE TABLE seq (topic TEXT PRIMARY KEY, next_seq INTEGER NOT NULL)"},
		{Kind: rqlite.BatchOpExec, SQL: "INSERT INTO seq(topic, next_seq) VALUES ('a', 1)"},
	})
	if err != nil || !res.Committed {
		t.Fatalf("create counter: res=%+v err=%v", res, err)
	}
}

func intCell(t *testing.T, r rqlite.OpResult, col string) int64 {
	t.Helper()
	if r.Error != "" || len(r.Rows) != 1 {
		t.Fatalf("want one row, got %+v", r)
	}
	v, ok := r.Rows[0][col].(float64)
	if !ok {
		t.Fatalf("column %q = %#v, want a number", col, r.Rows[0][col])
	}
	return int64(v)
}

func TestBatch_queryReadsItsOwnTransactionsWrites_liveRqlite(t *testing.T) {
	c := rqlitetest.Start(t)
	createCounter(t, c)

	res, err := c.Batch(context.Background(), []rqlite.BatchOp{
		{Kind: rqlite.BatchOpQuery, SQL: "SELECT next_seq FROM seq WHERE topic = ?", Args: []interface{}{"a"}},
		{Kind: rqlite.BatchOpExec, SQL: "UPDATE seq SET next_seq = next_seq + 1 WHERE topic = ?", Args: []interface{}{"a"}},
		{Kind: rqlite.BatchOpQuery, SQL: "SELECT next_seq FROM seq WHERE topic = ?", Args: []interface{}{"a"}},
		{Kind: rqlite.BatchOpExec, SQL: "UPDATE seq SET next_seq = next_seq * 10 WHERE topic = ?", Args: []interface{}{"a"}},
		{Kind: rqlite.BatchOpQuery, SQL: "SELECT next_seq FROM seq WHERE topic = ?", Args: []interface{}{"a"}},
	})
	if err != nil || !res.Committed {
		t.Fatalf("Batch = %+v, %v; want committed", res, err)
	}
	for i, want := range map[int]int64{0: 1, 2: 2, 4: 20} {
		if got := intCell(t, res.Results[i], "next_seq"); got != want {
			t.Errorf("op %d read next_seq = %d, want %d (the state after the ops before it)", i, got, want)
		}
	}
	if res.Results[1].RowsAffected != 1 || res.Results[1].Kind != rqlite.BatchOpExec {
		t.Errorf("exec result = %+v, want kind exec with rows_affected 1", res.Results[1])
	}
}

func TestBatch_concurrentCommitsEachReadTheirOwnValue_liveRqlite(t *testing.T) {
	// The AnChat failure: allocate a seq in a commit and read it back. With the
	// read after the commit, a commit landing in between handed two callers
	// the same (later) value.
	c := rqlitetest.Start(t)
	createCounter(t, c)

	const callers = 40
	seqs := make(chan int64, callers)
	var wg sync.WaitGroup
	for range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := c.Batch(context.Background(), []rqlite.BatchOp{
				{Kind: rqlite.BatchOpExec, SQL: "UPDATE seq SET next_seq = next_seq + 1 WHERE topic = 'a'"},
				{Kind: rqlite.BatchOpQuery, SQL: "SELECT next_seq FROM seq WHERE topic = 'a'"},
			})
			if err != nil || !res.Committed {
				t.Errorf("Batch = %+v, %v", res, err)
				return
			}
			seqs <- intCell(t, res.Results[1], "next_seq")
		}()
	}
	wg.Wait()
	close(seqs)

	seen := map[int64]bool{}
	for s := range seqs {
		if seen[s] {
			t.Errorf("seq %d handed to two commits", s)
		}
		seen[s] = true
	}
	for s := int64(2); s <= callers+1; s++ {
		if !seen[s] {
			t.Errorf("seq %d handed to no commit; got %v", s, seen)
		}
	}
}

func TestBatch_failingStatementRollsBackEverything_liveRqlite(t *testing.T) {
	c := rqlitetest.Start(t)
	createCounter(t, c)
	ctx := context.Background()

	for _, tc := range []struct {
		name   string
		failer rqlite.BatchOp
	}{
		{"failing exec", rqlite.BatchOp{Kind: rqlite.BatchOpExec, SQL: "INSERT INTO seq(topic, next_seq) VALUES ('a', 9)"}},
		{"failing query", rqlite.BatchOp{Kind: rqlite.BatchOpQuery, SQL: "SELECT nosuch FROM seq"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res, err := c.Batch(ctx, []rqlite.BatchOp{
				{Kind: rqlite.BatchOpExec, SQL: "INSERT INTO seq(topic, next_seq) VALUES ('b', 1)"},
				{Kind: rqlite.BatchOpQuery, SQL: "SELECT count(*) AS n FROM seq"},
				tc.failer,
				{Kind: rqlite.BatchOpExec, SQL: "UPDATE seq SET next_seq = 100"},
			})
			if err == nil || res == nil || res.Committed || res.FailedIndex != 2 {
				t.Fatalf("Batch = %+v, %v; want a rollback failing at op 2", res, err)
			}
			if res.Error != "" || res.Code != "" {
				t.Errorf("batch-level error/code %q/%q; a statement failure belongs to its op", res.Error, res.Code)
			}
			failed := res.Results[2]
			if failed.Error == "" || failed.Code == "" || failed.Kind != tc.failer.Kind {
				t.Errorf("failing op = %+v, want its kind, error and code", failed)
			}
			for _, i := range []int{0, 1, 3} {
				if r := res.Results[i]; r.Kind != "" || r.Rows != nil || r.RowsAffected != 0 {
					t.Errorf("op %d = %+v; only the failing op is reported on a rollback", i, r)
				}
			}

			after, err := c.Batch(ctx, []rqlite.BatchOp{
				{Kind: rqlite.BatchOpQuery, SQL: "SELECT count(*) AS n FROM seq"},
				{Kind: rqlite.BatchOpQuery, SQL: "SELECT next_seq FROM seq WHERE topic = 'a'"},
			})
			if err != nil || !after.Committed {
				t.Fatalf("read back = %+v, %v", after, err)
			}
			if n := intCell(t, after.Results[0], "n"); n != 1 {
				t.Errorf("%d rows after the rollback, want 1: the insert before the failure was kept", n)
			}
			if v := intCell(t, after.Results[1], "next_seq"); v != 1 {
				t.Errorf("next_seq = %d after the rollback, want 1", v)
			}
		})
	}
}

func TestBatch_execReturningCarriesItsRows_liveRqlite(t *testing.T) {
	c := rqlitetest.Start(t)
	createCounter(t, c)

	res, err := c.Batch(context.Background(), []rqlite.BatchOp{{
		Kind: rqlite.BatchOpExec,
		SQL:  "UPDATE seq SET next_seq = next_seq + 1 WHERE topic = 'a' RETURNING next_seq",
	}})
	if err != nil || !res.Committed {
		t.Fatalf("Batch = %+v, %v; want committed", res, err)
	}
	if res.Results[0].Kind != rqlite.BatchOpExec {
		t.Errorf("kind = %q, want exec", res.Results[0].Kind)
	}
	if got := intCell(t, res.Results[0], "next_seq"); got != 2 {
		t.Errorf("RETURNING next_seq = %d, want 2", got)
	}
}

func TestBatchWithSeq_concurrentCommitsGetDistinctSeqs_liveRqlite(t *testing.T) {
	c := rqlitetest.Start(t)
	ctx := context.Background()
	res, err := c.Batch(ctx, []rqlite.BatchOp{{Kind: rqlite.BatchOpExec,
		SQL: "CREATE TABLE namespace_publish_seq (namespace TEXT PRIMARY KEY, next_seq INTEGER NOT NULL, updated_at INTEGER NOT NULL)"}})
	if err != nil || !res.Committed {
		t.Fatalf("create table: %+v, %v", res, err)
	}

	const callers = 40
	seqs := make(chan int64, callers)
	var wg sync.WaitGroup
	for range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, seq, err := c.BatchWithSeq(ctx, "ns", []rqlite.BatchOp{{Kind: rqlite.BatchOpQuery, SQL: "SELECT 1"}})
			if err != nil {
				t.Errorf("BatchWithSeq: %v", err)
				return
			}
			seqs <- seq
		}()
	}
	wg.Wait()
	close(seqs)

	seen := map[int64]bool{}
	for s := range seqs {
		if seen[s] {
			t.Errorf("seq %d assigned to two commits", s)
		}
		seen[s] = true
	}
	if len(seen) != callers {
		t.Errorf("%d distinct seqs for %d commits", len(seen), callers)
	}
}
