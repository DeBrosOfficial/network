package httputil

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// trickle yields one byte every interval, n times.
type trickle struct {
	n        int
	interval time.Duration
}

func (t *trickle) Read(p []byte) (int, error) {
	if t.n == 0 {
		return 0, io.EOF
	}
	time.Sleep(t.interval)
	t.n--
	p[0] = 'x'
	return 1, nil
}

// bugboard #428: a server write timeout is a budget for the whole response,
// which cut a large download off however steadily it was moving.
// StreamWithIdleWrites keeps a body that keeps moving alive well past it.
func TestStreamWithIdleWrites_outlivesTheServersWriteTimeout(t *testing.T) {
	const writeTimeout = 200 * time.Millisecond
	const bytes = 10
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "10")
		_, _ = StreamWithIdleWrites(w, &trickle{n: bytes, interval: writeTimeout / 2}, writeTimeout)
	}))
	srv.Config.WriteTimeout = writeTimeout
	srv.Start()
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil || len(body) != bytes {
		t.Fatalf("read %d of %d bytes over %s with a %s write timeout: %v", len(body), bytes, bytes*writeTimeout/2, writeTimeout, err)
	}
}

func TestStreamWithIdleWrites_copiesWithoutDeadlineSupport(t *testing.T) {
	rec := httptest.NewRecorder()
	n, err := StreamWithIdleWrites(rec, strings.NewReader("content"), time.Second)
	if err != nil || n != 7 || rec.Body.String() != "content" {
		t.Fatalf("n=%d err=%v body=%q", n, err, rec.Body.String())
	}
}
