package health

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type fixedStatus map[string]bool

func (f fixedStatus) GetStatus() map[string]bool { return f }

// reportsTo starts a gateway stand-in that hands every report it receives to
// the returned channel.
func reportsTo(t *testing.T) (string, <-chan Report) {
	t.Helper()
	got := make(chan Report, 4)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != GatewayHealthEndpoint {
			t.Errorf("report sent as %s %s, want POST %s", r.Method, r.URL.Path, GatewayHealthEndpoint)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type %q", ct)
		}
		var rep Report
		if err := json.NewDecoder(r.Body).Decode(&rep); err != nil {
			t.Errorf("report is not JSON: %v", err)
		}
		got <- rep
	}))
	t.Cleanup(srv.Close)
	return srv.URL + GatewayHealthEndpoint, got
}

func TestSendReport_healthyOnlyWhenEveryServiceRuns(t *testing.T) {
	for name, tc := range map[string]struct {
		status fixedStatus
		want   bool
	}{
		"all running": {fixedStatus{"rqlite": true, "gateway": true}, true},
		"one down":    {fixedStatus{"rqlite": true, "gateway": false}, false},
	} {
		url, got := reportsTo(t)
		newReporter(tc.status, url).sendReport()
		rep := <-got
		if rep.Healthy != tc.want {
			t.Errorf("%s: healthy = %v, want %v", name, rep.Healthy, tc.want)
		}
		if len(rep.Services) != len(tc.status) {
			t.Errorf("%s: services %v, want %v", name, rep.Services, tc.status)
		}
		for svc, running := range tc.status {
			if rep.Services[svc] != running {
				t.Errorf("%s: %s reported %v, want %v", name, svc, rep.Services[svc], running)
			}
		}
		if rep.NodeID == "" || rep.Version == "" {
			t.Errorf("%s: node id %q or version %q is empty, want the value or \"unknown\"", name, rep.NodeID, rep.Version)
		}
		if time.Since(rep.Timestamp) > time.Minute || rep.Uptime < 0 {
			t.Errorf("%s: timestamp %v uptime %d", name, rep.Timestamp, rep.Uptime)
		}
	}
}

// The gateway is not up yet early in boot; a report that cannot be delivered
// is dropped, not retried or fatal.
func TestSendReport_noGatewayIsNotFatal(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL + GatewayHealthEndpoint
	srv.Close()
	newReporter(fixedStatus{"rqlite": true}, url).sendReport()
}

func TestRunLoop_reportsAtOnceAndStopsWhenAsked(t *testing.T) {
	url, got := reportsTo(t)
	r := newReporter(fixedStatus{"rqlite": true}, url)
	done := make(chan struct{})
	go func() {
		r.RunLoop()
		close(done)
	}()
	select {
	case <-got:
	case <-time.After(10 * time.Second):
		t.Fatal("no report on start; the first one waited for the interval")
	}
	r.Stop()
	r.Stop() // a second Stop must not panic on the closed channel
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("RunLoop did not return after Stop")
	}
}
