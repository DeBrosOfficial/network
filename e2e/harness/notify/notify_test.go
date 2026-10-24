package notify

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNew_refusesWhatIsNotAnHTTPSTopicURLWithoutNamingIt(t *testing.T) {
	for _, bad := range []string{"", "http://push.example.org/topic", "https://push.example.org", "https://push.example.org/", "https://push.example.org/a/b", "::"} {
		_, err := New(bad)
		if err == nil {
			t.Errorf("%q accepted", bad)
			continue
		}
		if bad != "" && strings.Contains(err.Error(), bad) {
			t.Errorf("the error names the URL: %v", err)
		}
	}
	if _, err := New("https://push.example.org/e2e-topic"); err != nil {
		t.Fatal(err)
	}
}

func TestSend_postsTheBodyWithTitlePriorityAndTags(t *testing.T) {
	var got *http.Request
	var body string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got, body = r, string(b)
		_, _ = w.Write([]byte(`{"id":"x"}`))
	}))
	t.Cleanup(srv.Close)
	n, err := New(srv.URL + "/e2e-topic")
	if err != nil {
		t.Fatal(err)
	}
	n.http = srv.Client()
	if err := n.Send(t.Context(), Message{Title: "Orama e2e FAIL", Body: "2 failed", Priority: PriorityHigh, Tags: []string{"x", "e2e"}}); err != nil {
		t.Fatal(err)
	}
	if got.Method != http.MethodPost || got.URL.Path != "/e2e-topic" || body != "2 failed" {
		t.Fatalf("%s %s %q", got.Method, got.URL.Path, body)
	}
	if got.Header.Get("Title") != "Orama e2e FAIL" || got.Header.Get("Priority") != "4" || got.Header.Get("Tags") != "x,e2e" {
		t.Fatalf("headers %v", got.Header)
	}
}

func TestSend_aRefusedPushIsAnErrorThatDoesNotNameTheTopic(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
	}))
	t.Cleanup(srv.Close)
	n, err := New(srv.URL + "/secret-topic")
	if err != nil {
		t.Fatal(err)
	}
	n.http = srv.Client()
	err = n.Send(t.Context(), Message{Title: "t", Body: "b", Priority: PriorityDefault})
	if err == nil || !strings.Contains(err.Error(), "HTTP 429") || strings.Contains(err.Error(), "secret-topic") {
		t.Fatalf("err = %v", err)
	}
	srv.Close()
	if err := n.Send(t.Context(), Message{Title: "t", Body: "b"}); err == nil || strings.Contains(err.Error(), "secret-topic") {
		t.Fatalf("unreachable server: %v", err)
	}
}
