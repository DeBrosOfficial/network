package networkcmd

import (
	"errors"
	"os"

	"github.com/spf13/cobra"

	"github.com/Orama-Network/orama/core/cmd/orama/internal/clierr"
	"github.com/Orama-Network/orama/core/cmd/orama/internal/printer"
	"github.com/Orama-Network/orama/core/pkg/netregistry"
	"github.com/Orama-Network/orama/core/pkg/version"
)

func newClusterOnlyCmd() *cobra.Command {
	var dir, name, rootFile, releaseRepo, channel, minVersion string
	cmd := &cobra.Command{
		Use:   "cluster-only",
		Short: "Write networks/<name>/ for a network of clusters that runs no chain",
		Long: `Write networks/<name>/ (release-root.json and, last, manifest.json) for a
cluster-only network: one that runs no chain, only clusters whose nodes follow a
release channel. The manifest names the release repository, the channel, the
oldest version that may join and the release root its releases are verified
against. Its chain_id and genesis_sha256 are empty and it has no seeds, faucet or
Tor network file.

A cluster on such a network is upgraded from its channel with 'orama upgrade'
once its environment names the network ('orama network add <name> <gateway URL>
--network <name>'). 'orama setup' installs only cluster nodes on it (--cluster-only),
and a command that signs for a chain is refused there: it has none.

It can be written again to change the channel, the repository, the minimum version
or the root. A network whose manifest names a chain, announced or created, is
refused. A cluster-only network that is to get a chain is announced
('orama maint network announce') or created over it.

Afterwards run 'make -C core sync-networks' so the binary embeds the manifest, and
commit networks/ and core/pkg/netregistry/embedded/ together.`,
		Example: `  orama maint network cluster-only --name devnet \
    --release-repo https://test-releases.orama.network --release-root release-root.json \
    --channel nightly --min-version 0.3.6`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := os.ReadFile(rootFile)
			if err != nil {
				return clierr.Usage("read the release root: %v", err)
			}
			m, err := netregistry.PublishClusterOnly(netregistry.ClusterOnlyInput{
				Dir: dir, Name: name, ReleaseRoot: root, Channel: channel, MinVersion: minVersion, ReleaseRepo: releaseRepo,
			})
			if errors.Is(err, netregistry.ErrHasChain) {
				return clierr.Conflict("%v", err)
			}
			if err != nil {
				return clierr.Usage("write cluster-only network %s: %v", name, err)
			}
			p := printer.For(cmd)
			p.Printf("Wrote cluster-only network %s: channel %s from %s, release root sha256 %s, no chain\n",
				m.Name, m.Channel, m.ReleaseRepo, m.ReleaseRootSHA256)
			p.Printf("Next: make -C core sync-networks, then commit %s/%s and core/pkg/netregistry/embedded/\n", dir, m.Name)
			p.Printf("Its clusters follow it once their environment names it: orama network add <env> <gateway URL> --network %s\n", m.Name)
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&dir, "dir", defaultPublishDir, "The repository's networks directory")
	f.StringVar(&name, "name", "", "Network name, for example devnet [required]")
	f.StringVar(&releaseRepo, "release-repo", "", "https base URL of the release repository [required]")
	f.StringVar(&rootFile, "release-root", "", "The release-root.json file its releases are verified against [required]")
	f.StringVar(&channel, "channel", "", "Release channel: nightly, main or dev/<branch> [required]")
	f.StringVar(&minVersion, "min-version", version.Current, "Oldest orama version that may join, X.Y.Z")
	for _, required := range []string{"name", "release-repo", "release-root", "channel"} {
		_ = cmd.MarkFlagRequired(required)
	}
	return cmd
}
