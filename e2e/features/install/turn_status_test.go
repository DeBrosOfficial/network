//go:build e2e_fleet

package install

import (
	"encoding/json"
	"regexp"
	"testing"

	"github.com/Orama-Network/orama/e2e/features/internal/infra"
	"github.com/Orama-Network/orama/e2e/harness"
	"github.com/Orama-Network/orama/e2e/harness/monitor"
)

// turnUnit is the shared, host-level TURN unit every install writes
// (core/pkg/systemd HostTURNServiceName).
const turnUnit = "orama-turn"

// turnStatusRow is orama-turn's row of `orama node status`: active, or
// inactive with "(not needed)" when the node holds no TURN allocation.
var turnStatusRow = regexp.MustCompile(`(?m)^orama-turn\s+(active|inactive \(not needed\)|inactive)\s`)

// TestInstall_sharedTURNUnitOnTheStatusSurfaces (bug-386): `orama node
// status` and `orama node report` list orama-turn on every node, and on none
// is it down where the TURN reconciler wants it running: it is active, or not
// needed because the node holds no TURN allocation
// (website/src/docs/operator/webrtc-operations.mdx "Debugging").
func TestInstall_sharedTURNUnitOnTheStatusSurfaces(t *testing.T) {
	t.Parallel()
	f := harness.Fleet(t)
	for _, n := range f.State.Nodes {
		st := infra.OnNode(t, f, n, "node", "status")
		m := turnStatusRow.FindStringSubmatch(st.Stdout)
		if st.Exit != 0 || m == nil {
			t.Errorf("%s: orama node status does not list %s (exit %d):\n%s", n.Name, turnUnit, st.Exit, st.Stdout)
		} else if m[1] == "inactive" {
			t.Errorf("%s: %s is inactive where the TURN reconciler wants it running:\n%s", n.Name, turnUnit, st.Stdout)
		}

		out := infra.OnNode(t, f, n, "node", "report")
		var r monitor.NodeReport
		if err := json.Unmarshal([]byte(out.Stdout), &r); err != nil || r.Services == nil {
			t.Fatalf("%s: node report (exit %d) is not a report with services: %v", n.Name, out.Exit, err)
		}
		var found *monitor.Service
		for i := range r.Services.Services {
			if r.Services.Services[i].Name == turnUnit {
				found = &r.Services.Services[i]
			}
		}
		switch {
		case found == nil:
			t.Errorf("%s: the node report does not list %s", n.Name, turnUnit)
		case found.ActiveState != "active" && !found.NotNeeded:
			t.Errorf("%s: the node report has %s %s and needed", n.Name, turnUnit, found.ActiveState)
		}
	}
}
