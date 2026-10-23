package gateway

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Orama-Network/orama/core/pkg/olric/olrictest"
	olriclib "github.com/olric-data/olric"
)

// Against Olric itself: distinct members are counted in arrival order, a
// member counted before keeps its place, and keys do not share counts.
func TestOlricWindowCounter_countsDistinctMembers(t *testing.T) {
	client := olrictest.Start(t).EmbeddedClient()
	c := olricWindowCounter{client: func() olriclib.Client { return client }}
	ctx := context.Background()

	for i, member := range []string{"0xa", "0xb", "0xa", "0xc", "0xb"} {
		want := map[string]int{"0xa": 1, "0xb": 2, "0xc": 3}[member]
		got, err := c.Position(ctx, "ns|203.0.113.7|1", member, time.Hour)
		if err != nil || got != want {
			t.Fatalf("call %d (%s): place %d, %v; want %d", i+1, member, got, err, want)
		}
	}
	if got, err := c.Position(ctx, "ns|198.51.100.9|1", "0xa", time.Hour); err != nil || got != 1 {
		t.Errorf("another network's count: %d, %v; want 1", got, err)
	}
}

// The count and the places expire with the window they were written for.
func TestOlricWindowCounter_expiresWithTheWindow(t *testing.T) {
	client := olrictest.Start(t).EmbeddedClient()
	c := olricWindowCounter{client: func() olriclib.Client { return client }}
	ctx := context.Background()
	const window = 200 * time.Millisecond

	if got, err := c.Position(ctx, "k", "0xa", window); err != nil || got != 1 {
		t.Fatalf("first: %d, %v", got, err)
	}
	time.Sleep(2 * window)
	if got, err := c.Position(ctx, "k", "0xb", window); err != nil || got != 1 {
		t.Errorf("after the window: %d, %v; want a fresh count", got, err)
	}
}

func TestOlricWindowCounter_noClient(t *testing.T) {
	c := olricWindowCounter{client: func() olriclib.Client { return nil }}
	if _, err := c.Position(context.Background(), "k", "0xa", time.Hour); !errors.Is(err, errNoCache) {
		t.Errorf("no client: %v", err)
	}
}
