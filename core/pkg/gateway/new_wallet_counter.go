package gateway

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Orama-Network/orama/core/pkg/olric"
	olriclib "github.com/olric-data/olric"
)

// newWalletDMap holds the counts behind a namespace's new-wallet ceiling
// (auth.Service.AdmitSignIn, bugboard #749): per (namespace, client network,
// hour) a count of distinct wallets, and per wallet the place it was given.
const newWalletDMap = "orama-signin-new-wallets"

// errNoCache is a count asked of a gateway whose Olric client is down.
var errNoCache = errors.New("no Olric client is connected")

// olricWindowCounter is auth.WindowCounter on the gateway's Olric: the index
// Olric on the index gateway, the namespace's own on a namespace gateway, so
// every gateway of one kind shares a namespace's counts. Each entry expires
// window after it is first written; the caller puts the window's start in the
// key, so an entry is never read past its window.
type olricWindowCounter struct {
	client func() olriclib.Client
}

// Position implements auth.WindowCounter: three operations for a member first
// seen (its place, the count, its place written), one for a member seen
// before. Two first sightings of one member racing may both be counted, which
// an abuse ceiling can afford.
func (c olricWindowCounter) Position(ctx context.Context, key, member string, window time.Duration) (int, error) {
	client := c.client()
	if client == nil {
		return 0, errNoCache
	}
	dm, err := client.NewDMap(newWalletDMap)
	if err != nil {
		return 0, fmt.Errorf("open DMap %s: %w", newWalletDMap, err)
	}
	memberKey := key + "|" + member
	gr, err := dm.Get(ctx, memberKey)
	switch {
	case err == nil:
		place, err := gr.Int()
		if err != nil {
			return 0, fmt.Errorf("read the place of a counted member: %w", err)
		}
		return place, nil
	case !olric.IsKeyNotFound(err):
		return 0, fmt.Errorf("read the place of a member: %w", err)
	}
	if err := dm.Put(ctx, key, 0, olriclib.NX(), olriclib.EX(window)); err != nil && !olric.IsKeyFound(err) {
		return 0, fmt.Errorf("start the count: %w", err)
	}
	place, err := dm.Incr(ctx, key, 1)
	if err != nil {
		return 0, fmt.Errorf("count the member: %w", err)
	}
	if err := dm.Put(ctx, memberKey, place, olriclib.EX(window)); err != nil {
		return 0, fmt.Errorf("record the place of the member: %w", err)
	}
	return place, nil
}
