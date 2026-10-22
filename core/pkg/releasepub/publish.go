package releasepub

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Orama-Network/orama/core/pkg/releaseverify"
)

// giteaRepoRe matches an owner/name Gitea repository.
var giteaRepoRe = regexp.MustCompile(`^[A-Za-z0-9._-]+/[A-Za-z0-9._-]+$`)

// Where releases go. Both are overridable by flag; they are the owner's
// release host and the Gitea repository whose releases carry the archives.
const (
	// DefaultGiteaRepo (owner/name) holds the archives as release assets.
	DefaultGiteaRepo = "ICXCNIKA/orama"
	// DefaultGiteaAPI is the Gitea instance's API base.
	DefaultGiteaAPI = "https://git.icxcnika.io/api/v1"
	// DefaultMetadataDest is the rsync destination of the metadata: the
	// directory the release host's nginx serves (website/deploy/releases.orama.network.nginx.conf).
	DefaultMetadataDest = "releases.orama.network:/opt/orama-releases/"
)

// Runner runs a command and returns what it printed. A test replaces it.
type Runner func(ctx context.Context, name string, args ...string) ([]byte, error)

// ExecRunner runs the command for real, passing its output on to out.
func ExecRunner(out io.Writer) Runner {
	return func(ctx context.Context, name string, args ...string) ([]byte, error) {
		var buf bytes.Buffer
		cmd := exec.CommandContext(ctx, name, args...)
		// One writer for both streams, so exec serialises the writes to buf.
		w := io.MultiWriter(&buf, out)
		cmd.Stdout, cmd.Stderr = w, w
		err := cmd.Run()
		return buf.Bytes(), err
	}
}

// PublishParams is one upload.
type PublishParams struct {
	Repo Repo
	// GiteaRepo is the owner/name whose releases hold the archives, GiteaAPI
	// the instance's API base and GiteaToken a token with repository write.
	GiteaRepo    string
	GiteaAPI     string
	GiteaToken   string
	MetadataDest string
	// DryRun checks the directory and prints the plan without sending anything.
	DryRun bool
	Now    time.Time
	// Run runs rsync; the archives go over HTTP.
	Run      Runner
	HTTP     *http.Client
	Progress io.Writer
}

// Publish uploads what the last cut or refresh left in the repository
// directory. The order keeps a client from ever seeing metadata that points at
// something absent: the archives go to Gitea first, then the root, targets and
// snapshot, and the timestamp, the file a client reads first, last. Before
// anything is sent the directory is held to what a client does with it, and
// each archive to the hash the signed targets name.
func Publish(ctx context.Context, p PublishParams) error {
	if p.Progress == nil {
		p.Progress = io.Discard
	}
	if p.HTTP == nil {
		// No overall client timeout: a large archive upload may run long and
		// the context governs the whole call. The transport bounds a hung
		// server at each stage instead.
		p.HTTP = &http.Client{Transport: &http.Transport{
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second,
			IdleConnTimeout:       60 * time.Second,
		}}
	}
	if err := p.checkLocal(); err != nil {
		return err
	}
	if err := p.checkDestinations(); err != nil {
		return err
	}
	pending, err := p.Repo.ReadPending()
	if err != nil {
		return err
	}
	if pending != nil {
		// The token is needed only to upload archives; a timestamp-only refresh
		// publishes metadata over rsync and needs none.
		if !p.DryRun && p.GiteaToken == "" {
			return errors.New("GITEA_TOKEN is not set: export a Gitea token with repository write access to upload the archives")
		}
		if err := p.uploadAssets(ctx, pending); err != nil {
			return err
		}
	}
	files, err := p.metadataFiles()
	if err != nil {
		return err
	}
	if err := p.sync(ctx, files[:len(files)-1]); err != nil {
		return err
	}
	if err := p.sync(ctx, files[len(files)-1:]); err != nil {
		return err
	}
	if p.DryRun {
		return nil
	}
	return p.Repo.clearPending()
}

// checkLocal verifies the directory as a client would, and the pending
// archives against it.
func (p PublishParams) checkLocal() error {
	rootBytes, _, err := p.Repo.ReadRoot()
	if err != nil {
		return err
	}
	meta := releaseverify.Metadata{Root: rootBytes}
	for name, dst := range map[string]*[]byte{TimestampFile: &meta.Timestamp, SnapshotFile: &meta.Snapshot, TargetsFile: &meta.Targets} {
		if *dst, err = os.ReadFile(p.Repo.path(name)); err != nil {
			return fmt.Errorf("read %s: %w (cut a release first)", p.Repo.path(name), err)
		}
	}
	verified, err := releaseverify.Verify(meta, releaseverify.Seen{}, p.Now)
	if err != nil {
		return fmt.Errorf("the metadata in %s would not verify on a client; nothing was uploaded: %w", p.Repo.Dir, err)
	}
	pending, err := p.Repo.ReadPending()
	if err != nil || pending == nil {
		return err
	}
	if err := checkPending(pending); err != nil {
		return err
	}
	for _, a := range pending.Assets {
		target, ok := verified.Targets[a.Target]
		if !ok {
			return fmt.Errorf("the signed targets do not list %s, which the pending release would upload", a.Target)
		}
		got, err := fileSHA256(a.Path)
		if err != nil {
			return fmt.Errorf("read the archive %s: %w", a.Path, err)
		}
		if want := fmt.Sprintf("%x", target.Hashes["sha256"]); got != want {
			return fmt.Errorf("%s has sha256 %s, not the %s the signed targets name; it changed after the cut", a.Path, got, want)
		}
	}
	return nil
}

// metadataFiles are the files the host serves, the timestamp last: every
// version of the root, the targets and the snapshot.
func (p PublishParams) metadataFiles() ([]string, error) {
	roots, err := filepath.Glob(p.Repo.path("*.root.json"))
	if err != nil {
		return nil, fmt.Errorf("list the root versions: %w", err)
	}
	sort.Strings(roots)
	files := append(roots, p.Repo.path(RootFile), p.Repo.path(TargetsFile), p.Repo.path(SnapshotFile), p.Repo.path(TimestampFile))
	return files, nil
}

// sync copies files to the metadata destination with rsync over ssh.
func (p PublishParams) sync(ctx context.Context, files []string) error {
	args := append([]string{"-a", "--chmod=F644", "-e", "ssh -o BatchMode=yes", "--"}, files...)
	args = append(args, p.MetadataDest)
	if p.DryRun {
		p.print("rsync", args, "")
		return nil
	}
	fmt.Fprintf(p.Progress, "Sending %s to %s...\n", summarize(baseNames(files)), p.MetadataDest)
	if _, err := p.Run(ctx, "rsync", args...); err != nil {
		return fmt.Errorf("send the metadata to %s: %w", p.MetadataDest, err)
	}
	return nil
}

func (p PublishParams) print(name string, args []string, note string) {
	if note != "" {
		fmt.Fprintln(p.Progress, note)
	}
	fmt.Fprintf(p.Progress, "  %s %s\n", name, strings.Join(args, " "))
}

func baseNames(paths []string) []string {
	out := make([]string, len(paths))
	for i, p := range paths {
		out[i] = filepath.Base(p)
	}
	return out
}

// checkDestinations refuses a destination that rsync would read as an option,
// a Gitea API base that is not an HTTP URL, and (outside a dry run) a missing
// token.
func (p PublishParams) checkDestinations() error {
	if p.MetadataDest == "" || strings.HasPrefix(p.MetadataDest, "-") {
		return fmt.Errorf("--metadata-dest %q is empty or starts with '-'", p.MetadataDest)
	}
	if !giteaRepoRe.MatchString(p.GiteaRepo) {
		return fmt.Errorf("--gitea-repo %q is not an owner/name repository", p.GiteaRepo)
	}
	u, err := url.Parse(p.GiteaAPI)
	// https only, so a typo or redirect can never send the token in the clear;
	// plain http is allowed only to a loopback host, for tests.
	if err != nil || u.Host == "" || (u.Scheme != "https" && !(u.Scheme == "http" && isLoopbackHost(u.Hostname()))) {
		return fmt.Errorf("--gitea-api %q must be an https:// URL (http:// only for a loopback test host)", p.GiteaAPI)
	}
	return nil
}

// isLoopbackHost reports whether host is localhost or a loopback IP literal.
func isLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// checkPending holds the pending record, a file the maintainer's account can
// write, to what a cut would have written: the tag is the channel's tag for the
// version, and every asset is an absolute path of an archive named for them.
func checkPending(p *Pending) error {
	ch, err := ParseChannel(p.Channel)
	if err != nil || ch.Name != p.Channel {
		return fmt.Errorf("the pending record names the channel %q, which is not one a cut makes", p.Channel)
	}
	if want := ch.Tag(p.Version); p.Tag != want {
		return fmt.Errorf("the pending record's tag %q is not %q, the tag of %s %s", p.Tag, want, p.Channel, p.Version)
	}
	for _, a := range p.Assets {
		ref, err := releaseverify.ParseArchiveTarget(a.Target)
		if err != nil || ref.Channel != p.Channel || ref.Version != p.Version || !filepath.IsAbs(a.Path) || filepath.Base(a.Path) != a.Name {
			return fmt.Errorf("the pending record's asset %q (%s) is not an archive of %s %s at an absolute path", a.Name, a.Path, p.Channel, p.Version)
		}
	}
	return nil
}
