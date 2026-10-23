package namespace

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"go.uber.org/zap"
)

// A /proc/net/tcp excerpt: 10006 listening on 0.0.0.0 (the orphaned rqlite of
// bug-276), 10007 listening on 10.0.0.2, 10008 only as the local end of an
// established connection, and 22 listening outside the range.
const procNetTCP = `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 00000000:2716 00000000:0000 0A 00000000:00000000 00:00000000 00000000   999        0 1 1 0 100 0 0 10 0
   1: 0200000A:2717 00000000:0000 0A 00000000:00000000 00:00000000 00000000   999        0 2 1 0 100 0 0 10 0
   2: 0200000A:2718 0300000A:D431 01 00000000:00000000 00:00000000 00000000   999        0 3 1 0 100 0 0 10 0
   3: 00000000:0016 00000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 4 1 0 100 0 0 10 0
`

// A /proc/net/udp6 excerpt: Olric's memberlist bound on 10009.
const procNetUDP6 = `  sl  local_address                         remote_address                        st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode ref pointer drops
  0: 00000000000000000000000000000000:2719 00000000000000000000000000000000:0000 07 00000000:00000000 00:00000000 00000000   999        0 5 2 0 0
`

func TestHeldPorts_listeningTCPAndBoundUDPInTheRange(t *testing.T) {
	held := map[int]bool{}
	if err := heldPorts([]byte(procNetTCP), true, NamespacePortRangeStart, NamespacePortRangeEnd, held); err != nil {
		t.Fatal(err)
	}
	if err := heldPorts([]byte(procNetUDP6), false, NamespacePortRangeStart, NamespacePortRangeEnd, held); err != nil {
		t.Fatal(err)
	}
	want := map[int]bool{10006: true, 10007: true, 10009: true}
	if !reflect.DeepEqual(held, want) {
		t.Fatalf("held %v, want %v (an established connection's local port and a port outside the range are not held)", held, want)
	}
}

func TestHeldPorts_aMalformedAddressIsAnError(t *testing.T) {
	bad := "header\n   0: 00000000 00000000:0000 0A\n"
	if err := heldPorts([]byte(bad), true, 0, 65535, map[int]bool{}); err == nil {
		t.Fatal("a local address without a port was accepted")
	}
}

// fixedProber answers BoundPorts with fixed ports, or an error.
type fixedProber struct {
	ports []int
	err   error
}

func (p fixedProber) BoundPorts(context.Context, string, string) ([]int, error) {
	return p.ports, p.err
}

// insertedPortStart is the port_start of the allocation the mock recorded.
func insertedPortStart(t *testing.T, db *mockRQLiteClient) int {
	t.Helper()
	for _, c := range db.execCalls {
		if strings.Contains(c.Query, "INSERT INTO namespace_port_allocations") {
			return c.Args[3].(int)
		}
	}
	t.Fatal("no allocation was inserted")
	return 0
}

// bug-276: the table says the range is empty, but the node holds 10000 and
// 10003; the block starts after them instead of on them.
func TestAllocatePortBlock_skipsPortsTheNodeHolds(t *testing.T) {
	db := newMockRQLiteClient()
	npa := NewNamespacePortAllocator(db, zap.NewNop())
	npa.prober = fixedProber{ports: []int{10000, 10003}}

	block, _, err := npa.AllocatePortBlock(context.Background(), "node-1", "cluster-1", BlueprintTenant())
	if err != nil {
		t.Fatal(err)
	}
	if got := insertedPortStart(t, db); got != 10004 || block.PortStart != 10004 {
		t.Fatalf("block starts at %d (inserted %d), want 10004: the first 5 free ports after the held 10000 and 10003", block.PortStart, got)
	}
}

// A node that cannot be asked fails the allocation: picking a block blind is
// what crash-looped the namespace.
func TestAllocatePortBlock_aProbeFailureFailsTheAllocation(t *testing.T) {
	db := newMockRQLiteClient()
	npa := NewNamespacePortAllocator(db, zap.NewNop())
	npa.prober = fixedProber{err: errors.New("node unreachable")}

	if _, _, err := npa.AllocatePortBlock(context.Background(), "node-1", "cluster-1", BlueprintTenant()); err == nil {
		t.Fatal("allocated a block on a node that could not be probed")
	}
	if len(db.execCalls) != 0 {
		t.Fatalf("wrote %d statements after a failed probe", len(db.execCalls))
	}
}

// A node on a release before the probe allocates from the table, as that
// release did; its spawner still refuses a held port.
func TestAllocatePortBlock_aNodeWithoutTheProbeAllocatesFromTheTable(t *testing.T) {
	db := newMockRQLiteClient()
	npa := NewNamespacePortAllocator(db, zap.NewNop())
	npa.prober = fixedProber{err: errProbeUnsupported}

	if _, _, err := npa.AllocatePortBlock(context.Background(), "node-1", "cluster-1", BlueprintTenant()); err != nil {
		t.Fatal(err)
	}
	if got := insertedPortStart(t, db); got != NamespacePortRangeStart {
		t.Fatalf("block starts at %d, want %d", got, NamespacePortRangeStart)
	}
}

// probeDB answers the two reads BoundPorts makes for a remote node: the
// cluster's namespace and the node's overlay address.
func probeDB() *recoveryMockDB {
	db := &recoveryMockDB{}
	db.queryFunc = func(dest any, query string, _ ...any) error {
		switch {
		case strings.Contains(query, "FROM namespace_clusters WHERE id"):
			appendToSlice(dest, map[string]any{"ID": "c1", "NamespaceName": "acme"})
		case strings.Contains(query, "FROM dns_nodes WHERE id"):
			appendToSlice(dest, map[string]any{"InternalIP": "10.0.0.2"})
		}
		return nil
	}
	return db
}

// A remote node is asked with the probe-ports action, addressed to it, and its
// answer is the held ports.
func TestClusterManagerBoundPorts_asksTheRemoteNode(t *testing.T) {
	var sent map[string]interface{}
	var sentTo string
	cm := &ClusterManager{db: probeDB(), logger: zap.NewNop(), localNodeID: "node-a",
		spawnRequestFn: func(_ context.Context, ip string, req map[string]interface{}) (*spawnResponse, error) {
			sent, sentTo = req, ip
			return &spawnResponse{Success: true, BoundPorts: []int{10006}}, nil
		}}
	ports, err := cm.BoundPorts(context.Background(), "c1", "node-b")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(ports, []int{10006}) {
		t.Fatalf("ports %v, want [10006]", ports)
	}
	if sentTo != "10.0.0.2" || sent["action"] != ActionProbePorts || sent["node_id"] != "node-b" || sent["namespace"] != "acme" {
		t.Fatalf("sent %v to %s, want probe-ports for node-b of acme to 10.0.0.2", sent, sentTo)
	}
}

// A node on a release before the action says it does not know it; that is
// errProbeUnsupported, which the allocator tells from a failure.
func TestClusterManagerBoundPorts_anOlderNodeIsUnsupported(t *testing.T) {
	cm := &ClusterManager{db: probeDB(), logger: zap.NewNop(), localNodeID: "node-a",
		spawnRequestFn: func(context.Context, string, map[string]interface{}) (*spawnResponse, error) {
			return nil, errors.New("spawn request failed on 10.0.0.2: unknown action: probe-ports")
		}}
	if _, err := cm.BoundPorts(context.Background(), "c1", "node-b"); !errors.Is(err, errProbeUnsupported) {
		t.Fatalf("err %v, want errProbeUnsupported", err)
	}
}
