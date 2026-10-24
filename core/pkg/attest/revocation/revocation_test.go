package revocation

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Orama-Network/orama/core/pkg/attest/internal/feed"
)

// status_excerpt.json is ten entries of the real list as Google served it on
// 2026-10-22, including serials that are all decimal digits (factory
// certificate serials whose hex happens to have no letters).
func excerpt(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/status_excerpt.json")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

type clock struct{ now atomic.Pointer[time.Time] }

func newClock(t time.Time) *clock { c := &clock{}; c.set(t); return c }
func (c *clock) set(t time.Time)  { c.now.Store(&t) }
func (c *clock) Now() time.Time   { return *c.now.Load() }

func TestParse_realExcerptAndGooglesSchemaExample(t *testing.T) {
	l, err := Parse(excerpt(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Entries) != 10 {
		t.Fatalf("%d entries", len(l.Entries))
	}
	example := `{"entries":{"2c8cdddfd5e03bfc":{"status":"REVOKED","expires":"2020-11-13","reason":"KEY_COMPROMISE","comment":"Key stored on unsecure system"},"c8966fcb2fbb0d7a":{"status":"SUSPENDED","reason":"SOFTWARE_FLAW","comment":"Bug in keystore causes this key malfunction b/555555"}}}`
	if _, err := Parse([]byte(example)); err != nil {
		t.Fatal(err)
	}
	for name, bad := range map[string]string{
		"unknown status":   `{"entries":{"ab":{"status":"FINE"}}}`,
		"unknown field":    `{"entries":{"ab":{"status":"REVOKED","severity":"high"}}}`,
		"no entries":       `{}`,
		"trailing data":    `{"entries":{}} {}`,
		"not JSON":         `<html>`,
		"top-level extras": `{"entries":{},"next":"x"}`,
	} {
		if _, err := Parse([]byte(bad)); err == nil {
			t.Errorf("%s: parsed", name)
		}
	}
}

func TestRefresher_honoursMaxAgeKeepsLastGoodAndGoesStale(t *testing.T) {
	var fail atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail.Load() {
			http.Error(w, "down", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=86400")
		w.Header().Set("Age", "3600")
		w.Write(excerpt(t))
	}))
	defer srv.Close()
	clk := newClock(time.Date(2026, 10, 23, 0, 0, 0, 0, time.UTC))
	cache := filepath.Join(t.TempDir(), "status.json")
	r, err := New(Config{URL: srv.URL, Client: srv.Client(), CacheFile: cache, Now: clk.Now})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := r.Revoked("c35747a084470c3135aeefe2b8d40cd6"); !errors.Is(err, ErrNoList) {
		t.Fatalf("before any list: %v", err)
	}
	if !r.Stale() {
		t.Fatal("no list is stale")
	}

	if wait := r.RefreshOnce(t.Context()); wait != 23*time.Hour {
		t.Fatalf("next refresh in %s, want max-age less Age (23h)", wait)
	}
	for serial, want := range map[string]bool{"c35747a084470c3135aeefe2b8d40cd6": true, "6681152659205225093": true, "1234": false} {
		if got, err := r.Revoked(serial); err != nil || got != want {
			t.Fatalf("Revoked(%s) = %v, %v", serial, got, err)
		}
	}

	fail.Store(true)
	clk.set(clk.Now().Add(48 * time.Hour))
	if wait := r.RefreshOnce(t.Context()); wait != feed.RetryAfterFailure {
		t.Fatalf("after a failure, next try in %s", wait)
	}
	if got, err := r.Revoked("c35747a084470c3135aeefe2b8d40cd6"); err != nil || !got {
		t.Fatal("the last good list was dropped after a failed fetch")
	}
	if r.Stale() || r.Status().LastError == nil {
		t.Fatalf("48h old is not stale yet, and the failure is reported: %+v", r.Status())
	}
	clk.set(clk.Now().Add(25 * time.Hour))
	if !r.Stale() {
		t.Fatal("73h old is stale")
	}

	again, err := New(Config{URL: srv.URL, Client: srv.Client(), CacheFile: cache, Now: clk.Now})
	if err != nil {
		t.Fatal(err)
	}
	if got, err := again.Revoked("c35747a084470c3135aeefe2b8d40cd6"); err != nil || !got {
		t.Fatalf("the cached list after a restart: %v, %v", got, err)
	}
}

func TestRefresher_refusesABadListAndKeepsTheGoodOne(t *testing.T) {
	var body atomic.Value
	body.Store(excerpt(t))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(body.Load().([]byte))
	}))
	defer srv.Close()
	r, err := New(Config{URL: srv.URL, Client: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	if wait := r.RefreshOnce(t.Context()); wait != feed.MinRefresh {
		t.Fatalf("without max-age, next refresh in %s, want %s", wait, feed.MinRefresh)
	}
	body.Store([]byte(`{"entries":{"ab":{"status":"MAYBE"}}}`))
	r.RefreshOnce(t.Context())
	if got, _ := r.Revoked("c35747a084470c3135aeefe2b8d40cd6"); !got {
		t.Fatal("a refused list replaced the good one")
	}
}

func TestNew_refusesACorruptCache(t *testing.T) {
	cache := filepath.Join(t.TempDir(), "status.json")
	if err := os.WriteFile(cache, []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := New(Config{CacheFile: cache}); err == nil {
		t.Fatal("loaded a corrupt cache")
	}
}
