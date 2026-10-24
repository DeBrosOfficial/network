// Package upgradecmd is `orama upgrade`: put the newest signed release of your
// network's channel on every node, one node at a time.
package upgradecmd

import (
	"github.com/spf13/cobra"

	"github.com/Orama-Network/orama/core/cmd/orama/internal/production/relupgrade"
	"github.com/Orama-Network/orama/core/pkg/rollout"
)

// Cmd is the top-level "upgrade" command.
var Cmd = New()

// New builds the command; each call has its own flag storage.
func New() *cobra.Command {
	var opts relupgrade.Options
	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade your nodes to the newest signed release of the network's channel",
		Long: `Find the newest release of your network's channel, show what each node runs and
what it will go through, and after you confirm, upgrade the nodes one at a time.

The release comes from the release repository and channel the network publishes
(orama network list). Its signed metadata is verified here against the release
root built into this CLI; the archive is not downloaded here. Each node downloads
the archive from the repository itself, checks it against the signed length and
SHA-256, and verifies it again against the release root it adopted, refusing a
release that does not match, before it replaces a file. --upload-release
downloads and verifies the archive here instead and uploads it to each node over
SSH, for nodes that cannot reach the repository.

The plan lists each node with the release it runs now (from the cluster's
telemetry, as 'orama status' shows it), the release it will run, and its place in
the rollout: followers first, nameservers spread so the zone keeps answering, the
raft leader last. A node that already runs the release is left alone
(--reinstall puts it in place again); a node that runs a newer one is never
downgraded.

The release is staged on every node first, which restarts nothing. Then each node
in turn is upgraded and restarted, and the next one starts only when that node is
healthy and carrying its share of the cluster again: never two RQLite voters at once.
A node that has the global layer also refreshes it right after its own upgrade:
the global binaries are replaced, and the services that run them are restarted.
The chain binary (oramad) is staged for cosmovisor only when the release carries
another one and the chain has a governed upgrade scheduled; otherwise the running
oramad is kept and the output says so.

--node upgrades one node. --dry-run prints the plan and stops.

Examples:
  orama upgrade --dry-run          # What would change
  orama upgrade                    # Show the plan, ask, then roll
  orama upgrade --yes              # Roll without asking
  orama upgrade --node 203.0.113.7
  orama upgrade --upload-release   # Nodes that cannot reach the repository`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts.Out = cmd.OutOrStdout()
			return relupgrade.Run(cmd.Context(), opts)
		},
	}
	f := cmd.Flags()
	f.StringVar(&opts.Env, "env", "", "Network to upgrade (default: the active one)")
	f.StringVar(&opts.Node, "node", "", "Upgrade only the node with this public IP")
	f.BoolVar(&opts.Yes, "yes", false, "Do not ask for confirmation")
	f.BoolVar(&opts.DryRun, "dry-run", false, "Print the plan and stop; nothing is staged or restarted")
	f.BoolVar(&opts.Reinstall, "reinstall", false, "Put the release in place again on nodes that already run it")
	f.BoolVar(&opts.UploadRelease, "upload-release", false,
		"Download the release on this computer and upload it to each node over SSH, instead of each node downloading it from the release repository (for nodes that cannot reach the repository)")
	f.BoolVar(&opts.SSH, "ssh", false, "Read what the nodes run over SSH instead of the gateway's telemetry")
	f.IntVar(&opts.Delay, "delay", int(rollout.GateBudget.Seconds()),
		"Seconds a node has to rejoin the cluster after its upgrade before the rollout stops")
	return cmd
}
