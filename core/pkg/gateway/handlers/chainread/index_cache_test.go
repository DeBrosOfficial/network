package chainread

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// cacheProxy is a proxy whose indexer answers every request with the given status and Cache-Control.
func cacheProxy(t *testing.T, status int, cacheControl string) *Proxy {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if cacheControl != "" {
			w.Header().Set("Cache-Control", cacheControl)
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(srv.Close)
	p, err := New(Config{RPCURL: "http://127.0.0.1:9", RESTURL: "http://127.0.0.1:9", IndexURL: srv.URL})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return p
}

func cacheControlOf(p *Proxy, path string) (int, string) {
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec.Code, rec.Header().Get("Cache-Control")
}

func TestIndexProxy_passesOnTheIndexersCacheLifetime(t *testing.T) {
	for _, tc := range []struct{ upstream, want string }{
		{"public, max-age=2", "public, max-age=2"},
		{"public, max-age=600", "public, max-age=600"},
		{"public, max-age=3600", "public, max-age=3600"},
	} {
		code, got := cacheControlOf(cacheProxy(t, http.StatusOK, tc.upstream), "/v1/chain/index/blocks/5")
		if code != http.StatusOK || got != tc.want {
			t.Errorf("upstream %q: HTTP %d, Cache-Control %q, want %q", tc.upstream, code, got, tc.want)
		}
	}
}

// Only the form the indexer writes is passed on, within the proxy's own ceiling; anything else, and
// anything that is not a success, is kept out of every cache.
func TestIndexProxy_keepsEverythingElseOutOfCaches(t *testing.T) {
	for _, upstream := range []string{
		"", "no-store", "private, max-age=60", "public, max-age=3601", "public, max-age=0", "public, max-age=99999",
		"public, max-age=60, immutable", "public, max-age=-1", "max-age=60", "public, s-maxage=60",
	} {
		code, got := cacheControlOf(cacheProxy(t, http.StatusOK, upstream), "/v1/chain/index/blocks/5")
		if code != http.StatusOK || got != "no-store" {
			t.Errorf("upstream %q: HTTP %d, Cache-Control %q, want no-store", upstream, code, got)
		}
	}
	for _, status := range []int{http.StatusNotFound, http.StatusBadGateway, http.StatusInternalServerError} {
		_, got := cacheControlOf(cacheProxy(t, status, "public, max-age=600"), "/v1/chain/index/blocks/5")
		if got != "no-store" {
			t.Errorf("HTTP %d from the indexer: Cache-Control %q, want no-store", status, got)
		}
	}
}

// The other chain routes are live reads and stay out of caches, and so do refusals.
func TestProxy_theNonIndexRoutesAreNeverCached(t *testing.T) {
	p, _ := indexProxy(t)
	for _, path := range []string{"/v1/chain/index/nonsense", "/v1/chain/nonsense", "/v1/chain/index/txs?page=0"} {
		_, got := cacheControlOf(p, path)
		if got != "no-store" {
			t.Errorf("%s: Cache-Control %q, want no-store", path, got)
		}
	}
}
