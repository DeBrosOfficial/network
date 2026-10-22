package display

import (
	"strings"
	"testing"

	"github.com/Orama-Network/orama/core/cmd/orama/internal/monitor/view"
	"github.com/Orama-Network/orama/core/pkg/constants"
	"github.com/Orama-Network/orama/core/pkg/telemetry/report"
)

func TestGlobalLine_indexerShowsItsPlaceOrWhyItIsStuck(t *testing.T) {
	th := view.NewTheme(false)
	g := &report.GlobalReport{
		Units:   []report.GlobalUnit{{Name: constants.GlobalIndexerUnit, State: "active"}},
		Indexer: &report.IndexerReport{Cursor: 40, Tip: 900},
	}
	if got := globalLine(th, g); got != "indexer active (block 40 of 900)" {
		t.Fatalf("got %q", got)
	}
	g.Indexer.Stuck = &report.IndexerStuck{Height: 41, Reason: "the node pruned the state", StatePruned: true}
	got := globalLine(th, g)
	if !strings.Contains(got, "STUCK at block 41: the node pruned the state") {
		t.Fatalf("a stuck indexer printed %q", got)
	}
	g.Indexer = &report.IndexerReport{Error: "connection refused"}
	if got := globalLine(th, g); got != "indexer active" {
		t.Fatalf("an indexer that did not answer printed %q", got)
	}
}
