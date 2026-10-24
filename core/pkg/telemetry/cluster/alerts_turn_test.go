package cluster

import (
	"testing"

	"github.com/Orama-Network/orama/core/pkg/telemetry/report"
)

// bug-386: an inactive shared TURN unit warns only where the node holds a
// TURN allocation; a failed one is always critical.
func TestCheckNodeServices_sharedTURNUnit(t *testing.T) {
	for _, tc := range []struct {
		name      string
		svc       report.ServiceInfo
		wantAlert bool
	}{
		{"not needed and inactive", report.ServiceInfo{Name: "orama-turn", ActiveState: "inactive", NotNeeded: true}, false},
		{"needed and inactive", report.ServiceInfo{Name: "orama-turn", ActiveState: "inactive"}, true},
		{"not needed but failed", report.ServiceInfo{Name: "orama-turn", ActiveState: "failed", NotNeeded: true}, true},
		{"needed and active", report.ServiceInfo{Name: "orama-turn", ActiveState: "active"}, false},
	} {
		r := &report.NodeReport{Services: &report.ServicesReport{Services: []report.ServiceInfo{tc.svc}}}
		if got := len(checkNodeServices(r, "node-1", &nodeContext{})) > 0; got != tc.wantAlert {
			t.Errorf("%s: alert=%v, want %v", tc.name, got, tc.wantAlert)
		}
	}
}
