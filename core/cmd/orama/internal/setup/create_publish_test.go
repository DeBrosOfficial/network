package setup

import (
	"strings"
	"testing"

	"github.com/Orama-Network/orama/core/pkg/netregistry"
)

func createdTestnet(seeds ...string) *CreatedNetwork {
	return &CreatedNetwork{
		Manifest: &netregistry.Manifest{Name: "testnet", ChainID: "orama-testnet-6", Seeds: seeds},
		Dir:      "networks/testnet", Machines: []string{"203.0.113.10", "203.0.113.11"},
	}
}

func TestCreatedNetwork_seedRecordsPairSeedsWithMachines(t *testing.T) {
	got := createdTestnet("ns1.testnet.orama.network", "ns2.testnet.orama.network").SeedRecords()
	want := []string{"ns1.testnet.orama.network.\tIN\tA\t203.0.113.10", "ns2.testnet.orama.network.\tIN\tA\t203.0.113.11"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("records = %q, want %q", got, want)
	}
}

func TestCreatedNetwork_seedRecordsWhenSeedsAndMachinesDiffer(t *testing.T) {
	got := createdTestnet("seed.example.org").SeedRecords()
	if len(got) != 1 || !strings.Contains(got[0], "seed.example.org must resolve to one of [203.0.113.10 203.0.113.11]") {
		t.Errorf("records = %q", got)
	}
}

func TestCreatedNetwork_nextStepsNameTheCommandsInOrder(t *testing.T) {
	steps := strings.Join(createdTestnet("ns1.testnet.orama.network", "ns2.testnet.orama.network").NextSteps(), "\n")
	order := []string{
		"networks/testnet/", "1. Create these DNS records", "ns1.testnet.orama.network.\tIN\tA\t203.0.113.10",
		"2. In the repository", "make -C core sync-networks", "core/pkg/netregistry/embedded/", "3. Deploy the website",
		"orama network add https://orama.network/networks/testnet/manifest.json", "orama setup --network testnet", "new chain id",
	}
	last := -1
	for _, want := range order {
		i := strings.Index(steps, want)
		if i < 0 || i < last {
			t.Fatalf("%q is missing or out of order in:\n%s", want, steps)
		}
		last = i
	}
}

func TestCreatedNetwork_manifestURL(t *testing.T) {
	if got := createdTestnet("a.b").ManifestURL(); got != "https://orama.network/networks/testnet/manifest.json" {
		t.Errorf("URL = %s", got)
	}
}

// A network created at a domain has nameservers for seeds: the maintainer delegates the domain, and
// no A record is made by hand.
func TestCreatedNetwork_nextStepsDelegateTheDomainWhenSeedsAreNameservers(t *testing.T) {
	c := createdTestnet("ns1.testnet.orama.network", "ns2.testnet.orama.network")
	c.Domain = "testnet.orama.network"
	steps := strings.Join(c.NextSteps(), "\n")
	for _, want := range []string{"1. Delegate testnet.orama.network to the cluster", "ns1.testnet.orama.network ns2.testnet.orama.network", "2. In the repository"} {
		if !strings.Contains(steps, want) {
			t.Errorf("steps lack %q:\n%s", want, steps)
		}
	}
	if strings.Contains(steps, "Create these DNS records") || strings.Contains(steps, "\tIN\tA\t") {
		t.Errorf("a domain cluster is asked for A records:\n%s", steps)
	}
}
