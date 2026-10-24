package namespace

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

// The port allocator records blocks in namespace_port_allocations, but the
// table is not the port: a namespace that lost its rows, or any other process,
// can hold a block the table calls free. Allocating it made every service of
// the new namespace crash-loop on "address already in use" (bug-276). So the
// allocator asks the target node which ports of the range are held before it
// picks a block, and skips them.
//
// The node answers from /proc/net rather than by binding each port: a probe
// that binds can take a port away from a service starting on it at that very
// moment, and one read covers the whole range.

// ActionProbePorts is the spawn action that asks a node which ports of the
// namespace range are held on it.
const ActionProbePorts = "probe-ports"

// PortProber reports which ports of the namespace range are held on a node.
type PortProber interface {
	BoundPorts(ctx context.Context, namespaceClusterID, nodeID string) ([]int, error)
}

// procNetFiles are the kernel's socket tables, by protocol.
var procNetFiles = []struct {
	path string
	tcp  bool
}{
	{"/proc/net/tcp", true},
	{"/proc/net/tcp6", true},
	{"/proc/net/udp", false},
	{"/proc/net/udp6", false},
}

// tcpListen is the st column of a listening TCP socket in /proc/net/tcp.
const tcpListen = "0A"

// BoundPortsInRange returns, sorted, the ports in start..end that a socket on
// this host holds: a listening TCP socket or a bound UDP socket (Olric's
// memberlist uses both on one port).
func BoundPortsInRange(start, end int) ([]int, error) {
	held := map[int]bool{}
	for _, f := range procNetFiles {
		data, err := os.ReadFile(f.path)
		if errors.Is(err, os.ErrNotExist) && strings.HasSuffix(f.path, "6") {
			continue // a kernel without IPv6 has no v6 table
		}
		if err != nil {
			return nil, fmt.Errorf("read the socket table %s: %w", f.path, err)
		}
		if err := heldPorts(data, f.tcp, start, end, held); err != nil {
			return nil, fmt.Errorf("parse the socket table %s: %w", f.path, err)
		}
	}
	out := make([]int, 0, len(held))
	for p := range held {
		out = append(out, p)
	}
	sort.Ints(out)
	return out, nil
}

// heldPorts adds to held the local ports in start..end of the sockets in one
// /proc/net table: every bound UDP socket, and TCP sockets in LISTEN.
func heldPorts(table []byte, tcp bool, start, end int, held map[int]bool) error {
	sc := bufio.NewScanner(bytes.NewReader(table))
	first := true
	for sc.Scan() {
		if first { // the column header
			first = false
			continue
		}
		fields := strings.Fields(sc.Text())
		if len(fields) < 4 {
			continue
		}
		if tcp && fields[3] != tcpListen {
			continue
		}
		_, portHex, ok := strings.Cut(fields[1], ":")
		if !ok {
			return fmt.Errorf("local address %q has no port", fields[1])
		}
		port, err := strconv.ParseUint(portHex, 16, 16)
		if err != nil {
			return fmt.Errorf("local address %q: %w", fields[1], err)
		}
		if p := int(port); p >= start && p <= end {
			held[p] = true
		}
	}
	return sc.Err()
}

// errProbeUnsupported is a node on a release before ActionProbePorts.
var errProbeUnsupported = errors.New("the node does not know the probe-ports action")

// BoundPorts implements PortProber: this node reads its own socket tables, and
// any other node is asked over the coordination channel. A node on a release
// that predates the action answers errProbeUnsupported; the spawner's own
// refusal of a held port (portInUse) still stands there.
func (cm *ClusterManager) BoundPorts(ctx context.Context, namespaceClusterID, nodeID string) ([]int, error) {
	if nodeID == cm.localNodeID {
		return BoundPortsInRange(NamespacePortRangeStart, NamespacePortRangeEnd)
	}
	cluster, err := cm.GetCluster(ctx, namespaceClusterID)
	if err != nil {
		return nil, fmt.Errorf("read cluster %s to probe node %s: %w", namespaceClusterID, nodeID, err)
	}
	if cluster == nil {
		return nil, fmt.Errorf("cluster %s does not exist, so no port block can be allocated for it", namespaceClusterID)
	}
	ip, err := cm.nodeInternalIP(nodeID)
	if err != nil {
		return nil, err
	}
	resp, err := cm.sendSpawnRequest(ctx, ip, map[string]interface{}{
		"action":     ActionProbePorts,
		"cluster_id": namespaceClusterID,
		"namespace":  cluster.NamespaceName,
		"node_id":    nodeID,
	})
	if err != nil {
		if strings.Contains(err.Error(), "unknown action: "+ActionProbePorts) {
			return nil, fmt.Errorf("%w: node %s: %v", errProbeUnsupported, nodeID, err)
		}
		return nil, fmt.Errorf("ask node %s which namespace ports it holds: %w", nodeID, err)
	}
	return resp.BoundPorts, nil
}
