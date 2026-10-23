//go:build e2e_fleet

package namespaces

import (
	"fmt"
	"testing"

	"github.com/Orama-Network/orama/e2e/features/internal/edge"
	"github.com/Orama-Network/orama/e2e/features/internal/infra"
	"github.com/Orama-Network/orama/e2e/features/internal/tenancy"
	"github.com/Orama-Network/orama/e2e/harness"
	"github.com/Orama-Network/orama/e2e/harness/fleet"
	"github.com/Orama-Network/orama/e2e/harness/ns"
)

// Namespace port range (core/pkg/namespace NamespacePortRangeStart/End).
const (
	namespacePortFirst = 10000
	namespacePortLast  = 10099
)

// nodeBlocks are the namespace port blocks recorded for the node at overlay
// address wgIP, as [start, end] pairs.
func nodeBlocks(t *testing.T, f *fleet.Fleet, wgIP string) [][2]int {
	t.Helper()
	res := infra.IndexQuery(t, f, f.State.Nodes[0],
		"SELECT npa.port_start, npa.port_end FROM namespace_port_allocations npa JOIN dns_nodes dn ON dn.id = npa.node_id WHERE dn.internal_ip = ?", wgIP)
	out := make([][2]int, 0, len(res.Values))
	for _, row := range res.Values {
		start, sok := row[0].(float64)
		end, eok := row[1].(float64)
		if !sok || !eok {
			t.Fatalf("namespace_port_allocations answered an unreadable row %v", row)
		}
		out = append(out, [2]int{int(start), int(end)})
	}
	return out
}

// firstUnrecordedPort is the lowest port of the namespace range no recorded
// block covers: the start of the block the allocator's table would hand out.
func firstUnrecordedPort(blocks [][2]int) (int, bool) {
	for p := namespacePortFirst; p <= namespacePortLast; p++ {
		covered := false
		for _, b := range blocks {
			if p >= b[0] && p <= b[1] {
				covered = true
				break
			}
		}
		if !covered {
			return p, true
		}
	}
	return 0, false
}

// holdPort keeps a TCP listener on port of n, outside any namespace and with
// no allocation row, until t's cleanup: the orphan of bug-276.
func holdPort(t *testing.T, f *fleet.Fleet, n fleet.Node, port int) {
	t.Helper()
	unit := edge.RandomLabel(t, "e2e-port-hold-") + ".service"
	t.Cleanup(func() { edge.RunInCleanup(t, f, n, "systemctl stop "+unit+" 2>/dev/null; true") })
	f.MustExec(t, n, fmt.Sprintf(
		`systemd-run --quiet --collect --unit %s -p DynamicUser=yes /usr/bin/python3 -c 'import socket,time; s=socket.socket(); s.bind(("0.0.0.0", %d)); s.listen(); time.sleep(3600)'`,
		unit, port))
	f.MustExec(t, n, fmt.Sprintf("for i in $(seq 50); do ss -Htln 'sport = :%d' | grep -q . && exit 0; sleep 0.1; done; exit 1", port))
}

// TestNamespaceCreate_skipsAPortTheNodeHolds (bug-276): a port of the
// namespace range that something holds on a node, with no allocation row
// for it, is not handed to a new namespace there. On every node the port the
// table would hand out next is held; the namespace created afterwards gets
// blocks around it and comes up (docs/whitepaper/technical-reference/vol1/09-namespaces.md
// "Port blocks", step 2).
func TestNamespaceCreate_skipsAPortTheNodeHolds(t *testing.T) {
	f := harness.Fleet(t)
	held := map[string]int{}
	for _, n := range f.State.Nodes {
		port, ok := firstUnrecordedPort(nodeBlocks(t, f, n.WGIP))
		if !ok {
			harness.SkipNotApplicable(t, n.Name+" has no unrecorded port in the namespace range")
		}
		holdPort(t, f, n, port)
		held[n.WGIP] = port
	}

	created := tenancy.Namespace(t, f, ns.Options{})
	res := infra.IndexQuery(t, f, f.State.Nodes[0],
		"SELECT dn.internal_ip, npa.port_start, npa.port_end FROM namespace_port_allocations npa JOIN namespace_clusters nc ON nc.id = npa.namespace_cluster_id JOIN dns_nodes dn ON dn.id = npa.node_id WHERE nc.namespace_name = ?",
		created.Name)
	if len(res.Values) == 0 {
		t.Fatalf("namespace %s has no port blocks recorded", created.Name)
	}
	for _, row := range res.Values {
		ip, _ := row[0].(string)
		start, _ := row[1].(float64)
		end, _ := row[2].(float64)
		if p, ok := held[ip]; ok && p >= int(start) && p <= int(end) {
			t.Errorf("namespace %s got %d-%d on %s, which covers the held port %d", created.Name, int(start), int(end), ip, p)
		}
	}
}
