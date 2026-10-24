package relupgrade

import (
	"testing"
	"testing/fstest"
	"time"

	cli "github.com/Orama-Network/orama/core/cmd/orama/internal"
	"github.com/Orama-Network/orama/core/pkg/netregistry"
)

// A cluster-only network names a release channel and no chain; a cluster on it is upgraded from
// that channel like one on a network with a chain.
func TestResolveTarget_aClusterOnlyNetworkHasAReleaseChannel(t *testing.T) {
	root := []byte(`{"signed":{"_type":"root"}}`)
	m := netregistry.Manifest{
		Name: "devnet", Channel: netregistry.ChannelNightly, MinVersion: "0.3.6",
		ReleaseRepo: "https://test-releases.orama.network", ReleaseRootSHA256: netregistry.Digest(root),
	}
	manifest, err := m.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	reg, err := netregistry.LoadFS(fstest.MapFS{
		"r/devnet/" + netregistry.ManifestFile:    {Data: manifest},
		"r/devnet/" + netregistry.ReleaseRootFile: {Data: root},
	}, "r")
	if err != nil {
		t.Fatalf("a cluster-only network does not load: %v", err)
	}
	target, err := ResolveTarget(&cli.Environment{Name: "devnet", Network: "devnet"}, reg)
	if err != nil {
		t.Fatalf("ResolveTarget: %v", err)
	}
	p := target.FetchParams("/home/u/.orama", "amd64", time.Unix(5, 0))
	if p.RepoURL != m.ReleaseRepo || p.Channel != "nightly" || p.MinVersion != "0.3.6" || p.RootSHA256 != m.ReleaseRootSHA256 || string(p.Root) != string(root) {
		t.Errorf("fetch params %+v: the channel, repository, minimum version and root come from the manifest", p)
	}
}
