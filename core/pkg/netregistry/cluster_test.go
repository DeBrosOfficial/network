package netregistry

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// clusterOnlyManifest is a network that runs no chain: the release facts and the
// root, and nothing of a chain.
func clusterOnlyManifest() Manifest {
	return Manifest{
		Name: "testdev", Channel: ChannelNightly, MinVersion: "0.3.6",
		ReleaseRepo: "https://releases.example.org", ReleaseRootSHA256: Digest(testRoot),
	}
}

func clusterOnlyInput(dir string) ClusterOnlyInput {
	return ClusterOnlyInput{
		Dir: dir, Name: "testdev", ReleaseRoot: testRoot, Channel: ChannelNightly,
		MinVersion: "0.3.6", ReleaseRepo: "https://releases.example.org",
	}
}

func TestParseManifest_aClusterOnlyNetworkIsValidAndNeitherAnnouncedNorCreated(t *testing.T) {
	m, err := ParseManifest(marshalManifest(t, clusterOnlyManifest()))
	if err != nil {
		t.Fatalf("a cluster-only manifest was refused: %v", err)
	}
	if !m.ClusterOnly() || m.Announced() || m.Created() {
		t.Errorf("ClusterOnly %v, Announced %v, Created %v; want true, false, false", m.ClusterOnly(), m.Announced(), m.Created())
	}
	for name, kind := range map[string]Manifest{"created": validManifest(), "announced": announcedManifest()} {
		if kind.ClusterOnly() {
			t.Errorf("a %s network is not cluster-only", name)
		}
	}
}

func TestManifestValidate_aClusterOnlyNetworkCarriesNothingOfAChain(t *testing.T) {
	for _, tc := range []struct {
		name   string
		edit   func(*Manifest)
		wantIn string
	}{
		{"a genesis digest", func(m *Manifest) { m.GenesisSHA256 = Digest(testGenesis) }, "genesis_sha256"},
		{"seeds", func(m *Manifest) { m.Seeds = []string{"seed1.testdev.example.org"} }, "seeds"},
		{"a faucet", func(m *Manifest) { m.Faucet = true }, "faucet"},
		{"a Tor network file", func(m *Manifest) { m.TorNetworkSHA256 = Digest(testRoot) }, "tor_network_sha256"},
		{"no release repo", func(m *Manifest) { m.ReleaseRepo = "" }, "release_repo"},
		{"no release root", func(m *Manifest) { m.ReleaseRootSHA256 = "" }, "release_root_sha256"},
		{"no channel", func(m *Manifest) { m.Channel = "" }, "channel"},
		{"no min version", func(m *Manifest) { m.MinVersion = "" }, "min_version"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := clusterOnlyManifest()
			tc.edit(&m)
			err := m.Validate()
			if err == nil {
				t.Fatal("Validate accepted it")
			}
			if !strings.Contains(err.Error(), tc.wantIn) {
				t.Errorf("error %q does not mention %q", err, tc.wantIn)
			}
		})
	}
}

func TestManifestCheckCreated_aClusterOnlyNetworkHasNoChainToJoin(t *testing.T) {
	m := clusterOnlyManifest()
	err := m.CheckCreated()
	if !errors.Is(err, ErrClusterOnly) || errors.Is(err, ErrNotCreated) || !strings.Contains(err.Error(), "testdev is a cluster-only network") {
		t.Fatalf("CheckCreated = %v, want ErrClusterOnly naming the network", err)
	}
	if err := m.VerifyGenesis(testGenesis); !errors.Is(err, ErrClusterOnly) {
		t.Errorf("VerifyGenesis = %v, want ErrClusterOnly", err)
	}
}

func TestFetchGenesis_aClusterOnlyNetworkIsNotFetched(t *testing.T) {
	m := clusterOnlyManifest()
	m.Name = "teststage"
	files := map[string][]byte{ManifestFile: marshalManifest(t, m), ReleaseRootFile: testRoot}
	srv, client := networkServer(t, files)
	n, err := FetchNetwork(context.Background(), client, srv.URL+"/nets/teststage/manifest.json")
	if err != nil {
		t.Fatalf("a cluster-only network can be added by URL: %v", err)
	}
	if _, err := n.FetchGenesis(context.Background(), client); !errors.Is(err, ErrClusterOnly) {
		t.Fatalf("FetchGenesis = %v, want ErrClusterOnly", err)
	}
}

func TestPublishClusterOnly_writesAManifestThatLoads(t *testing.T) {
	dir := t.TempDir()
	m, err := PublishClusterOnly(clusterOnlyInput(dir))
	if err != nil {
		t.Fatal(err)
	}
	if !m.ClusterOnly() || m.Channel != ChannelNightly || m.MinVersion != "0.3.6" || m.ReleaseRootSHA256 != Digest(testRoot) {
		t.Errorf("manifest = %+v", m)
	}
	onDisk, err := os.ReadFile(filepath.Join(dir, "testdev", ManifestFile))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"chain_id": ""`, `"genesis_sha256": ""`, `"seeds": []`, `"faucet": false`} {
		if !strings.Contains(string(onDisk), want) {
			t.Errorf("manifest.json lacks %s:\n%s", want, onDisk)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "testdev", GenesisFile)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a cluster-only network carries no genesis.json: %v", err)
	}
	n, err := loadNetworkDir(os.DirFS(filepath.Join(dir, "testdev")), ".", "testdev")
	if err != nil || !n.Manifest.ClusterOnly() {
		t.Fatalf("what PublishClusterOnly wrote does not load: %v", err)
	}
}

func TestPublishClusterOnly_writingAgainReplacesIt(t *testing.T) {
	dir := t.TempDir()
	if _, err := PublishClusterOnly(clusterOnlyInput(dir)); err != nil {
		t.Fatal(err)
	}
	in := clusterOnlyInput(dir)
	in.Channel, in.MinVersion = ChannelMain, "0.4.0"
	m, err := PublishClusterOnly(in)
	if err != nil || m.Channel != ChannelMain || m.MinVersion != "0.4.0" {
		t.Fatalf("PublishClusterOnly = %+v, %v", m, err)
	}
}

func TestPublishClusterOnly_refusesANetworkWithAChain(t *testing.T) {
	for name, write := range map[string]func(t *testing.T, dir string){
		"created": func(t *testing.T, dir string) { firstPublish(t, dir) },
		"announced": func(t *testing.T, dir string) {
			if _, err := Announce(announceInput(dir)); err != nil {
				t.Fatal(err)
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			write(t, dir)
			before, _ := os.ReadFile(filepath.Join(dir, "teststage", ManifestFile))
			in := clusterOnlyInput(dir)
			in.Name = "teststage"
			_, err := PublishClusterOnly(in)
			if !errors.Is(err, ErrHasChain) || !strings.Contains(err.Error(), "orama-teststage-1") {
				t.Fatalf("PublishClusterOnly = %v, want ErrHasChain naming the chain", err)
			}
			if after, _ := os.ReadFile(filepath.Join(dir, "teststage", ManifestFile)); string(after) != string(before) {
				t.Error("a refused cluster-only manifest changed the published network")
			}
		})
	}
}

func TestPublishClusterOnly_inputErrors(t *testing.T) {
	for name, edit := range map[string]func(*ClusterOnlyInput){
		"a release root that is not JSON": func(in *ClusterOnlyInput) { in.ReleaseRoot = []byte("x") },
		"a bad channel":                   func(in *ClusterOnlyInput) { in.Channel = "stable" },
		"a release repo over http":        func(in *ClusterOnlyInput) { in.ReleaseRepo = "http://releases.example.org" },
		"a bad name":                      func(in *ClusterOnlyInput) { in.Name = "Devnet" },
	} {
		t.Run(name, func(t *testing.T) {
			in := clusterOnlyInput(t.TempDir())
			edit(&in)
			if _, err := PublishClusterOnly(in); err == nil {
				t.Fatal("PublishClusterOnly accepted it")
			}
			if entries, _ := os.ReadDir(in.Dir); len(entries) != 0 {
				t.Errorf("a refused manifest left %d entries behind", len(entries))
			}
		})
	}
}

func TestAnnounce_andPublishReplaceAClusterOnlyNetworkThatGetsAChain(t *testing.T) {
	dir := t.TempDir()
	in := clusterOnlyInput(dir)
	in.Name = "teststage"
	if _, err := PublishClusterOnly(in); err != nil {
		t.Fatal(err)
	}
	m, err := Announce(announceInput(dir))
	if err != nil || !m.Announced() {
		t.Fatalf("Announce over a cluster-only network = %+v, %v", m, err)
	}
	if _, err := Publish(PublishInput{Dir: dir, Name: "teststage", ChainID: "orama-teststage-1", Genesis: testGenesis}); err != nil {
		t.Fatalf("Publish over the announcement: %v", err)
	}
}
