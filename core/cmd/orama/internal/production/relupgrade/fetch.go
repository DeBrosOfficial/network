package relupgrade

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/Orama-Network/orama/core/cmd/orama/internal/clierr"
	"github.com/Orama-Network/orama/core/cmd/orama/internal/production/push"
	"github.com/Orama-Network/orama/core/cmd/orama/internal/production/releasedownload"
	"github.com/Orama-Network/orama/core/pkg/releasefetch"
	"github.com/Orama-Network/orama/core/pkg/rollout"
)

// archByMachine maps `uname -m` to the architectures a release is built for.
var archByMachine = map[string]string{
	"x86_64": "amd64", "amd64": "amd64",
	"aarch64": "arm64", "arm64": "arm64",
}

// archRelease is the verified release for one architecture, as a node is given
// it to stage.
type archRelease struct {
	version string
	files   push.ReleaseFiles
	// remove deletes what was fetched to this machine. accept raises this
	// machine's rollback record once every node has verified the archive; it is
	// nil for a release whose archive was verified here, which raised it then.
	remove func() error
	accept func() error
}

// releaseSet is the verified releases of one upgrade, by architecture.
type releaseSet struct {
	byArch map[string]*archRelease
	// archOf is each planned node's architecture, read once.
	archOf  map[string]string
	version string
}

// remove deletes what the fetches left on this machine.
func (s *releaseSet) remove() {
	for _, rel := range s.byArch {
		_ = rel.remove()
	}
}

// accept raises the rollback record for every release the nodes downloaded and
// verified themselves.
func (s *releaseSet) accept() error {
	var errs []error
	for _, rel := range s.byArch {
		if rel.accept != nil {
			errs = append(errs, rel.accept())
		}
	}
	return errors.Join(errs...)
}

// fetchReleases verifies the channel's newest release for each architecture the
// planned nodes run, and requires them to be one version.
func (r *runner) fetchReleases(ctx context.Context, steps []rollout.Step, out io.Writer) (*releaseSet, error) {
	set := &releaseSet{byArch: map[string]*archRelease{}, archOf: map[string]string{}}
	for _, step := range steps {
		machine, err := r.seams.arch(step.Node)
		if err != nil {
			return nil, set.fail(err)
		}
		arch, ok := archByMachine[strings.TrimSpace(machine)]
		if !ok {
			return nil, set.fail(clierr.Failure("%s reports the machine type %q; releases are built for amd64 and arm64", step.Node.Host, strings.TrimSpace(machine)))
		}
		set.archOf[step.Node.Host] = arch
		if _, done := set.byArch[arch]; done {
			continue
		}
		rel, err := r.release(ctx, arch, out)
		if err != nil {
			return nil, set.fail(clierr.Failure("%v", err))
		}
		set.byArch[arch] = rel
		if set.version != "" && set.version != rel.version {
			return nil, set.fail(clierr.Conflict("the channel's newest release is %s for one architecture and %s for another; "+
				"the release is still being published, run `orama upgrade` again once it is complete", set.version, rel.version))
		}
		set.version = rel.version
	}
	if len(set.byArch) == 0 {
		return nil, clierr.Failure("no node to upgrade")
	}
	return set, nil
}

// uploads says whether this machine downloads the archive and uploads it to the
// nodes (--upload-release). A dry run stages nothing, so it never downloads it.
func (r *runner) uploads() bool { return r.opts.UploadRelease && !r.opts.DryRun }

// release verifies the newest release of the channel for arch. By default only
// the metadata, a few kilobytes, is fetched and verified here, and every node
// downloads the archive from the repository itself. With --upload-release the
// archive is downloaded and verified here too, to be uploaded to each node.
func (r *runner) release(ctx context.Context, arch string, out io.Writer) (*archRelease, error) {
	p := r.target.FetchParams(r.home, arch, r.now)
	if r.uploads() {
		fmt.Fprintf(out, "Fetching the newest %s release of %s for linux/%s...\n", r.target.Manifest.Channel, r.target.Network, arch)
		return r.seams.fetch(ctx, p)
	}
	fmt.Fprintf(out, "Verifying the newest %s release of %s for linux/%s; each node downloads it from %s itself...\n",
		r.target.Manifest.Channel, r.target.Network, arch, r.target.Manifest.ReleaseRepo)
	return r.seams.resolve(ctx, p)
}

// fetchRelease downloads and verifies the release here, for it to be uploaded
// to the nodes (releasefetch.Fetch).
func fetchRelease(ctx context.Context, p releasefetch.Params) (*archRelease, error) {
	rel, err := releasefetch.Fetch(ctx, p)
	if err != nil {
		return nil, err
	}
	return &archRelease{
		version: rel.Version, remove: rel.Remove,
		files: push.ReleaseFiles{Archive: rel.ArchivePath, MetadataDir: rel.MetadataDir, Target: rel.Target, Root: rel.Root},
	}, nil
}

// resolveRelease verifies the release's metadata here and names the archive
// for every node to download (releasefetch.Resolve).
func resolveRelease(ctx context.Context, p releasefetch.Params) (*archRelease, error) {
	res, err := releasefetch.Resolve(ctx, p)
	if err != nil {
		return nil, err
	}
	return &archRelease{
		version: res.Version, remove: res.Remove, accept: res.Accept,
		files: push.ReleaseFiles{
			Download:    &releasedownload.Archive{URL: res.URL, Length: res.Length, SHA256: res.SHA256},
			MetadataDir: res.MetadataDir, Target: res.Target, Root: res.Root,
		},
	}, nil
}

// fail removes the fetches already made and returns err.
func (s *releaseSet) fail(err error) error {
	s.remove()
	return err
}
