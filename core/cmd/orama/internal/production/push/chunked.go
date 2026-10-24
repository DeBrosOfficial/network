package push

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Orama-Network/orama/core/cmd/orama/internal/printer"
	"github.com/Orama-Network/orama/core/pkg/inspector"
	"github.com/Orama-Network/orama/core/pkg/remotessh"
)

// Uploading an archive so that a dropped connection costs one chunk.
//
// One scp of the whole archive started over from byte zero whenever the
// operator's uplink reset it, so on a slow or unstable link a push could fail
// on every attempt. The archive now goes up in chunks over the node's pinned
// SSH session: each chunk lands in a file of its own in the private upload
// directory and is renamed into place only once its sha256 matches, and a
// chunk whose session drops is sent again, a bounded number of times, from
// its first byte. Chunks already in place are never sent again. The node then
// concatenates the chunks and checks the whole archive's sha256; the stage
// that follows verifies its signature as before. Only the plain shell tools
// every node has are used (cat, mktemp, mv, sha256sum): nothing is installed
// on the node for this.

const (
	// uploadChunkSize bounds what a dropped connection costs: at the slowest
	// uplinks seen (~300 KiB/s) a chunk is about a minute.
	uploadChunkSize = 16 << 20
	// uploadAttempts is how many times one chunk is sent when its session
	// drops before the push gives up on the node.
	uploadAttempts = 5
	// uploadResumePause lets a reset link come back before a chunk is resent.
	uploadResumePause = 5 * time.Second
	// sshTransportExit is ssh's exit status when the session itself failed,
	// as opposed to the remote command's own status.
	sshTransportExit = 255
	// chunkPrefix and chunkTempPrefix name a verified chunk and one being
	// received inside the upload directory.
	chunkPrefix     = "part."
	chunkTempPrefix = ".chunk."
)

// errTransport marks a failure of the SSH session, not of the command it ran:
// the one failure a chunk is resent for.
var errTransport = errors.New("the SSH session dropped")

// chunk is one piece of the archive and its sha256.
type chunk struct {
	off, size int64
	sum       string
}

// uploadPlan is the archive cut into chunks, with its whole sha256.
type uploadPlan struct {
	path   string
	size   int64
	sum    string
	chunks []chunk
}

// planUpload hashes the archive at path once, chunk by chunk and whole.
func planUpload(path string, chunkSize int64) (*uploadPlan, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open the archive: %w", err)
	}
	defer f.Close()
	whole := sha256.New()
	p := &uploadPlan{path: path}
	for {
		part := sha256.New()
		n, err := io.Copy(io.MultiWriter(whole, part), io.LimitReader(f, chunkSize))
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		if n == 0 {
			break
		}
		p.chunks = append(p.chunks, chunk{off: p.size, size: n, sum: hex.EncodeToString(part.Sum(nil))})
		p.size += n
	}
	p.sum = hex.EncodeToString(whole.Sum(nil))
	return p, nil
}

// remoteRun runs command on the node with stdin as its input. It returns an
// error wrapping errTransport when the session failed rather than the command.
type remoteRun func(command string, stdin io.Reader) error

// nodeRun is remoteRun over the node's pinned SSH session, as its login user:
// the upload directory is that user's (makeUploadDir).
func nodeRun(node inspector.Node) remoteRun {
	return func(command string, stdin io.Reader) error {
		err := remotessh.RunSSHStreaming(node, command, remotessh.WithStdin(stdin))
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == sshTransportExit {
			return fmt.Errorf("%w: %w", errTransport, err)
		}
		return err
	}
}

// uploader sends a plan to one upload directory.
type uploader struct {
	run   remoteRun
	pause func()
	// progress reports the bytes in place on the node after each chunk.
	progress func(done, total int64)
}

// newNodeUploader is the uploader a push uses for node.
func newNodeUploader(node inspector.Node) *uploader {
	return &uploader{
		run:   nodeRun(node),
		pause: func() { time.Sleep(uploadResumePause) },
		progress: func(done, total int64) {
			fmt.Printf("  uploaded %s / %s\n", printer.FormatBytes(done), printer.FormatBytes(total))
		},
	}
}

// upload puts the archive of plan at uploadPath(dir), verified whole.
func (u *uploader) upload(plan *uploadPlan, dir string) error {
	f, err := os.Open(plan.path)
	if err != nil {
		return fmt.Errorf("open the archive: %w", err)
	}
	defer f.Close()
	var done int64
	for i, c := range plan.chunks {
		err := u.attempt(func() error {
			return u.run(chunkCommand(dir, i, c.sum), io.NewSectionReader(f, c.off, c.size))
		})
		if err != nil {
			return fmt.Errorf("chunk %d/%d: %w", i+1, len(plan.chunks), err)
		}
		done += c.size
		u.progress(done, plan.size)
	}
	err = u.attempt(func() error {
		return u.run(assembleCommand(dir, len(plan.chunks), plan.sum), strings.NewReader(""))
	})
	if err != nil {
		return fmt.Errorf("assemble the archive: %w", err)
	}
	return nil
}

// attempt runs op until it succeeds, fails other than by a dropped session,
// or has dropped uploadAttempts times. op is idempotent: a chunk lands under
// a fresh name and is renamed into place only once verified, and assembling
// starts from verified chunks.
func (u *uploader) attempt(op func() error) error {
	var err error
	for n := 1; n <= uploadAttempts; n++ {
		if err = op(); !errors.Is(err, errTransport) {
			return err
		}
		if n < uploadAttempts {
			fmt.Printf("  %v; resuming (%d/%d)\n", err, n+1, uploadAttempts)
			u.pause()
		}
	}
	return fmt.Errorf("%d attempts: %w", uploadAttempts, err)
}

// chunkName is chunk i inside dir; zero-padded, so the names sort in order.
func chunkName(dir string, i int) string {
	return fmt.Sprintf("%s/%s%06d", dir, chunkPrefix, i)
}

// chunkCommand receives chunk i on stdin into a fresh file in dir and renames
// it into place only when it matches sum. A session that drops leaves a short
// temporary file, never a chunk. dir is an uploadDirPattern path and sum hex.
func chunkCommand(dir string, i int, sum string) string {
	return fmt.Sprintf(`tmp=$(mktemp %s/%sXXXXXXXX) && { cat > "$tmp" && echo "%s  $tmp" | sha256sum -c --quiet --strict - && mv "$tmp" %s; } || { rm -f "$tmp"; exit 1; }`,
		dir, chunkTempPrefix, sum, chunkName(dir, i))
}

// assembleCommand concatenates the n chunks in dir into the archive, checks it
// against sum and only then removes the chunks. An archive already assembled
// and verified, by a session that dropped before it could report, is kept.
func assembleCommand(dir string, n int, sum string) string {
	parts := make([]string, n)
	for i := range parts {
		parts[i] = chunkName(dir, i)
	}
	archive := uploadPath(dir)
	check := fmt.Sprintf(`echo "%s  %s" | sha256sum -c --quiet --strict -`, sum, archive)
	return fmt.Sprintf(`{ %s 2>/dev/null || { cat %s > %s && %s; }; } && rm -f %s/%s*`,
		check, strings.Join(parts, " "), archive, check, dir, chunkPrefix)
}
