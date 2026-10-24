//go:build e2e_fleet

package clienvauthmisc

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestNetworkAdd_manifestURLsThatCannotBeTrustedAreRefused: a manifest names
// the chain and the release root a node installs from, so only an https URL
// that ends in /manifest.json is fetched, and nothing is stored when it is not
// (docs/whitepaper/technical-reference/appendices/d-cli-reference.md#orama-network-add).
func TestNetworkAdd_manifestURLsThatCannotBeTrustedAreRefused(t *testing.T) {
	t.Parallel()
	cli := isolated(t)
	before := readEnvFile(t, cli)
	for label, tc := range map[string]struct {
		url  string
		want string
	}{
		"plain http":        {"http://example.invalid/nets/x/manifest.json", "https"},
		"not a manifest":    {"https://example.invalid/nets/x/network.json", "manifest.json"},
		"not an address":    {"example", "neither a manifest URL"},
		"nothing listening": {"https://127.0.0.1:1/nets/x/manifest.json", "fetch"},
	} {
		res := run(t, cli, "network", "add", tc.url, "--yes")
		if res.Exit == exitOK || !strings.Contains(output(res), tc.want) {
			t.Errorf("network add %s (%s): exit %d, want a refusal naming %q\n%s", tc.url, label, res.Exit, tc.want, output(res))
		}
	}
	if after := readEnvFile(t, cli); len(after.Environments) != len(before.Environments) {
		t.Errorf("a refused manifest changed the configured networks: %+v -> %+v", before, after)
	}
}

// TestNetworkAdd_gatewayOptionsNeedAGateway: --ca-file and --network describe
// a gateway, --yes a manifest; mixing them is a usage error.
func TestNetworkAdd_gatewayOptionsNeedAGateway(t *testing.T) {
	t.Parallel()
	cli := isolated(t)
	for _, args := range [][]string{
		{"network", "add", "https://example.invalid/m/manifest.json", "--ca-file", "/nonexistent.pem"},
		{"network", "add", "https://example.invalid/m/manifest.json", "--network", "testnet"},
		{"network", "add", "lab", "https://lab.example.invalid", "--yes"},
	} {
		if res := run(t, cli, args...); res.Exit != exitUsage {
			t.Errorf("orama %v: exit %d, want %d\n%s", args, res.Exit, exitUsage, output(res))
		}
	}
}

// TestMaintNetworkPublish_writesANetworkAndKeepsAPublishedGenesis: publish
// writes genesis.json, release-root.json and manifest.json, and a chain id
// that is already published refuses another genesis with the conflict code
// (docs/whitepaper/technical-reference/appendices/d-cli-reference.md#orama-maint-network-publish).
func TestMaintNetworkPublish_writesANetworkAndKeepsAPublishedGenesis(t *testing.T) {
	t.Parallel()
	cli := isolated(t)
	dir := t.TempDir()
	write := func(name, body string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	networks := filepath.Join(dir, "networks")
	genesis := write("genesis.json", `{"chain_id":"orama-e2enet-1"}`)
	root := write("root.json", `{"signed":{"_type":"root"}}`)
	res := cli.MustOK(t, "maint", "network", "publish", "--dir", networks, "--name", "e2enet", "--chain-id", "orama-e2enet-1",
		"--genesis", genesis, "--release-root", root, "--seeds", "seed1.e2enet.example.org", "--channel", "nightly",
		"--min-version", "0.3.0", "--release-repo", "https://releases.example.org/")
	if !strings.Contains(res.Stdout, "orama-e2enet-1") || !strings.Contains(res.Stdout, "sync-networks") {
		t.Errorf("publish printed:\n%s", res.Stdout)
	}
	for _, file := range []string{"manifest.json", "genesis.json", "release-root.json"} {
		if _, err := os.Stat(filepath.Join(networks, "e2enet", file)); err != nil {
			t.Errorf("publish did not write %s: %v", file, err)
		}
	}
	changed := write("changed.json", `{"chain_id":"orama-e2enet-1","app_state":{"changed":true}}`)
	again := run(t, cli, "maint", "network", "publish", "--dir", networks, "--name", "e2enet", "--chain-id", "orama-e2enet-1", "--genesis", changed)
	if again.Exit != exitConflict || !strings.Contains(output(again), "new chain id") {
		t.Errorf("publishing another genesis under a published chain id: exit %d, want %d and the reset rule\n%s", again.Exit, exitConflict, output(again))
	}
}

// TestMaintNetworkAnnounce_writesAnAnnouncementAndRefusesACreatedNetwork:
// announce writes manifest.json and release-root.json with no genesis, can be
// written again, refuses a production chain id with a faucet, and refuses a
// network that is already published with its genesis
// (docs/whitepaper/technical-reference/appendices/d-cli-reference.md#orama-maint-network-announce).
func TestMaintNetworkAnnounce_writesAnAnnouncementAndRefusesACreatedNetwork(t *testing.T) {
	t.Parallel()
	cli := isolated(t)
	dir := t.TempDir()
	root := filepath.Join(dir, "root.json")
	if err := os.WriteFile(root, []byte(`{"signed":{"_type":"root"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	networks := filepath.Join(dir, "networks")
	announce := []string{"maint", "network", "announce", "--dir", networks, "--name", "e2eann", "--chain-id", "orama-e2eann-testnet-1",
		"--release-repo", "https://releases.example.org", "--release-root", root, "--channel", "nightly", "--faucet", "--seed", "seed1.e2eann.example.org"}
	res := cli.MustOK(t, announce...)
	if !strings.Contains(res.Stdout, "orama setup --create-network e2eann") {
		t.Errorf("announce printed:\n%s", res.Stdout)
	}
	if _, err := os.Stat(filepath.Join(networks, "e2eann", "genesis.json")); err == nil {
		t.Error("an announcement wrote a genesis")
	}
	cli.MustOK(t, announce...)

	production := run(t, cli, "maint", "network", "announce", "--dir", networks, "--name", "e2eprod", "--chain-id", "orama-1",
		"--release-repo", "https://releases.example.org", "--release-root", root, "--channel", "main", "--faucet")
	if production.Exit != exitUsage {
		t.Errorf("a production chain id with a faucet: exit %d, want %d\n%s", production.Exit, exitUsage, output(production))
	}

	genesis := filepath.Join(dir, "genesis.json")
	if err := os.WriteFile(genesis, []byte(`{"chain_id":"orama-e2eann-testnet-1"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cli.MustOK(t, "maint", "network", "publish", "--dir", networks, "--name", "e2eann", "--chain-id", "orama-e2eann-testnet-1", "--genesis", genesis)
	again := run(t, cli, announce...)
	if again.Exit != exitConflict || !strings.Contains(output(again), "already created") {
		t.Errorf("announcing a created network: exit %d, want %d and the reason\n%s", again.Exit, exitConflict, output(again))
	}
}

// cluster-only writes manifest.json and release-root.json with no chain id, can be
// written again, refuses a channel no release is published on, and refuses a
// network whose manifest names a chain; the devnet built into the binary is one
// and lists no chain
// (docs/whitepaper/technical-reference/appendices/d-cli-reference.md#orama-maint-network-cluster-only).
func TestMaintNetworkClusterOnly_writesANetworkWithNoChainAndRefusesOneWithAChain(t *testing.T) {
	t.Parallel()
	cli := isolated(t)
	dir := t.TempDir()
	root := filepath.Join(dir, "root.json")
	if err := os.WriteFile(root, []byte(`{"signed":{"_type":"root"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	networks := filepath.Join(dir, "networks")
	write := func(name, channel string) []string {
		return []string{"maint", "network", "cluster-only", "--dir", networks, "--name", name,
			"--release-repo", "https://releases.example.org", "--release-root", root, "--channel", channel, "--min-version", "0.3.6"}
	}
	res := cli.MustOK(t, write("e2edev", "nightly")...)
	if !strings.Contains(res.Stdout, "Wrote cluster-only network e2edev") || !strings.Contains(res.Stdout, "--network e2edev") {
		t.Errorf("cluster-only printed:\n%s", res.Stdout)
	}
	manifest, err := os.ReadFile(filepath.Join(networks, "e2edev", "manifest.json"))
	if err != nil || !strings.Contains(string(manifest), `"chain_id": ""`) || !strings.Contains(string(manifest), `"seeds": []`) {
		t.Errorf("manifest.json: %v\n%s", err, manifest)
	}
	cli.MustOK(t, write("e2edev", "main")...)

	if stable := run(t, cli, write("e2edev", "stable")...); stable.Exit != exitUsage {
		t.Errorf("an unpublished channel: exit %d, want %d\n%s", stable.Exit, exitUsage, output(stable))
	}

	cli.MustOK(t, "maint", "network", "announce", "--dir", networks, "--name", "e2echain", "--chain-id", "orama-e2echain-testnet-1",
		"--release-repo", "https://releases.example.org", "--release-root", root, "--channel", "nightly")
	if over := run(t, cli, write("e2echain", "nightly")...); over.Exit != exitConflict || !strings.Contains(output(over), "orama-e2echain-testnet-1") {
		t.Errorf("over a network with a chain: exit %d, want %d naming the chain\n%s", over.Exit, exitConflict, output(over))
	}

	if row := networkRow(t, cli, "devnet"); row["chain"] != "none (cluster-only)" {
		t.Errorf("the built-in devnet lists chain %q, want none (cluster-only)", row["chain"])
	}
}

// testnetChainID is the chain id class of the public test network (core/pkg/netclass).
var testnetChainID = regexp.MustCompile(`^orama-testnet-[0-9]+$`)

// TestNetworkList_builtinTestnetReplacesStagenet: the binary embeds testnet
// (networks/testnet, core/pkg/netregistry/embedded) with its testnet-class chain id,
// and no longer the retired stagenet
// (website/src/docs/operator/upgrade-notes.mdx, "the test class is testnet, not stagenet").
func TestNetworkList_builtinTestnetReplacesStagenet(t *testing.T) {
	t.Parallel()
	cli := emptyHome(t)
	row := networkRow(t, cli, "testnet")
	if !testnetChainID.MatchString(row["chain"]) || row["source"] != "built in" {
		t.Errorf("the built-in testnet lists chain %q from %q, want an orama-testnet-<n> chain built in", row["chain"], row["source"])
	}
	var rows []map[string]string
	if err := json.Unmarshal([]byte(cli.MustOK(t, "network", "list", "--json").Stdout), &rows); err != nil {
		t.Fatalf("orama network list --json: %v", err)
	}
	for _, r := range rows {
		if r["name"] == "stagenet" {
			t.Errorf("orama network list still lists the retired stagenet: %v", r)
		}
	}
}
