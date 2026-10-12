package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/DeBrosOfficial/network/chain/x/shielded/pool"
	"github.com/DeBrosOfficial/network/chain/x/shielded/types"
)

// EventTypeQueuePaymentFailed is emitted when a window could not deliver a queued request. The
// request stays in the queue.
const EventTypeQueuePaymentFailed = "shielded_queue_payment_failed"

func (k Keeper) queueHasEntries(ctx context.Context) (bool, error) {
	it, err := k.Queue.Iterate(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("read the unshield queue: %w", err)
	}
	defer it.Close()
	return it.Valid(), nil
}

func (k Keeper) enqueue(ctx sdk.Context, msg *types.MsgUnshield, owner sdk.AccAddress, amount math.Int) error {
	id, err := k.NextQueueID.Next(ctx)
	if err != nil {
		return fmt.Errorf("next queue id: %w", err)
	}
	key := nativePool()
	return k.Queue.Set(ctx, id, types.QueuedUnshield{
		Id: id, Vintage: key.K1(), Asset: key.K2(), Owner: owner.String(), Target: msg.Target,
		Validator: msg.Validator, NodeId: msg.NodeId, Role: msg.Role, Amount: amount, Height: ctx.BlockHeight(),
	})
}

// MaxServedPerWindow bounds the payments one window makes, so the end blocker's work does not grow
// with the queue. The rest wait for the next window, oldest first.
const MaxServedPerWindow = 1000

// queued returns every queued request in id order, which is arrival order (genesis export and
// queries).
func (k Keeper) queued(ctx context.Context) ([]types.QueuedUnshield, error) {
	return k.queuedUpTo(ctx, 0)
}

// queuedUpTo returns the first limit queued requests in id order; limit 0 means all.
func (k Keeper) queuedUpTo(ctx context.Context, limit int) ([]types.QueuedUnshield, error) {
	var out []types.QueuedUnshield
	err := k.Queue.Walk(ctx, nil, func(_ uint64, q types.QueuedUnshield) (bool, error) {
		out = append(out, q)
		return limit > 0 && len(out) >= limit, nil
	})
	return out, err
}

// serveQueue pays the queue at most once per 24h. The capacity is what the cap leaves of the
// drained bucket, served oldest first and capped per address, so no address takes more than its
// cap and a bond is never paid in a part the delegation minimum would refuse. A request whose
// target refuses the payment stays queued and an event says so; the other requests are still paid.
func (k Keeper) serveQueue(ctx sdk.Context) error {
	waiting, err := k.queueHasEntries(ctx)
	if err != nil || !waiting {
		return err
	}
	key := nativePool()
	lim, stored, err := k.limiter(ctx, key)
	if err != nil {
		return err
	}
	now := ctx.BlockTime().Unix()
	if stored.ServedWindowStart != 0 && now < stored.ServedWindowStart+pool.WindowSeconds {
		return nil
	}
	p, err := k.params(ctx)
	if err != nil {
		return err
	}
	balance, err := k.poolBalance(ctx, key)
	if err != nil {
		return err
	}
	limit := pool.CapAmount(balance, p.UnshieldFloor)
	lim.Roll(ctx.BlockTime(), limit)
	reqs, err := k.queuedUpTo(ctx, MaxServedPerWindow)
	if err != nil {
		return err
	}
	poolReqs := make([]pool.Request, len(reqs))
	for i, q := range reqs {
		poolReqs[i] = pool.Request{Address: q.Owner, Amount: q.Amount}
	}
	grants := pool.Serve(lim.Remaining(limit), p.QueuePerAddressCap, poolReqs, func(i int, grant math.Int) bool {
		return k.partPaymentFits(ctx, reqs[i], grant)
	})
	for i, q := range reqs {
		grant := grants[i].Amount
		if grant.IsPositive() && k.payQueued(ctx, q, grant) {
			lim.Counted = lim.Counted.Add(grant)
		}
	}
	stored.ServedWindowStart = now
	return k.saveLimiter(ctx, key, lim, stored)
}

// partPaymentFits reports whether paying only grant of a queued bond would be accepted by its
// target. A part payment below the delegation minimum would be refused when paid, and issuing it
// would spend the window's capacity on nothing. The minimum of a node bond is checked by x/nodes
// only when the bond is made.
func (k Keeper) partPaymentFits(ctx sdk.Context, q types.QueuedUnshield, grant math.Int) bool {
	if q.Target != types.UnshieldTargetBond {
		return true
	}
	owner, err := sdk.AccAddressFromBech32(q.Owner)
	if err != nil {
		return true
	}
	return k.deps.Bonder.CheckMinimum(ctx, owner, q.Validator, grant) == nil
}

// payQueued pays grant of one queued request in a cache context and reports whether it went out.
func (k Keeper) payQueued(ctx sdk.Context, q types.QueuedUnshield, grant math.Int) bool {
	cache, write := ctx.CacheContext()
	owner, err := sdk.AccAddressFromBech32(q.Owner)
	if err == nil {
		err = k.pay(cache, owner, q.Target, q.Validator, q.NodeId, q.Role, grant)
	}
	if err == nil {
		q.Amount = q.Amount.Sub(grant)
		if q.Amount.IsPositive() {
			err = k.Queue.Set(cache, q.Id, q)
		} else {
			err = k.Queue.Remove(cache, q.Id)
		}
	}
	if err != nil {
		ctx.EventManager().EmitEvent(sdk.NewEvent(EventTypeQueuePaymentFailed,
			sdk.NewAttribute("id", fmt.Sprint(q.Id)),
			sdk.NewAttribute("owner", q.Owner),
			sdk.NewAttribute("reason", err.Error()),
		))
		return false
	}
	write()
	return true
}
