// Package releasecmd is `orama maint release`: the maintainer's tool for
// cutting a release into the Orama release repository. It signs TUF metadata
// with the maintainer's RootWallet (one human approval per signature) and
// publishes it: archives to Gitea release assets, metadata to the release
// host. See website/src/docs/contributor/deployment.mdx.
package releasecmd

import (
	"crypto/ed25519"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/Orama-Network/orama/core/cmd/orama/internal/clierr"
	"github.com/Orama-Network/orama/core/pkg/releasepub"
	"github.com/Orama-Network/orama/core/pkg/rwagent"
)

// defaultRepoSubdir is the working copy of the release repository, below the
// home directory.
const defaultRepoSubdir = ".orama/release-repo"

// deps are what the commands reach outside the process for; a test replaces
// them.
type deps struct {
	agent func() releasepub.Agent
	run   releasepub.Runner
	now   func() time.Time
	// runnerKeys are the runner keys whose fleet report a release accepts.
	runnerKeys func() ([]ed25519.PublicKey, error)
}

// NewCommand is the `release` command group, to be mounted under `orama maint`.
func NewCommand() *cobra.Command {
	return newCommand(deps{
		agent:      func() releasepub.Agent { return rwagent.New(os.Getenv("RW_AGENT_SOCK")) },
		now:        time.Now,
		runnerKeys: releasepub.RunnerKeys,
	})
}

func newCommand(d deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "release",
		Short: "Cut and publish a signed release (maintainers)",
		Long: `Cut a release into the Orama release repository and publish it.

The metadata is TUF, signed with the one release key in your RootWallet. Every
signature is approved by you in the RootWallet desktop app; the command says
what each approval is for before it asks. The release key is listed in the
root for all four roles, so the same wallet signs everything.

  init-root          make the repository's root (once); prints the digest networks pin
  renew-root         the next version of the root, same keys, a new expiry
  cut                list an archive on a channel and sign it (3 approvals);
                     nightly and main need a passing, runner-signed fleet e2e report
  refresh-timestamp  re-sign only the timestamp (1 approval)
  publish            upload the archives to Gitea and the metadata to the release host

A channel is nightly, main, or dev/<branch>. cut writes only to the repository
directory (--dir); publish is the step that touches the network, so cut
--dry-run and cut work offline.`,
	}
	cmd.AddCommand(newInitRootCmd(d), newRenewRootCmd(d), newCutCmd(d), newRefreshCmd(d), newPublishCmd(d))
	return cmd
}

// repoFlag registers --dir.
func repoFlag(cmd *cobra.Command, dir *string) {
	cmd.Flags().StringVar(dir, "dir", "", "The release repository working directory (default ~/"+defaultRepoSubdir+")")
}

// repoFor resolves --dir.
func repoFor(dir string) (releasepub.Repo, error) {
	if dir != "" {
		abs, err := filepath.Abs(dir)
		if err != nil {
			return releasepub.Repo{}, clierr.Usage("--dir %q: %v", dir, err)
		}
		return releasepub.Repo{Dir: abs}, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return releasepub.Repo{}, clierr.Usage("--dir is required: no home directory to default it from: %v", err)
	}
	return releasepub.Repo{Dir: filepath.Join(home, defaultRepoSubdir)}, nil
}

func fail(what string, err error) error { return clierr.Failure("%s: %v", what, err) }

func printf(w io.Writer, format string, args ...any) { fmt.Fprintf(w, format, args...) }
