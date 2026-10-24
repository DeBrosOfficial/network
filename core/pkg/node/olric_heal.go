package node

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/Orama-Network/orama/core/pkg/config"
	"github.com/Orama-Network/orama/core/pkg/logging"
	"github.com/Orama-Network/orama/core/pkg/namespace"
	"github.com/Orama-Network/orama/core/pkg/olric"
	"github.com/Orama-Network/orama/core/pkg/rqlite"
	"go.uber.org/zap"
)

// The index Olric partition healer (bug-933). Every core node runs it; see
// pkg/olric/heal.go for the decision and why it restarts exactly one node.

const (
	// olricHealProbeTimeout bounds one member-list call to one node's Olric.
	olricHealProbeTimeout = 3 * time.Second
	// olricHealProbeParallel caps the member-list calls in flight when the
	// ring is partitioned. Nothing is asked of peers while it is whole, so a
	// healthy cluster costs one local call per check whatever its size.
	olricHealProbeParallel = 16
)

// olricHealInputs are what one check reads: the index raft configuration and
// one Olric's member names.
type olricHealInputs struct {
	nodes   func(ctx context.Context) (rqlite.RQLiteNodes, error)
	members func(ctx context.Context, addr string) ([]string, error)
}

// gatherHealViews returns the index Olric address of every node in the raft
// configuration and the member names of each Olric that answered. Peers are
// asked only when self sees fewer members than there are nodes; a peer the
// local raft does not see as reachable is not asked (its absence makes the
// decision wait).
func gatherHealViews(ctx context.Context, self string, in olricHealInputs) ([]string, map[string][]string, error) {
	nodes, err := in.nodes(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("read the index raft configuration: %w", err)
	}
	selfMembers, err := in.members(ctx, self)
	if err != nil {
		return nil, nil, fmt.Errorf("read this node's Olric members: %w", err)
	}
	expected := make([]string, 0, len(nodes))
	reachable := map[string]bool{}
	for _, nd := range nodes {
		host, _, err := net.SplitHostPort(nd.Addr)
		if err != nil {
			return nil, nil, fmt.Errorf("raft node %s has address %q: %w", nd.ID, nd.Addr, err)
		}
		addr := net.JoinHostPort(host, strconv.Itoa(namespace.IndexOlricHTTPPort))
		expected = append(expected, addr)
		reachable[addr] = nd.Reachable || addr == self
	}
	views := map[string][]string{self: selfMembers}
	if len(selfMembers) >= len(expected) {
		return expected, views, nil
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	slots := make(chan struct{}, olricHealProbeParallel)
	for _, addr := range expected {
		if addr == self || !reachable[addr] {
			continue
		}
		wg.Add(1)
		slots <- struct{}{}
		go func(addr string) {
			defer func() { <-slots; wg.Done() }()
			m, err := in.members(ctx, addr)
			if err != nil {
				return // an Olric that does not answer is a node the decision waits for
			}
			mu.Lock()
			views[addr] = m
			mu.Unlock()
		}(addr)
	}
	wg.Wait()
	return expected, views, nil
}

// startOlricHeal starts the healer loop once, from the supervisor's context.
func (n *Node) startOlricHeal(ctx context.Context) error {
	host, err := rqlite.BindHost(n.config.Discovery.HttpAdvAddress)
	if err != nil {
		return fmt.Errorf("olric heal: %w", err)
	}
	dataDir, err := config.ExpandPath(n.config.Node.DataDir)
	if err != nil {
		return fmt.Errorf("olric heal: %w", err)
	}
	self := net.JoinHostPort(host, strconv.Itoa(namespace.IndexOlricHTTPPort))
	n.olricHealOnce.Do(func() {
		go n.olricHealLoop(ctx, self, dataDir)
		n.logger.ComponentInfo(logging.ComponentNode, "Index Olric partition healer started", zap.String("olric", self))
	})
	return nil
}

func (n *Node) olricHealLoop(ctx context.Context, self, dataDir string) {
	in := olricHealInputs{
		nodes: func(ctx context.Context) (rqlite.RQLiteNodes, error) {
			admin, err := n.rqliteManager.LocalAdminClient()
			if err != nil {
				return nil, err
			}
			return admin.Nodes(ctx)
		},
		members: func(ctx context.Context, addr string) ([]string, error) {
			ms, err := olric.Members(ctx, addr, olricHealProbeTimeout)
			names := make([]string, len(ms))
			for i, m := range ms {
				names[i] = m.Name
			}
			return names, err
		},
	}
	var healer olric.Healer
	ticker := time.NewTicker(olric.HealCheckInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		expected, views, err := gatherHealViews(ctx, self, in)
		if err != nil {
			n.logger.ComponentDebug(logging.ComponentNode, "Index Olric membership check skipped", zap.Error(err))
			continue
		}
		d := olric.DecideHeal(self, expected, views)
		switch {
		case d.Action == olric.HealNone:
		case len(views) == len(expected):
			// Every node answers and the ring is still split: a partition.
			n.logger.ComponentWarn(logging.ComponentNode, "Index Olric is partitioned",
				zap.Int("members", len(views[self])), zap.Int("nodes", len(expected)), zap.String("decision", d.Reason))
		default:
			// A node does not answer: down, or the mesh has not healed.
			n.logger.ComponentDebug(logging.ComponentNode, "Index Olric is short of members",
				zap.Int("members", len(views[self])), zap.Int("nodes", len(expected)), zap.String("decision", d.Reason))
		}
		if !healer.Observe(d, time.Now()) {
			continue
		}
		n.restartOlricToHeal(self, dataDir, d.Reason)
	}
}

// restartOlricToHeal restarts this node's index Olric, which joins every peer
// again when it starts. Its share of the cache is lost.
func (n *Node) restartOlricToHeal(self, dataDir, reason string) {
	n.logger.ComponentWarn(logging.ComponentNode, "Restarting the index Olric to merge a partition; its share of the cache is lost",
		zap.String("olric", self), zap.String("reason", reason))
	sup, _, err := n.indexSupervisor()
	if err == nil {
		err = sup.RestartOlric()
	}
	if err != nil {
		n.logger.ComponentError(logging.ComponentNode, "Index Olric restart to merge a partition failed", zap.Error(err))
		return
	}
	if err := olric.RecordHealRestart(dataDir, reason, time.Now()); err != nil {
		n.logger.ComponentWarn(logging.ComponentNode, "Could not record the Olric heal restart for the node report", zap.Error(err))
	}
}
