package report

import (
	"testing"
	"time"

	"github.com/Orama-Network/orama/core/pkg/olric"
)

// bug-933: the restarts the partition healer made reach the node report.
func TestCollectOlricHeal(t *testing.T) {
	dir := t.TempDir()
	var r OlricReport
	collectOlricHeal(&r, dir)
	if r.HealRestarts != 0 || r.LastHealRestart != nil {
		t.Fatalf("no record: %+v", r)
	}
	at := time.Unix(1_700_000_000, 0)
	if err := olric.RecordHealRestart(dir, "2 sides", at); err != nil {
		t.Fatal(err)
	}
	collectOlricHeal(&r, dir)
	if r.HealRestarts != 1 || r.LastHealRestart == nil || !r.LastHealRestart.Equal(at) || r.LastHealReason != "2 sides" {
		t.Fatalf("one restart recorded: %+v", r)
	}
}
