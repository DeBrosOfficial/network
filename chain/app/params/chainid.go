package params

import "regexp"

// ChainClass is the kind of network a chain id names. Every gate that treats a test network
// differently from a production one (the faucet, the bootstrap stake, the short committee, the
// relaxed genesis parameters, the optional shielded verifier) asks ClassOfChainID, so there is
// one rule and it is exact.
type ChainClass int

const (
	// ChainProduction is every chain id that is not one of the test classes. It is the default
	// and the safe answer: an id that is not recognised exactly is production.
	ChainProduction ChainClass = iota
	// ChainLocalnet is a throwaway chain on one machine; nothing is locked.
	ChainLocalnet
	// ChainDevnet is a chain a script builds for one e2e run.
	ChainDevnet
	// ChainStagenet is the shared pre-production network.
	ChainStagenet
)

// testChainIDPattern is the whole shape of a test chain id: "orama-", optionally one name word that
// starts with a letter (a test network of one's own: "orama-pubnet-stagenet-1"), the class, "-",
// then one or more lowercase alphanumeric words joined by hyphens ("orama-stagenet-10",
// "orama-devnet-e2e-run7", "orama-localnet-bench-1"). The match is anchored at both ends, so a
// class word anywhere else in an id ("orama-1-stagenet-x", "mainnet-localnet-1", "orama-stagenet-")
// names nothing. core/pkg/netclass holds the same pattern for the CLI, and its test reads this
// line to keep the two equal.
var testChainIDPattern = regexp.MustCompile(`^orama-(?:[a-z][a-z0-9]*-)?(localnet|devnet|stagenet)-[a-z0-9]+(?:-[a-z0-9]+)*$`)

// ClassOfChainID returns the class chainID names, or ChainProduction when it is not exactly a
// localnet, devnet or stagenet id.
func ClassOfChainID(chainID string) ChainClass {
	m := testChainIDPattern.FindStringSubmatch(chainID)
	if m == nil {
		return ChainProduction
	}
	switch m[1] {
	case "localnet":
		return ChainLocalnet
	case "devnet":
		return ChainDevnet
	default:
		return ChainStagenet
	}
}

// IsNonProductionChainID reports whether chainID is exactly a localnet, devnet or stagenet id.
func IsNonProductionChainID(chainID string) bool {
	return ClassOfChainID(chainID) != ChainProduction
}
