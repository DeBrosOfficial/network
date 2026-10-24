package relupgrade

import (
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/Orama-Network/orama/core/cmd/orama/internal/production/releasedownload"
	"github.com/Orama-Network/orama/core/pkg/rollout"
	"golang.org/x/sync/errgroup"
)

const (
	// uploadParallel bounds how many nodes are staged at once with
	// --upload-release: staging uploads the archive from this machine, so more
	// would only share its uplink.
	uploadParallel = 4
	// downloadParallel bounds it when every node downloads the archive over its
	// own link: the repository serves static files, so the limit is on this
	// machine's SSH sessions.
	downloadParallel = 8
)

// uploadReleaseHint is what a node that cannot reach the release repository is
// told.
const uploadReleaseHint = "if the node cannot reach the release repository (a closed outbound firewall, a private network), " +
	"run `orama upgrade --upload-release` to upload the archive from this computer instead"

// stageAll stages the verified release on every node the plan restarts, before
// any of them is restarted. A node that refuses leaves the rest staged and
// nothing restarted: staging replaces files, never services.
func (r *runner) stageAll(steps []rollout.Step, releases *releaseSet, out io.Writer) error {
	fmt.Fprintf(out, "\nStaging %s on %d node(s)...\n", releases.version, len(steps))
	var (
		mu   sync.Mutex
		errs []error
		g    errgroup.Group
	)
	limit := downloadParallel
	if r.uploads() {
		limit = uploadParallel
	}
	g.SetLimit(limit)
	for _, step := range steps {
		g.Go(func() error {
			err := r.stageNode(step, releases)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, err)
				fmt.Fprintf(out, "  ✗ %s: %v\n", step.Node.Host, err)
				return nil
			}
			fmt.Fprintf(out, "  ✓ %s staged\n", step.Node.Host)
			return nil
		})
	}
	_ = g.Wait()
	if len(errs) > 0 {
		return fmt.Errorf("%d node(s) refused the release, so nothing was restarted: %w", len(errs), errors.Join(errs...))
	}
	return nil
}

// stageNode stages the release for the node's architecture on one node.
func (r *runner) stageNode(step rollout.Step, releases *releaseSet) error {
	rel := releases.byArch[releases.archOf[step.Node.Host]]
	if rel == nil {
		return fmt.Errorf("no release was fetched for %s", step.Node.Host)
	}
	_, err := r.seams.stage(step.Node, rel.files)
	if err != nil && rel.files.Download != nil && releasedownload.Unreachable(err) {
		return fmt.Errorf("%w; %s", err, uploadReleaseHint)
	}
	return err
}
