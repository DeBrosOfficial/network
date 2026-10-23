package push

import (
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// testChunkSize keeps the test archive small and still several chunks.
const testChunkSize = 64 << 10

// localNode runs upload commands with bash on this machine, the way a node's
// shell runs them, counting the bytes each chunk command was sent.
type localNode struct {
	sent  map[string]int64
	runs  int
	inner func(command string, stdin io.Reader) error
}

func newLocalNode() *localNode {
	n := &localNode{sent: map[string]int64{}}
	n.inner = func(command string, stdin io.Reader) error {
		cmd := exec.Command("bash", "-c", command)
		cmd.Stdin = stdin
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%w: %s", err, stderr.String())
		}
		return nil
	}
	return n
}

func (n *localNode) run(command string, stdin io.Reader) error {
	n.runs++
	counted := &countingReader{r: stdin}
	err := n.inner(command, counted)
	n.sent[command] += counted.n
	return err
}

type countingReader struct {
	r io.Reader
	n int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	k, err := c.r.Read(p)
	c.n += int64(k)
	return k, err
}

func testArchive(t *testing.T, size int) (string, []byte) {
	t.Helper()
	data := make([]byte, size)
	if _, err := rand.Read(data); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "orama.tar.gz")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path, data
}

func testUploader(run remoteRun) *uploader {
	return &uploader{run: run, pause: func() {}, progress: func(int64, int64) {}}
}

func assertUploaded(t *testing.T, dir string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(uploadPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("the uploaded archive differs from the local one (%d vs %d bytes)", len(got), len(want))
	}
	left, _ := filepath.Glob(filepath.Join(dir, "*"))
	if len(left) != 1 {
		t.Errorf("the upload directory holds more than the archive: %v", left)
	}
}

func TestUpload_isByteIdentical(t *testing.T) {
	path, data := testArchive(t, 5*testChunkSize+123)
	plan, err := planUpload(path, testChunkSize)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.chunks) != 6 || plan.size != int64(len(data)) {
		t.Fatalf("plan: %d chunks, %d bytes", len(plan.chunks), plan.size)
	}
	dir := t.TempDir()
	if err := testUploader(newLocalNode().run).upload(plan, dir); err != nil {
		t.Fatal(err)
	}
	assertUploaded(t, dir, data)
}

func TestUpload_aDroppedSessionResendsOnlyItsChunk(t *testing.T) {
	path, data := testArchive(t, 4*testChunkSize)
	plan, err := planUpload(path, testChunkSize)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	node := newLocalNode()
	inner := node.inner
	dropped := false
	dropAt := chunkCommand(dir, 2, plan.chunks[2].sum)
	node.inner = func(command string, stdin io.Reader) error {
		if command == dropAt && !dropped {
			// Half the chunk reaches the node, then the link resets.
			dropped = true
			_ = inner(command, io.LimitReader(stdin, testChunkSize/2))
			return fmt.Errorf("%w: connection reset", errTransport)
		}
		return inner(command, stdin)
	}
	if err := testUploader(node.run).upload(plan, dir); err != nil {
		t.Fatal(err)
	}
	assertUploaded(t, dir, data)
	for i, c := range plan.chunks {
		want := c.size
		if i == 2 {
			want = c.size + testChunkSize/2
		}
		if got := node.sent[chunkCommand(dir, i, c.sum)]; got != want {
			t.Errorf("chunk %d: %d bytes sent, want %d", i, got, want)
		}
	}
}

func TestUpload_anAssemblyWhoseReportWasLostIsKept(t *testing.T) {
	path, data := testArchive(t, 3*testChunkSize)
	plan, err := planUpload(path, testChunkSize)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	node := newLocalNode()
	inner := node.inner
	lost := false
	node.inner = func(command string, stdin io.Reader) error {
		err := inner(command, stdin)
		if strings.HasPrefix(command, "{ echo") && !lost {
			lost = true
			return fmt.Errorf("%w: connection reset", errTransport)
		}
		return err
	}
	if err := testUploader(node.run).upload(plan, dir); err != nil {
		t.Fatal(err)
	}
	assertUploaded(t, dir, data)
}

func TestUpload_givesUpAfterBoundedDrops(t *testing.T) {
	path, _ := testArchive(t, testChunkSize)
	plan, err := planUpload(path, testChunkSize)
	if err != nil {
		t.Fatal(err)
	}
	runs := 0
	err = testUploader(func(string, io.Reader) error {
		runs++
		return fmt.Errorf("%w: connection reset", errTransport)
	}).upload(plan, t.TempDir())
	if !errors.Is(err, errTransport) || runs != uploadAttempts {
		t.Fatalf("upload = %v after %d runs; want a transport error after %d", err, runs, uploadAttempts)
	}
}

func TestUpload_aFailingCommandIsNotResent(t *testing.T) {
	path, _ := testArchive(t, testChunkSize)
	plan, err := planUpload(path, testChunkSize)
	if err != nil {
		t.Fatal(err)
	}
	node := newLocalNode()
	// The upload directory does not exist: mktemp fails on the node.
	err = testUploader(node.run).upload(plan, filepath.Join(t.TempDir(), "absent"))
	if err == nil || errors.Is(err, errTransport) || node.runs != 1 {
		t.Fatalf("upload = %v after %d runs; want one failed run", err, node.runs)
	}
}

func TestUpload_aCorruptChunkIsNotPutInPlace(t *testing.T) {
	path, _ := testArchive(t, testChunkSize)
	plan, err := planUpload(path, testChunkSize)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	node := newLocalNode()
	inner := node.inner
	node.inner = func(command string, stdin io.Reader) error {
		return inner(command, io.MultiReader(stdin, strings.NewReader("x")))
	}
	if err := testUploader(node.run).upload(plan, dir); err == nil {
		t.Fatal("a chunk that does not match its sha256 was accepted")
	}
	if left, _ := filepath.Glob(filepath.Join(dir, "*")); len(left) != 0 {
		t.Errorf("a corrupt chunk was left in place: %v", left)
	}
}
