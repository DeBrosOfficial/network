//go:build e2e_fleet

package cachechaos

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/Orama-Network/orama/e2e/features/internal/infra"
	"github.com/Orama-Network/orama/e2e/features/internal/realistic"
	"github.com/Orama-Network/orama/e2e/harness"
	"github.com/Orama-Network/orama/e2e/harness/eventually"
	"github.com/Orama-Network/orama/e2e/harness/fleet"
)

const (
	// partitionBudget is how long memberlist takes to declare the cut-off
	// peers dead (probe 1 s, suspicion a few seconds) with a margin.
	partitionBudget = 2 * time.Minute
	// partitionHold keeps the cut past memberlist's GossipToTheDeadTime (30 s),
	// after which the sides no longer refute each other and stay apart.
	partitionHold = 45 * time.Second
	// healBudget is two healer checks 30 s apart, the restart and the join,
	// with a margin.
	healBudget = 4 * time.Minute
)

// indexOlric is the olric section of `orama node report`.
type indexOlric struct {
	MemberCount  int `json:"member_count"`
	HealRestarts int `json:"heal_restarts"`
}

func readIndexOlric(t testing.TB, f *fleet.Fleet, n fleet.Node) (indexOlric, error) {
	t.Helper()
	out := infra.OnNode(t, f, n, "node", "report")
	var r struct {
		Olric *indexOlric `json:"olric"`
	}
	if out.Exit != 0 {
		return indexOlric{}, fmt.Errorf("orama node report exit %d: %s", out.Exit, f.Redact(out.Stderr))
	}
	if err := json.Unmarshal([]byte(out.Stdout), &r); err != nil || r.Olric == nil {
		return indexOlric{}, fmt.Errorf("node report has no olric section: %v", err)
	}
	return *r.Olric, nil
}

// TestOlricChaos_indexPartitionIsHealedByOneRestart (bug-933): one node is cut
// off from the others long enough that memberlist on both sides declares the
// other dead, then the cut is lifted. The ring does not merge by itself; the
// partition healer restarts the index Olric of exactly the node on the
// smaller side, and the ring is whole again on every node
// (docs/whitepaper/technical-reference/vol1/18-cache.md "Partition healing").
func TestOlricChaos_indexPartitionIsHealedByOneRestart(t *testing.T) {
	f := harness.Fleet(t)
	nodes := f.State.Nodes
	if len(nodes) < 3 {
		harness.SkipNotApplicable(t, "a partition with a majority side needs three nodes")
	}
	target := nodes[len(nodes)-1]
	before := map[string]int{}
	for _, n := range nodes {
		o, err := readIndexOlric(t, f, n)
		if err != nil {
			t.Fatal(err)
		}
		if o.MemberCount != len(nodes) {
			t.Fatalf("%s: index Olric has %d members before the test, want %d", n.Name, o.MemberCount, len(nodes))
		}
		before[n.Name] = o.HealRestarts
	}

	t.Run("cut", func(t *testing.T) {
		for _, n := range nodes[:len(nodes)-1] {
			realistic.DropFrom(t, f, target, n)
		}
		eventually.Require(t, 5*time.Second, partitionBudget, target.Name+"'s index Olric alone", func() (bool, error) {
			o, err := readIndexOlric(t, f, target)
			return err == nil && o.MemberCount == 1, err
		})
		time.Sleep(partitionHold)
	}) // the cut's rules are removed when the subtest ends

	for _, n := range nodes {
		eventually.Eventually(t, 10*time.Second, healBudget, n.Name+"'s index Olric whole again", func() (bool, error) {
			o, err := readIndexOlric(t, f, n)
			return err == nil && o.MemberCount == len(nodes), err
		})
	}
	for _, n := range nodes {
		o, err := readIndexOlric(t, f, n)
		if err != nil {
			t.Fatal(err)
		}
		want := before[n.Name]
		if n.Name == target.Name {
			want++
		}
		if o.HealRestarts != want {
			t.Errorf("%s: %d heal restarts, want %d: only the node on the smaller side restarts, once", n.Name, o.HealRestarts, want)
		}
	}
}
