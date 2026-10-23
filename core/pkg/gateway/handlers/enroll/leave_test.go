package enroll

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Orama-Network/orama/core/pkg/rqlite"
	"go.uber.org/zap"
)

// leaveRegistry is the registry as HandleNodeLeave sees it: it records every
// write and answers every read with no rows (so the node has no agent token).
type leaveRegistry struct {
	rqlite.Client
	writes []string
}

func (r *leaveRegistry) Query(context.Context, any, string, ...any) error { return nil }

func (r *leaveRegistry) Exec(_ context.Context, query string, _ ...any) (sql.Result, error) {
	r.writes = append(r.writes, query)
	return nil, nil
}

// bug-720: a node whose agent cannot be asked to stop (here it has no agent
// token, as an Ubuntu node with no agent or an address no node holds) is not
// removed from the mesh: the call is 502 NODE_STOP_UNCONFIRMED and nothing is
// written.
func TestHandleNodeLeave_anUnconfirmedStopRemovesNothing(t *testing.T) {
	reg := &leaveRegistry{}
	h := &Handler{logger: zap.NewNop(), rqliteClient: reg}
	rec := httptest.NewRecorder()
	h.HandleNodeLeave(rec, httptest.NewRequest(http.MethodPost, "/v1/node/leave", bytes.NewBufferString(`{"wg_ip":"10.0.0.250"}`)))

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("HTTP %d %s, want 502", rec.Code, rec.Body)
	}
	var body struct{ Error, Code string }
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %q: %v", rec.Body, err)
	}
	if body.Code != ErrCodeNodeStopUnconfirmed || !strings.Contains(body.Error, "not removed") {
		t.Errorf("body %+v, want code %s and an error saying the node was not removed", body, ErrCodeNodeStopUnconfirmed)
	}
	if len(reg.writes) != 0 {
		t.Errorf("the registry was written: %v", reg.writes)
	}
}

// Only the agent's own confirmation, HTTP 200 {"status":"stopped"}, lets the
// leave go on.
func TestConfirmAgentStopped(t *testing.T) {
	cases := []struct {
		name   string
		body   string
		status int
		err    error
		ok     bool
	}{
		{"confirmed", `{"status":"stopped"}`, http.StatusOK, nil, true},
		{"unreachable", "", 0, errors.New("request to agent failed: dial tcp 10.0.0.250:9998: i/o timeout"), false},
		{"refused", `{"error":"forbidden"}`, http.StatusForbidden, nil, false},
		{"failed", `{"error":"stop failed"}`, http.StatusInternalServerError, nil, false},
		{"another answer", `{"status":"restarted"}`, http.StatusOK, nil, false},
		{"not json", `stopped`, http.StatusOK, nil, false},
	}
	for _, c := range cases {
		err := confirmAgentStopped([]byte(c.body), c.status, c.err)
		if c.ok != (err == nil) {
			t.Errorf("%s: err %v, want ok=%v", c.name, err, c.ok)
		}
		if err != nil && !errors.Is(err, errStopUnconfirmed) {
			t.Errorf("%s: %v is not errStopUnconfirmed", c.name, err)
		}
	}
}
