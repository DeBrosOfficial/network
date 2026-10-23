package config

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Targets of the runner. The default target is a disposable fleet the run
// provisions; TargetTestnet is the existing testnet cluster, which the
// runner only tests: it never provisions, destroys or sweeps anything there.
const (
	TargetFleet   = ""
	TargetTestnet = "testnet"
)

// The testnet target is pinned to exactly these values: a state that names
// anything else is refused, so a state file cannot aim the suite at devnet
// or any other cluster.
const (
	TestnetEnv        = "testnet"
	TestnetBaseDomain = "testnet.orama.network"
	TestnetGatewayURL = "https://testnet.orama.network"
	// TestnetChainHost is where a node's co-located chain answers (RPC 31001, REST 31003, indexer 31015):
	// inside the orama-global netns, reachable from the host's root namespace.
	TestnetChainHost = "198.18.0.2"
	// TestnetOperatorNamespace is the namespace the operator's wallet owns
	// and signs in to for a run. An operator route needs only a wallet
	// session on the operator list (docs/whitepaper/technical-reference/vol1/14-authorization.md#operators),
	// so a lobby session would do; the run's commands that act on the current
	// namespace need one it owns.
	TestnetOperatorNamespace = "testnetproof"
)

// Paths under the owner's real home that the testnet target uses.
const (
	// TestnetRWSockRel is the socket of the dev RootWallet headless agent.
	TestnetRWSockRel = "rwdev/agent.sock"
	// TestnetRWReadyRel is the ready file that agent wrote; it holds the wallet address.
	TestnetRWReadyRel = "rwdev/ready.json"
	// TestnetHomeRel is the CLI HOME (it holds ~/.orama/environments.json for the testnet env).
	TestnetHomeRel = "orama-testnet-handoff/cli-home"
	// TestnetCAFileRel is the Let's Encrypt production roots bundle (ISRG
	// Root X1 and X2): testnet serves production certificates, and the run
	// pins the roots they chain to rather than the system store.
	TestnetCAFileRel = "orama-testnet-handoff/le-roots.pem"
	// TestnetSSHKeyRel is the private key that reaches the testnet nodes.
	TestnetSSHKeyRel = ".ssh/debros-nodes"
	// TestnetKnownHostsRel is the owner's known_hosts the pinned host keys are cross-checked against.
	TestnetKnownHostsRel = ".ssh/known_hosts"
)

// TestnetNode is one testnet node. Nameserver says whether it runs the
// zone's CoreDNS; the others are plain cluster nodes.
type TestnetNode struct {
	Name, Label, IP, User, WGIP string
	Nameserver                  bool
}

// TestnetNodes are the five testnet nodes, in state order (the join order;
// node-1 is the genesis nameserver). WGIP is the overlay address the join
// assigned. The machines differ in ways the tests see:
//
//   - mew and mewtwo: OVH (ASN 16276), Ubuntu 26.04, systemd 259 built with
//     BPF_FRAMEWORK, so SocketBindDeny is enforced and the deployment sandbox
//     test sees a refused bind there.
//   - gengar, magicarp and froakie: Contabo (ASN 51167), Ubuntu 24.04, systemd
//     255 without BPF_FRAMEWORK, so SocketBindDeny is accepted and not
//     enforced and the sandbox test's outcome on these three is the
//     unenforced one.
//
// The first three are nameservers (mew, mewtwo, gengar); magicarp and froakie
// are plain nodes. Gengar, magicarp and froakie log in as root, the others as
// ubuntu.
var TestnetNodes = []TestnetNode{
	{Name: "node-1", Label: "mew", IP: "57.129.166.16", User: "ubuntu", WGIP: "10.0.0.1", Nameserver: true},
	{Name: "node-2", Label: "mewtwo", IP: "57.129.166.17", User: "ubuntu", WGIP: "10.0.0.2", Nameserver: true},
	{Name: "node-3", Label: "gengar", IP: "161.97.184.199", User: "root", WGIP: "10.0.0.3", Nameserver: true},
	{Name: "node-4", Label: "magicarp", IP: "161.97.184.202", User: "root", WGIP: "10.0.0.4"},
	{Name: "node-5", Label: "froakie", IP: "161.97.151.255", User: "root", WGIP: "10.0.0.5"},
}

var (
	testnetChainID = regexp.MustCompile(`^orama-testnet-[0-9]+$`)
	// testnetRunID is testnet-<yyyymmdd>-<hhmmss>.
	testnetRunID = regexp.MustCompile(`^testnet-[0-9]{8}-[0-9]{6}$`)
)

// TestnetRunID reports whether id has the shape of a testnet run id.
func TestnetRunID(id string) bool { return testnetRunID.MatchString(id) }

// TestnetPath is rel under realHome.
func TestnetPath(realHome, rel string) string { return filepath.Join(realHome, rel) }

// TestnetIPs are the pinned public addresses, sorted.
func TestnetIPs() []string {
	out := make([]string, 0, len(TestnetNodes))
	for _, n := range TestnetNodes {
		out = append(out, n.IP)
	}
	sort.Strings(out)
	return out
}

// CheckTarget refuses a target name that is neither the default nor testnet.
func CheckTarget(target string) error {
	if target != TargetFleet && target != TargetTestnet {
		return fmt.Errorf("target %q is unknown (use %q or the default fleet target)", target, TargetTestnet)
	}
	return nil
}

// TestnetPins are the values of a state the testnet target pins exactly.
type TestnetPins struct {
	RunID, Env, BaseDomain, GatewayURL, ChainID string
	// OperatorNamespace is the namespace the operator signs in to. A state
	// written before it was recorded holds none, and signing in to "" is the
	// lobby, where no operator route answers: every stage then failed.
	OperatorNamespace string
	// NodeIPs are the public addresses of every node, extra and probe of the state.
	NodeIPs []string
}

// CheckTestnet requires every pinned value to match exactly. Every mismatch is reported.
func CheckTestnet(p TestnetPins) error {
	var errs []error
	fail := func(format string, args ...any) { errs = append(errs, fmt.Errorf("testnet target: "+format, args...)) }
	if !testnetRunID.MatchString(p.RunID) {
		fail("run id %q is not testnet-<yyyymmdd>-<hhmmss>", p.RunID)
	}
	if p.Env != TestnetEnv {
		fail("environment %q is not %q", p.Env, TestnetEnv)
	}
	if p.BaseDomain != TestnetBaseDomain {
		fail("base domain %q is not %q", p.BaseDomain, TestnetBaseDomain)
	}
	if p.GatewayURL != TestnetGatewayURL {
		fail("gateway url %q is not %q", p.GatewayURL, TestnetGatewayURL)
	}
	if !testnetChainID.MatchString(p.ChainID) {
		fail("chain id %q does not match %s", p.ChainID, testnetChainID)
	}
	if p.OperatorNamespace != TestnetOperatorNamespace {
		fail("operator namespace %q is not %q (re-run `e2e-fleet target testnet`)", p.OperatorNamespace, TestnetOperatorNamespace)
	}
	got := append([]string{}, p.NodeIPs...)
	sort.Strings(got)
	if want := TestnetIPs(); strings.Join(got, ",") != strings.Join(want, ",") {
		fail("node public addresses [%s] are not exactly [%s]", strings.Join(got, ", "), strings.Join(want, ", "))
	}
	return errors.Join(errs...)
}

// CheckTestnetChainID requires id to be a testnet chain id.
func CheckTestnetChainID(id string) error {
	if !testnetChainID.MatchString(id) {
		return fmt.Errorf("chain id %q does not match %s", id, testnetChainID)
	}
	return nil
}
