package params_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Orama-Network/orama/chain/app/params"
)

func TestClassOfChainID(t *testing.T) {
	cases := map[string]params.ChainClass{
		// The ids the repository really uses.
		"orama-testnet-10":                 params.ChainTestnet,
		"orama-testnet-1":                  params.ChainTestnet,
		"orama-devnet-9":                   params.ChainDevnet,
		"orama-devnet-e2e-run7":            params.ChainDevnet,
		"orama-localnet-1":                 params.ChainLocalnet,
		"orama-localnet-bench-1":           params.ChainLocalnet,
		"orama-localnet-keepertest":        params.ChainLocalnet,
		"orama-localnet-shielded-wallet-1": params.ChainLocalnet,
		// A test network of one's own: one name word that starts with a letter.
		"orama-pubnet-testnet-1": params.ChainTestnet,
		"orama-x-devnet-1":       params.ChainDevnet,
		"orama-x-localnet-1":     params.ChainLocalnet,
		"orama-1x-testnet-1":     params.ChainProduction,
		// A class word is never the name word: these are ambiguous, so they are production.
		"orama-devnet-localnet-1":  params.ChainProduction,
		"orama-localnet-devnet-1":  params.ChainProduction,
		"orama-testnet-testnet-1":  params.ChainProduction,
		"orama-devnet-devnet-1":    params.ChainProduction,
		"orama-pubnet-localnet-1":  params.ChainLocalnet,
		"orama-localnet-devnet2-1": params.ChainLocalnet,
		"orama-devnet-localnetx-1": params.ChainDevnet,
		"orama-a-b-testnet-1":      params.ChainProduction,
		// Production, and ids that only look like a test chain.
		"orama-1":         params.ChainProduction,
		"orama-mainnet-1": params.ChainProduction,
		// stagenet was the test class's name before testnet; it is no class word now.
		"orama-stagenet-11":    params.ChainProduction,
		"orama-x-stagenet-1":   params.ChainProduction,
		"orama-1-testnet-x":    params.ChainProduction,
		"orama-1-devnet-x":     params.ChainProduction,
		"orama-1-localnet-x":   params.ChainProduction,
		"mainnet-testnet-1":    params.ChainProduction,
		"x-orama-testnet-1":    params.ChainProduction,
		"orama-testnet-1-":     params.ChainProduction,
		"orama-testnet-":       params.ChainProduction,
		"orama-testnet":        params.ChainProduction,
		"-testnet-":            params.ChainProduction,
		"-localnet-":           params.ChainProduction,
		"orama-testnet-1\n":    params.ChainProduction,
		"orama-testnet-1 ":     params.ChainProduction,
		"orama-testnet--1":     params.ChainProduction,
		"orama-Testnet-1":      params.ChainProduction,
		"orama-testnet-1/../x": params.ChainProduction,
		"":                     params.ChainProduction,
	}
	for id, want := range cases {
		require.Equal(t, want, params.ClassOfChainID(id), "chain id %q", id)
		require.Equal(t, want != params.ChainProduction, params.IsNonProductionChainID(id), "chain id %q", id)
	}
}
