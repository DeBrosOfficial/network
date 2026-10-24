package ipfs

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"
)

const storedCID = "QmfYzxZHqYpmy29rVWqs6f4igzYngACaxSxPWdf7FspuDV"

// fakeNode is a Kubo API and a cluster API on one test server.
type fakeNode struct {
	pinnedLocally bool
	inPinset      bool
	pinsetStalls  bool
	networked     atomic.Int32
	offlineReads  atomic.Int32
	pinsetAsked   atomic.Int32
}

func (n *fakeNode) serve(t *testing.T) (*Client, string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/allocations/"):
			n.pinsetAsked.Add(1)
			if n.pinsetStalls {
				<-r.Context().Done()
				return
			}
			if n.inPinset {
				_, _ = io.WriteString(w, `{}`)
				return
			}
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"code":404,"message":"pin is not part of the pinset"}`)
		case r.URL.Path == "/api/v0/pin/ls":
			if r.URL.Query().Get("type") != "recursive" {
				t.Errorf("pin/ls type=%q, want recursive: only a recursive pin holds the whole DAG", r.URL.Query().Get("type"))
			}
			if n.pinnedLocally {
				_, _ = fmt.Fprintf(w, `{"Keys":{%q:{"Type":"recursive"}}}`, r.URL.Query().Get("arg"))
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = fmt.Fprintf(w, `{"Message":"path '%s' is not pinned","Code":0,"Type":"error"}`, r.URL.Query().Get("arg"))
		case r.URL.Query().Get("offline") == "true":
			n.offlineReads.Add(1)
			writeCat(w, "local bytes")
		default:
			n.networked.Add(1)
			writeCat(w, "peer bytes")
		}
	}))
	t.Cleanup(srv.Close)
	c, err := NewClient(Config{ClusterAPIURL: srv.URL}, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	return c, srv.URL
}

// writeCat answers a cat the way Kubo does: the length in X-Content-Length.
func writeCat(w http.ResponseWriter, body string) {
	w.Header().Set("X-Content-Length", strconv.Itoa(len(body)))
	_, _ = io.WriteString(w, body)
}

func read(t *testing.T, r io.ReadCloser) string {
	t.Helper()
	defer r.Close()
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestGetStored_localPinIsReadOfflineWithoutTheCluster(t *testing.T) {
	n := &fakeNode{pinnedLocally: true}
	c, url := n.serve(t)
	got, err := c.GetStored(context.Background(), storedCID, url, 0)
	if err != nil {
		t.Fatal(err)
	}
	if body := read(t, got.Body); body != "local bytes" || got.Size != int64(len(body)) || n.pinsetAsked.Load() != 0 {
		t.Fatalf("body %q size %d, pinset asked %d times; want the local bytes, their size and no pinset lookup", body, got.Size, n.pinsetAsked.Load())
	}
}

// Blocks this node only cached (a fetch that was cut off) are not read offline:
// the stream could stop partway, after its headers were sent.
func TestGetStored_unpinnedLocalBlocksAreNotReadOffline(t *testing.T) {
	n := &fakeNode{inPinset: true}
	c, url := n.serve(t)
	got, err := c.GetStored(context.Background(), storedCID, url, 0)
	if err != nil || read(t, got.Body) != "peer bytes" || n.offlineReads.Load() != 0 {
		t.Fatalf("err=%v offline reads %d; want the networked read only", err, n.offlineReads.Load())
	}
}

func TestGetStored_goneIsErrNotInPinsetWithoutANetworkSearch(t *testing.T) {
	n := &fakeNode{}
	c, url := n.serve(t)
	_, err := c.GetStored(context.Background(), storedCID, url, 0)
	if !errors.Is(err, ErrNotInPinset) || n.networked.Load() != 0 {
		t.Fatalf("err=%v networked=%d; want ErrNotInPinset and no network search", err, n.networked.Load())
	}
}

func TestGetStored_aStalledPinsetIsBounded(t *testing.T) {
	n := &fakeNode{pinsetStalls: true}
	c, url := n.serve(t)
	start := time.Now()
	_, err := c.GetStored(context.Background(), storedCID, url, 0)
	if !errors.Is(err, ErrPinsetUnavailable) {
		t.Fatalf("err=%v, want ErrPinsetUnavailable", err)
	}
	if took := time.Since(start); took > pinsetLookupTimeout+2*time.Second {
		t.Errorf("a stalled pinset held the read %s", took)
	}
}

// Kubo streams `cat`: a failure partway through arrives in the X-Stream-Error
// trailer after a 200 and some bytes. It must not read as a complete object.
func TestCatOnce_streamErrorTrailerIsAnError(t *testing.T) {
	for _, tc := range []struct {
		name     string
		offline  bool
		trailer  string
		wantMiss bool
	}{
		{"networked failure mid-stream", false, "context deadline exceeded", false},
		{"offline miss partway through the DAG", true, "block was not found locally (offline): ipld: could not find Qm", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Trailer", "X-Stream-Error")
				w.WriteHeader(http.StatusOK)
				_, _ = io.WriteString(w, "partial")
				w.Header().Set("X-Stream-Error", tc.trailer)
			}))
			defer srv.Close()
			c, err := NewClient(Config{ClusterAPIURL: srv.URL}, zap.NewNop())
			if err != nil {
				t.Fatal(err)
			}
			body, err := c.catOnce(context.Background(), srv.URL, "Qm", tc.offline, 0)
			if err != nil {
				t.Fatal(err)
			}
			defer body.Close()
			_, err = io.ReadAll(body)
			if err == nil {
				t.Fatal("a stream Kubo reported as failed read as complete")
			}
			if isContentNotFound(err) != tc.wantMiss {
				t.Errorf("err=%v, content-not-found=%v, want %v", err, isContentNotFound(err), tc.wantMiss)
			}
		})
	}
}

// When the caller's own deadline ends the pinset lookup, the answer is a
// timeout (retry later), not "the cluster could not be reached".
func TestGetStored_callerDeadlineDuringLookupIsATimeout(t *testing.T) {
	n := &fakeNode{pinsetStalls: true}
	c, url := n.serve(t)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_, err := c.GetStored(ctx, storedCID, url, 0)
	if !errors.Is(err, context.DeadlineExceeded) || errors.Is(err, ErrPinsetUnavailable) {
		t.Fatalf("err=%v; want DeadlineExceeded, not ErrPinsetUnavailable", err)
	}
}

// pinnedServer serves a locally pinned CID's cat with handler.
func pinnedServer(t *testing.T, key []byte, cat http.HandlerFunc) (*Client, string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v0/pin/ls" {
			_, _ = fmt.Fprintf(w, `{"Keys":{%q:{"Type":"recursive"}}}`, r.URL.Query().Get("arg"))
			return
		}
		cat(w, r)
	}))
	t.Cleanup(srv.Close)
	c, err := NewClient(Config{ClusterAPIURL: srv.URL, WrapKey: key}, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	return c, srv.URL
}

// bugboard #428: content in the clear is streamed. GetStored answers with the
// size Kubo announced while the object is still arriving.
func TestGetStored_streamsContentInTheClear(t *testing.T) {
	release := make(chan struct{})
	first, rest := "first chunk ", "and the rest"
	c, url := pinnedServer(t, nil, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Length", strconv.Itoa(len(first)+len(rest)))
		_, _ = io.WriteString(w, first)
		w.(http.Flusher).Flush()
		<-release
		_, _ = io.WriteString(w, rest)
	})
	got, err := c.GetStored(context.Background(), storedCID, url, 0)
	if err != nil {
		close(release)
		t.Fatalf("GetStored waited for or failed on the whole object: %v", err)
	}
	if got.Size != int64(len(first)+len(rest)) {
		t.Errorf("Size = %d, want %d", got.Size, len(first)+len(rest))
	}
	buf := make([]byte, len(first))
	if _, err := io.ReadFull(got.Body, buf); err != nil || string(buf) != first {
		t.Fatalf("first bytes = %q, %v", buf, err)
	}
	close(release)
	if tail := read(t, got.Body); tail != rest {
		t.Fatalf("rest = %q, want %q", tail, rest)
	}
}

// Sealed content is one AES-GCM message: it is opened whole before any of it
// is released, and Size is the plaintext's.
func TestGetStored_opensSealedContentWhole(t *testing.T) {
	key := bytes.Repeat([]byte{0x42}, 32)
	plain := []byte("tenant-bytes-sealed-at-rest")
	sealed, err := sealBlob(plain, key)
	if err != nil {
		t.Fatal(err)
	}
	c, url := pinnedServer(t, key, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Length", strconv.Itoa(len(sealed)))
		_, _ = w.Write(sealed)
	})
	got, err := c.GetStored(context.Background(), storedCID, url, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got.Size != int64(len(plain)) || read(t, got.Body) != string(plain) {
		t.Fatalf("Size %d; want the plaintext and its size %d", got.Size, len(plain))
	}
}

// Content shorter than the seal's magic is content in the clear.
func TestGetStored_tinyContentInTheClear(t *testing.T) {
	c, url := pinnedServer(t, nil, func(w http.ResponseWriter, r *http.Request) { writeCat(w, "hi") })
	got, err := c.GetStored(context.Background(), storedCID, url, 0)
	if err != nil || got.Size != 2 || read(t, got.Body) != "hi" {
		t.Fatalf("err=%v size=%v", err, got)
	}
}

func TestGetStored_noAnnouncedLengthIsAnError(t *testing.T) {
	c, url := pinnedServer(t, nil, func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "unsized content") })
	if _, err := c.GetStored(context.Background(), storedCID, url, 0); err == nil || !strings.Contains(err.Error(), "X-Content-Length") {
		t.Fatalf("err=%v, want the missing length named", err)
	}
}

// The idle timeout bounds each wait, not the whole transfer: an object that
// keeps arriving for several idle periods is read to the end.
func TestGetStored_idleTimeoutDoesNotBoundAnObjectThatKeepsArriving(t *testing.T) {
	const idle = 150 * time.Millisecond
	const chunks = 8
	c, url := pinnedServer(t, nil, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Length", strconv.Itoa(chunks))
		for i := 0; i < chunks; i++ {
			_, _ = io.WriteString(w, "x")
			w.(http.Flusher).Flush()
			time.Sleep(idle / 2)
		}
	})
	got, err := c.GetStored(context.Background(), storedCID, url, idle)
	if err != nil {
		t.Fatal(err)
	}
	if body := read(t, got.Body); len(body) != chunks {
		t.Fatalf("read %d of %d bytes over %s with an idle timeout of %s", len(body), chunks, chunks*idle/2, idle)
	}
}

// A stream that stops arriving fails with ErrIdle, which is a timeout.
func TestGetStored_aStalledStreamFailsWithErrIdle(t *testing.T) {
	const idle = 150 * time.Millisecond
	c, url := pinnedServer(t, nil, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Length", "100")
		_, _ = io.WriteString(w, "some bytes")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	})
	got, err := c.GetStored(context.Background(), storedCID, url, idle)
	if err != nil {
		t.Fatal(err)
	}
	defer got.Body.Close()
	_, err = io.ReadAll(got.Body)
	if !errors.Is(err, ErrIdle) || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err=%v, want ErrIdle (a DeadlineExceeded)", err)
	}
}

// A first byte that never comes is the same timeout, before GetStored returns.
func TestGetStored_noFirstByteFailsWithErrIdle(t *testing.T) {
	const idle = 150 * time.Millisecond
	c, url := pinnedServer(t, nil, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Length", "100")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	})
	start := time.Now()
	_, err := c.GetStored(context.Background(), storedCID, url, idle)
	if !errors.Is(err, ErrIdle) {
		t.Fatalf("err=%v, want ErrIdle", err)
	}
	if took := time.Since(start); took > idle+time.Second {
		t.Errorf("a stalled first byte held GetStored %s", took)
	}
}

// Get keeps its fully buffered reader with Len (bugboard #266).
type lenReader interface{ Len() int }

func TestGet_returnsAReaderWithLen(t *testing.T) {
	key := bytes.Repeat([]byte{0x42}, 32)
	plain := []byte("tenant-bytes-sealed-at-rest")
	sealed, err := sealBlob(plain, key)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(sealed)
	}))
	t.Cleanup(srv.Close)
	c, err := NewClient(Config{ClusterAPIURL: srv.URL, WrapKey: key}, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	r, err := c.Get(context.Background(), storedCID, srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	l, ok := r.(lenReader)
	if !ok || l.Len() != len(plain) {
		t.Fatalf("Get returned %T; want a reader with Len %d", r, len(plain))
	}
	if got := read(t, r); got != string(plain) {
		t.Errorf("body=%q", got)
	}
}

// The client's 60 s whole-call timeout must not cut a streamed read off: its
// bound is the idle timeout.
func TestGetStored_isNotBoundByTheClientTimeout(t *testing.T) {
	const chunks = 6
	interval := 100 * time.Millisecond
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v0/pin/ls" {
			_, _ = fmt.Fprintf(w, `{"Keys":{%q:{"Type":"recursive"}}}`, r.URL.Query().Get("arg"))
			return
		}
		w.Header().Set("X-Content-Length", strconv.Itoa(chunks))
		for i := 0; i < chunks; i++ {
			_, _ = io.WriteString(w, "x")
			w.(http.Flusher).Flush()
			time.Sleep(interval)
		}
	}))
	t.Cleanup(srv.Close)
	c, err := NewClient(Config{ClusterAPIURL: srv.URL, Timeout: 2 * interval}, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	got, err := c.GetStored(context.Background(), storedCID, srv.URL, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if body := read(t, got.Body); len(body) != chunks {
		t.Fatalf("read %d of %d bytes", len(body), chunks)
	}
}
