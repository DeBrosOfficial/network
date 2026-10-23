package push

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// localScript runs a node-side script on this machine, as the node would.
func localScript(script string) error {
	cmd := exec.Command("bash", "-s")
	cmd.Stdin = strings.NewReader(script)
	return cmd.Run()
}

// stagedNode is a node that staged archive, verified, through stage-archive.
func stagedNode(t *testing.T, archive, signer string) string {
	t.Helper()
	base := installedNode(t)
	if err := stageArchive(trusting(base, signer).stageTarget, StageOptions{Archive: archive}); err != nil {
		t.Fatalf("stage: %v", err)
	}
	return base
}

func mustCheck(t *testing.T, archive string) string {
	t.Helper()
	list, ok, err := stagedCheck(archive)
	if err != nil || !ok {
		t.Fatalf("stagedCheck(%s) = ok %v, %v", archive, ok, err)
	}
	return list
}

func TestAlreadyStaged_theSameArchiveIsSkipped(t *testing.T) {
	key, addr := newSigner(t)
	archive := writeTarball(t, signedEntries(t, key, newBuild))
	base := stagedNode(t, archive, addr)
	if !alreadyStaged(localScript, base, mustCheck(t, archive)) {
		t.Fatal("a node that staged this archive was not recognized")
	}
}

func TestAlreadyStaged_anyDifferenceUploads(t *testing.T) {
	key, addr := newSigner(t)
	archive := writeTarball(t, signedEntries(t, key, newBuild))
	other := writeTarball(t, signedEntries(t, key, map[string]string{"bin/orama": "newer cli", "systemd/orama-namespace-x.service": "[Unit]\n"}))
	list := mustCheck(t, archive)

	if alreadyStaged(localScript, stagedNode(t, other, addr), list) {
		t.Error("a node that staged another archive was skipped")
	}
	if alreadyStaged(localScript, installedNode(t), list) {
		t.Error("a node on an older build was skipped")
	}

	tampered := stagedNode(t, archive, addr)
	if err := os.WriteFile(filepath.Join(tampered, "bin", "orama"), []byte("changed after staging"), 0o644); err != nil {
		t.Fatal(err)
	}
	if alreadyStaged(localScript, tampered, list) {
		t.Error("a node whose staged binary changed was skipped")
	}

	resigned := stagedNode(t, archive, addr)
	if err := os.WriteFile(filepath.Join(resigned, "manifest.sig"), []byte("0xother"), 0o644); err != nil {
		t.Fatal(err)
	}
	if alreadyStaged(localScript, resigned, list) {
		t.Error("a node with another signature over the manifest was skipped")
	}

	missing := stagedNode(t, archive, addr)
	if err := os.Remove(filepath.Join(missing, "systemd", "orama-namespace-x.service")); err != nil {
		t.Fatal(err)
	}
	if alreadyStaged(localScript, missing, list) {
		t.Error("a node missing a staged file was skipped")
	}

	if alreadyStaged(localScript, filepath.Join(t.TempDir(), "absent"), list) {
		t.Error("a node without /opt/orama was skipped")
	}
}

func TestAlreadyStaged_aNodeThatDoesNotAnswerUploads(t *testing.T) {
	key, _ := newSigner(t)
	archive := writeTarball(t, signedEntries(t, key, newBuild))
	unreachable := func(string) error { return errors.New("ssh: connect timed out") }
	if alreadyStaged(unreachable, "/opt/orama", mustCheck(t, archive)) {
		t.Fatal("a node that could not be asked was skipped")
	}
}

func TestStagedCheck_anUnsignedArchiveIsNeverSkipped(t *testing.T) {
	key, _ := newSigner(t)
	entries := signedEntries(t, key, newBuild)
	archive := writeTarball(t, entries[:len(entries)-1])
	if _, ok, err := stagedCheck(archive); err != nil || ok {
		t.Fatalf("stagedCheck(unsigned) = ok %v, %v; want not identifiable", ok, err)
	}
}
