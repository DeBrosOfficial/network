package rqlite_test

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/Orama-Network/orama/core/pkg/rqlite"
)

// feat-802: a linearizable read is served at rqlite's level=linearizable on
// every request, and when the node cannot serve one the read fails; it is
// never answered at a weaker level.

// levelRecorder is an rqlite /db/query stand-in that records the level each
// read asked for and answers with status.
type levelRecorder struct {
	mu     sync.Mutex
	levels []string
	status int
}

func (l *levelRecorder) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	l.mu.Lock()
	for _, part := range strings.Split(r.URL.RawQuery, "&") {
		if v, ok := strings.CutPrefix(part, "level="); ok {
			l.levels = append(l.levels, v)
		}
	}
	l.mu.Unlock()
	if l.status != http.StatusOK {
		http.Error(w, "leadership lost while committing log", l.status)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"results":[{"columns":["n"],"types":["integer"],"values":[[1]]}]}`))
}

func clientFor(t *testing.T, srv *httptest.Server) rqlite.Client {
	t.Helper()
	dsn := srv.URL + "?disableClusterDiscovery=true&level=weak"
	db, err := sql.Open("rqlite", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	c, err := rqlite.NewClientWithDSN(db, dsn)
	if err != nil {
		t.Fatalf("NewClientWithDSN: %v", err)
	}
	return c
}

type consistencyReader interface {
	BatchQueryConsistency(ctx context.Context, ops []rqlite.BatchOp, rc rqlite.ReadConsistency) ([]rqlite.OpResult, error)
}

func TestBatchQueryConsistency_linearizableAsksForLinearizable(t *testing.T) {
	rec := &levelRecorder{status: http.StatusOK}
	srv := httptest.NewServer(rec)
	defer srv.Close()
	c := clientFor(t, srv).(consistencyReader)

	ops := []rqlite.BatchOp{{Kind: rqlite.BatchOpQuery, SQL: "SELECT 1 AS n"}}
	for _, rc := range []rqlite.ReadConsistency{rqlite.ReadConsistencyLinearizable, rqlite.ReadConsistencyWeak, rqlite.ReadConsistencyLinearizable} {
		res, err := c.BatchQueryConsistency(context.Background(), ops, rc)
		if err != nil || len(res) != 1 || res[0].Error != "" || len(res[0].Rows) != 1 {
			t.Fatalf("%s read = %+v, %v", rc, res, err)
		}
	}
	want := []string{"linearizable", "weak", "linearizable"}
	if strings.Join(rec.levels, ",") != strings.Join(want, ",") {
		t.Errorf("levels asked = %v, want %v", rec.levels, want)
	}
}

func TestBatchQueryConsistency_linearizableFailsWhenItCannotBeServed(t *testing.T) {
	rec := &levelRecorder{status: http.StatusServiceUnavailable}
	srv := httptest.NewServer(rec)
	defer srv.Close()
	c := clientFor(t, srv).(consistencyReader)

	res, err := c.BatchQueryConsistency(context.Background(), []rqlite.BatchOp{{Kind: rqlite.BatchOpQuery, SQL: "SELECT 1"}}, rqlite.ReadConsistencyLinearizable)
	if err == nil || res != nil {
		t.Fatalf("a read the node could not serve = %+v, %v; want (nil, err)", res, err)
	}
	if got := rqlite.ClassifyBatchError(err); got != rqlite.BatchCodeUnavailable {
		t.Errorf("code = %q, want %s", got, rqlite.BatchCodeUnavailable)
	}
	for _, level := range rec.levels {
		if level != "linearizable" {
			t.Errorf("the read was retried at level %q", level)
		}
	}
}
