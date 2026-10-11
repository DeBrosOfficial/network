package types_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"cosmossdk.io/math"

	"github.com/DeBrosOfficial/network/chain/x/shielded/types"
)

func TestParamsValidate_defaultsAreValid(t *testing.T) {
	require.NoError(t, types.DefaultParams().Validate())
}

func TestParamsValidate_namesTheFirstBadAmountEveryTime(t *testing.T) {
	p := types.DefaultParams()
	p.NullifierFee = math.ZeroInt()
	p.UnshieldFloor = math.ZeroInt()
	p.MaxFeeTopup = math.ZeroInt()
	p.QueuePerAddressCap = math.ZeroInt()
	// The check used to range over a map, so which field the error named changed from run to run
	// and from node to node. Four keys come out in 24 orders; fifty runs that all agree by chance
	// have a probability of 24^-49.
	for range 50 {
		require.EqualError(t, p.Validate(), "nullifier_fee must be a positive integer")
	}
}

func TestParamsValidate_nilAndNonPositiveAmountsAreRefused(t *testing.T) {
	cases := map[string]func(*types.Params){
		"nil nullifier fee":     func(p *types.Params) { p.NullifierFee = math.Int{} },
		"negative unshield":     func(p *types.Params) { p.UnshieldFloor = math.NewInt(-1) },
		"zero fee top-up":       func(p *types.Params) { p.MaxFeeTopup = math.ZeroInt() },
		"nil queue per address": func(p *types.Params) { p.QueuePerAddressCap = math.Int{} },
	}
	for name, mutate := range cases {
		p := types.DefaultParams()
		mutate(&p)
		require.Error(t, p.Validate(), name)
	}
}

func TestParamsValidate_zeroCountIsRefused(t *testing.T) {
	p := types.DefaultParams()
	p.MaxActionsPerBundle = 0
	require.ErrorContains(t, p.Validate(), "max_actions_per_bundle")
}
