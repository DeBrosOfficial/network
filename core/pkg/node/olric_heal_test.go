package node

import (
	"context"
	"errors"
	"testing"

	"github.com/Orama-Network/orama/core/pkg/olric"
	"github.com/Orama-Network/orama/core/pkg/rqlite"
)

func healInputs(nodes rqlite.RQLiteNodes, members map[string][]string, asked *[]string) olricHealInputs {
	return olricHealInputs{
		nodes: func(context.Context) (rqlite.RQLiteNodes, error) { return nodes, nil },
		members: func(_ context.Context, addr string) ([]string, error) {
			if asked != nil {
				*asked = append(*asked, addr)
			}
			m, ok := members[addr]
			if !ok {
				return nil, errors.New("connection refused")
			}
			return m, nil
		},
	}
}

var threeRaft = rqlite.RQLiteNodes{
	{ID: "a", Addr: "10.0.0.1:7002", Reachable: true},
	{ID: "b", Addr: "10.0.0.2:7002", Reachable: true},
	{ID: "c", Addr: "10.0.0.3:7002", Reachable: true},
}

// A whole ring costs one local call: no peer is asked.
func TestGatherHealViews_aWholeRingAsksNoPeer(t *testing.T) {
	all := []string{"10.0.0.1:10102", "10.0.0.2:10102", "10.0.0.3:10102"}
	var asked []string
	expected, views, err := gatherHealViews(context.Background(), all[0],
		healInputs(threeRaft, map[string][]string{all[0]: all}, &asked))
	if err != nil {
		t.Fatal(err)
	}
	if len(expected) != 3 || len(asked) != 1 || olric.DecideHeal(all[0], expected, views).Action != olric.HealNone {
		t.Fatalf("expected %v, asked %v", expected, asked)
	}
}

// Partitioned, every peer answering: the singleton that sorts first restarts.
func TestGatherHealViews_aPartitionIsDecidedFromEveryView(t *testing.T) {
	a, b, c := "10.0.0.1:10102", "10.0.0.2:10102", "10.0.0.3:10102"
	members := map[string][]string{a: {a, b}, b: {a, b}, c: {c}}
	expected, views, err := gatherHealViews(context.Background(), c, healInputs(threeRaft, members, nil))
	if err != nil {
		t.Fatal(err)
	}
	if d := olric.DecideHeal(c, expected, views); d.Action != olric.HealRestart {
		t.Fatalf("%v (%s), want restart", d.Action, d.Reason)
	}
}

// A peer raft does not see is not asked, and the decision waits for it.
func TestGatherHealViews_aPeerRaftCannotReachIsNotAskedAndMakesItWait(t *testing.T) {
	a, b, c := "10.0.0.1:10102", "10.0.0.2:10102", "10.0.0.3:10102"
	nodes := append(rqlite.RQLiteNodes{}, threeRaft...)
	nodes[2].Reachable = false
	var asked []string
	expected, views, err := gatherHealViews(context.Background(), a,
		healInputs(nodes, map[string][]string{a: {a}, b: {b}, c: {c}}, &asked))
	if err != nil {
		t.Fatal(err)
	}
	for _, x := range asked {
		if x == c {
			t.Fatal("asked the Olric of a node raft cannot reach")
		}
	}
	if d := olric.DecideHeal(a, expected, views); d.Action != olric.HealWait {
		t.Fatalf("%v (%s), want wait", d.Action, d.Reason)
	}
}
