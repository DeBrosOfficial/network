package hostfunctions

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

const (
	fetchSecret     = "SECRET-QUERY-VALUE"
	fetchPathSecret = "SECRET-PATH-KEY"
	fetchUserinfo   = "SECRET-USERINFO"
	fetchHost       = "hidden-destination.example"
	fetchURL        = "https://" + fetchUserinfo + "@" + fetchHost + "/v2/" + fetchPathSecret + "/private/path?api_key=" + fetchSecret
)

// fetchURLParts is every part of fetchURL after the origin: none may reach the
// node log, nor the error the function is handed.
var fetchURLParts = []string{fetchSecret, fetchPathSecret, fetchUserinfo, "api_key", "/v2/", "/private/path"}

func observedHostFunctions(httpRT, anonRT roundTripperFunc) (*HostFunctions, func() string) {
	core, logs := observer.New(zapcore.DebugLevel)
	h := &HostFunctions{
		logger:         zap.New(core),
		httpClient:     &http.Client{Transport: httpRT},
		anonHTTPClient: &http.Client{Transport: anonRT},
	}
	return h, func() string {
		var out []string
		for _, e := range logs.All() {
			out = append(out, fmt.Sprintf("%s %v", e.Message, e.ContextMap()))
		}
		return strings.Join(out, "\n")
	}
}

func failingRT(msg string) roundTripperFunc {
	return func(*http.Request) (*http.Response, error) { return nil, errors.New(msg) }
}

type brokenBody struct{ msg string }

func (b brokenBody) Read([]byte) (int, error) { return 0, errors.New(b.msg) }

// A function's http_fetch URL can carry a credential in its path, query or
// user info, and a path can name the user. The node log keeps the origin and
// the cause, nothing after the host; the client's error, which quotes the whole
// URL, is not repeated to the function either.
func TestHTTPFetch_failureLogNamesOnlyTheOrigin(t *testing.T) {
	h, logged := observedHostFunctions(failingRT("connection reset by peer"), nil)

	raw, err := h.HTTPFetch(context.Background(), "GET", fetchURL, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := logged()
	if got == "" {
		t.Fatal("nothing was logged, so the test checks nothing")
	}
	assertNoURLParts(t, "the log", got)
	assertNoURLParts(t, "the envelope", string(raw))
	for _, kept := range []string{"origin:https://" + fetchHost, "connection reset by peer"} {
		if !strings.Contains(got, kept) {
			t.Errorf("the log lost %q: %s", kept, got)
		}
	}
	if !strings.Contains(string(raw), "connection reset by peer") {
		t.Errorf("the envelope lost the cause: %s", raw)
	}
}

func TestHTTPFetch_readFailureLogNamesOnlyTheOrigin(t *testing.T) {
	h, logged := observedHostFunctions(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{}, Request: r, Body: io.NopCloser(brokenBody{"body cut"})}, nil
	}, nil)

	raw, err := h.HTTPFetch(context.Background(), "GET", fetchURL, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := logged()
	if got == "" {
		t.Fatal("nothing was logged, so the test checks nothing")
	}
	assertNoURLParts(t, "the read failure log", got)
	assertNoURLParts(t, "the read failure envelope", string(raw))
}

// A URL the client cannot turn into a request is answered and logged without
// repeating it.
func TestHTTPFetch_requestCreationFailureNamesNoPath(t *testing.T) {
	h, logged := observedHostFunctions(failingRT("unreachable"), nil)

	raw, err := h.HTTPFetch(context.Background(), "BAD METHOD", fetchURL, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := logged()
	if got == "" {
		t.Fatal("nothing was logged, so the test checks nothing")
	}
	assertNoURLParts(t, "the request creation log", got)
	assertNoURLParts(t, "the request creation envelope", string(raw))
}

func assertNoURLParts(t *testing.T, where, got string) {
	t.Helper()
	for _, leaked := range fetchURLParts {
		if strings.Contains(got, leaked) {
			t.Errorf("%s repeats %q: %s", where, leaked, got)
		}
	}
}

// anon_fetch is the route a function takes so the node cannot say where it
// went: neither the URL nor an error naming the destination is logged.
func TestAnonFetch_failureLogNamesNoDestination(t *testing.T) {
	h, logged := observedHostFunctions(nil, failingRT("socks connect tcp 127.0.0.1:9050->"+fetchHost+":443: host unreachable"))

	raw, err := h.AnonFetch(context.Background(), "GET", fetchURL, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"status":0`) {
		t.Fatalf("the function was not told about the failure: %s", raw)
	}
	got := logged()
	if got == "" {
		t.Fatal("nothing was logged, so the test checks nothing")
	}
	for _, leaked := range append([]string{fetchHost, "9050"}, fetchURLParts...) {
		if strings.Contains(got, leaked) {
			t.Errorf("the anon_fetch log names %q: %s", leaked, got)
		}
	}
	if !strings.Contains(got, "error_class") {
		t.Errorf("the anon_fetch log lost its error class: %s", got)
	}
}
