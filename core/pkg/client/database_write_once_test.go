package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// fakeRQLite answers reads and drops the connection on every write without
// replying: the write reached the node, and its outcome never reached the
// client. That is the case a retry turns into a double write.
func fakeRQLite(t *testing.T, writes *atomic.Int32) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/db/query":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"results":[{"columns":["1"],"types":["integer"],"values":[[1]]}]}`))
		case "/db/execute":
			writes.Add(1)
			conn, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Errorf("hijack: %v", err)
				return
			}
			_ = conn.Close()
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func connectedDatabaseClient(endpoint string) *DatabaseClientImpl {
	c := &Client{config: &ClientConfig{DatabaseEndpoints: []string{endpoint}}, connected: true}
	return &DatabaseClientImpl{client: c}
}

func TestDatabaseClient_writeWithUnknownOutcome_isSentOnce(t *testing.T) {
	ctx := WithInternalAuth(context.Background())
	cases := []struct {
		name string
		run  func(d *DatabaseClientImpl) error
	}{
		{"query", func(d *DatabaseClientImpl) error {
			_, err := d.Query(ctx, "INSERT INTO t (a) VALUES (?)", 1)
			return err
		}},
		{"transaction", func(d *DatabaseClientImpl) error {
			return d.Transaction(ctx, []string{"INSERT INTO t (a) VALUES (1)", "INSERT INTO t (a) VALUES (2)"})
		}},
		{"create table", func(d *DatabaseClientImpl) error {
			return d.CreateTable(ctx, "CREATE TABLE t (a INTEGER)")
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var writes atomic.Int32
			srv := fakeRQLite(t, &writes)
			if err := tc.run(connectedDatabaseClient(srv.URL)); err == nil {
				t.Fatal("a write whose response was lost must fail, got nil")
			}
			if got := writes.Load(); got != 1 {
				t.Fatalf("write sent %d times, want 1: a lost response does not mean the write did not land", got)
			}
		})
	}
}
