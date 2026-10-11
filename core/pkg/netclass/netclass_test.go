package netclass

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestIsProduction_classifiesByMarker(t *testing.T) {
	cases := map[string]bool{
		"orama-stagenet-6": false, "orama-devnet-1": false, "orama-localnet-3": false,
		"orama-1": true, "orama-mainnet": true, "stagenet": true, "orama-stagenet": true, "": true,
		"orama-pubnet-stagenet-1": false, "orama-x-devnet-1": false, "orama-devnet-e2e-ab12cd": false,
		"orama-1-stagenet-x": true, "orama-1-devnet-x": true, "mainnet-localnet-1": true, "orama-stagenet-": true,
		"orama-a-b-stagenet-1": true,
	}
	for id, want := range cases {
		if got := IsProduction(id); got != want {
			t.Errorf("IsProduction(%q) = %v, want %v", id, got, want)
		}
	}
}

func TestIsLocalnet(t *testing.T) {
	for id, want := range map[string]bool{
		"orama-localnet-1": true, "orama-x-localnet-1": true,
		"orama-stagenet-1": false, "orama-1-localnet-x": false, "": false,
	} {
		if got := IsLocalnet(id); got != want {
			t.Errorf("IsLocalnet(%q) = %v, want %v", id, got, want)
		}
	}
}

func TestValidChainID(t *testing.T) {
	for id, want := range map[string]bool{
		"orama-stagenet-5": true, "a": true, "orama-1": true, strings.Repeat("a", MaxChainIDLen): true,
		"": false, strings.Repeat("a", MaxChainIDLen+1): false, "a b": false, "a\x1b": false, "é": false,
		"Orama-1": false, "orama_1": false, "orama.1": false, "-orama": false, "orama-": false, "orama-1\n": false,
	} {
		if got := ValidChainID(id); got != want {
			t.Errorf("ValidChainID(%q) = %v, want %v", id, got, want)
		}
	}
}

func TestIsTestNetwork_isTheOppositeOfProduction(t *testing.T) {
	for _, id := range []string{"orama-stagenet-6", "orama-devnet-1", "orama-localnet-2", "orama-1", "orama-testnet-3", "orama-stagenet", ""} {
		if IsTestNetwork(id) == IsProduction(id) {
			t.Errorf("%q is both or neither", id)
		}
	}
}

func TestCheckCommittee_productionFloor(t *testing.T) {
	if err := CheckCommittee("orama-1", ProductionMinCommittee-1); err == nil {
		t.Fatal("a production chain id with 29 validators was accepted")
	} else if !strings.Contains(err.Error(), "stagenet") || !strings.Contains(err.Error(), "30") {
		t.Errorf("the refusal does not say what to do instead: %v", err)
	}
	if err := CheckCommittee("orama-1", ProductionMinCommittee); err != nil {
		t.Errorf("a production chain id with 30 validators was refused: %v", err)
	}
}

func TestCheckCommittee_testNetworkAnySize(t *testing.T) {
	for _, n := range []int{1, 5, 29} {
		if err := CheckCommittee("orama-stagenet-1", n); err != nil {
			t.Errorf("a stagenet with %d validators was refused: %v", n, err)
		}
	}
}

func TestCheckCommittee_empty(t *testing.T) {
	if err := CheckCommittee("orama-stagenet-1", 0); err == nil {
		t.Fatal("an empty committee was accepted")
	}
}

// chainFile reads a file of the chain module, which sits beside core/.
func chainFile(t *testing.T, rel string) string {
	t.Helper()
	data, err := os.ReadFile("../../../chain/" + rel)
	if err != nil {
		t.Skipf("the chain sources are not reachable from here: %v", err)
	}
	return string(data)
}

func TestNetclass_matchesChain(t *testing.T) {
	pattern := regexp.MustCompile("testChainIDPattern = regexp.MustCompile\\(`([^`]*)`\\)")
	m := pattern.FindStringSubmatch(chainFile(t, "app/params/chainid.go"))
	if m == nil {
		t.Fatal("chain/app/params/chainid.go no longer declares testChainIDPattern: update pkg/netclass")
	}
	if m[1] != testChainIDPattern.String() {
		t.Errorf("the chain's test chain id pattern is %s, pkg/netclass has %s", m[1], testChainIDPattern)
	}
	floor := regexp.MustCompile(`ProductionMinCommitteeSize uint64 = (\d+)`).FindStringSubmatch(chainFile(t, "x/power/types/params.go"))
	if floor == nil || floor[1] != strconv.Itoa(ProductionMinCommittee) {
		t.Errorf("the chain's production committee floor is %v, pkg/netclass has %d", floor, ProductionMinCommittee)
	}
}
