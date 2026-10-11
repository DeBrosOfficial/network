package app_test

import (
	"errors"

	"cosmossdk.io/collections"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/DeBrosOfficial/network/chain/app"
	"github.com/DeBrosOfficial/network/chain/app/params"
	emissiontypes "github.com/DeBrosOfficial/network/chain/x/emission/types"
)

// mintNoramaForTest creates coins from x/emission's module account the way a test funds an account,
// and books the norama among them as supply that existed before the schedule, which is what x/emission
// does with a genesis supply. The chain refuses a block whose norama supply is above what x/emission
// recorded (conservation.go), so a test that mints through the bank keeper alone halts at its next
// block, exactly as a production mint outside x/emission would.
func mintNoramaForTest(a *app.OramaApp, ctx sdk.Context, coins sdk.Coins) error {
	if err := a.BankKeeper.MintCoins(ctx, emissiontypes.ModuleName, coins); err != nil {
		return err
	}
	norama := coins.AmountOf(params.BaseDenom)
	if norama.IsZero() {
		return nil
	}
	state, err := a.EmissionKeeper.EpochState.Get(ctx)
	if errors.Is(err, collections.ErrNotFound) {
		return nil // an app that never ran InitChain has no ledger and no blocks to halt
	}
	if err != nil {
		return err
	}
	state.GenesisSupply = state.GenesisSupply.Add(norama)
	return a.EmissionKeeper.EpochState.Set(ctx, state)
}
