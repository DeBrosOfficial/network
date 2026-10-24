package deployments

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/Orama-Network/orama/core/cmd/orama/internal/clierr"
	"github.com/spf13/cobra"
)

// envJWT is shaped like a token, so ORAMA_TOKEN is sent as it is.
const envJWT = "eyJhbGciOiJFZERTQSJ9.eyJzdWIiOiIweG93bmVyIn0.c2ln"

// deploymentGateway answers delete and rollback and counts the calls.
type deploymentGateway struct {
	*httptest.Server
	mu    sync.Mutex
	calls map[string]int
}

func newDeploymentGateway(t *testing.T) *deploymentGateway {
	t.Helper()
	g := &deploymentGateway{calls: map[string]int{}}
	g.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.mu.Lock()
		g.calls[r.Method+" "+r.URL.Path]++
		g.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"site","version":1,"status":"active"}`))
	}))
	t.Cleanup(g.Close)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ORAMA_API_URL", g.URL)
	t.Setenv("ORAMA_GATEWAY_URL", "")
	t.Setenv("ORAMA_GATEWAY", "")
	t.Setenv("ORAMA_TOKEN", envJWT)
	return g
}

func (g *deploymentGateway) count(call string) int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.calls[call]
}

// command is a cobra command reading stdin from in, so a prompt sees exactly
// what the test typed (or nothing).
func command(in string) *cobra.Command {
	c := &cobra.Command{}
	c.SetIn(strings.NewReader(in))
	c.SetOut(&bytes.Buffer{})
	return c
}

func setFlag(t *testing.T, flag *bool, v bool) {
	t.Helper()
	old := *flag
	*flag = v
	t.Cleanup(func() { *flag = old })
}

func setVersion(t *testing.T, v int) {
	t.Helper()
	old := rollbackVersion
	rollbackVersion = v
	t.Cleanup(func() { rollbackVersion = old })
}

const (
	deleteCall   = "DELETE /v1/deployments/delete"
	rollbackCall = "POST /v1/deployments/rollback"
)

func TestDelete_yesDeletesWithoutReadingStdin(t *testing.T) {
	g := newDeploymentGateway(t)
	setFlag(t, &deleteYes, true)

	if err := deleteDeployment(command(""), []string{"site"}); err != nil {
		t.Fatalf("app delete --yes: %v", err)
	}
	if g.count(deleteCall) != 1 {
		t.Fatalf("app delete --yes sent %d deletes, want 1", g.count(deleteCall))
	}
}

func TestDelete_answeredYesDeletes(t *testing.T) {
	g := newDeploymentGateway(t)
	setFlag(t, &deleteYes, false)

	if err := deleteDeployment(command("y\n"), []string{"site"}); err != nil {
		t.Fatalf("app delete answered y: %v", err)
	}
	if g.count(deleteCall) != 1 {
		t.Fatalf("a confirmed delete sent %d deletes, want 1", g.count(deleteCall))
	}
}

// Declining, or a script that types nothing, deletes nothing and exits with
// CodeAborted: it used to print "Cancelled" and exit 0.
func TestDelete_unconfirmedIsAbortedAndSendsNothing(t *testing.T) {
	for name, in := range map[string]string{"declined": "n\n", "no input": ""} {
		t.Run(name, func(t *testing.T) {
			g := newDeploymentGateway(t)
			setFlag(t, &deleteYes, false)

			err := deleteDeployment(command(in), []string{"site"})
			if clierr.CodeOf(err) != clierr.CodeAborted {
				t.Fatalf("an unconfirmed delete returned %v (code %d), want CodeAborted", err, clierr.CodeOf(err))
			}
			if g.count(deleteCall) != 0 {
				t.Fatal("an unconfirmed delete reached the gateway")
			}
		})
	}
}

func TestRollback_yesRollsBackWithoutReadingStdin(t *testing.T) {
	g := newDeploymentGateway(t)
	setFlag(t, &rollbackYes, true)
	setVersion(t, 1)

	if err := rollbackDeployment(command(""), []string{"site"}); err != nil {
		t.Fatalf("app rollback --yes: %v", err)
	}
	if g.count(rollbackCall) != 1 {
		t.Fatalf("app rollback --yes sent %d rollbacks, want 1", g.count(rollbackCall))
	}
}

func TestRollback_unconfirmedIsAbortedAndSendsNothing(t *testing.T) {
	g := newDeploymentGateway(t)
	setFlag(t, &rollbackYes, false)
	setVersion(t, 1)

	err := rollbackDeployment(command(""), []string{"site"})
	if clierr.CodeOf(err) != clierr.CodeAborted {
		t.Fatalf("an unconfirmed rollback returned %v, want CodeAborted", err)
	}
	if g.count(rollbackCall) != 0 {
		t.Fatal("an unconfirmed rollback reached the gateway")
	}
}

func TestDeleteAndRollback_offerYes(t *testing.T) {
	for _, c := range []*cobra.Command{DeleteCmd, RollbackCmd} {
		if c.Flags().Lookup("yes") == nil {
			t.Errorf("orama app %s has no --yes", c.Name())
		}
	}
}
