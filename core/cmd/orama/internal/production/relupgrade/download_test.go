package relupgrade

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestRun_byDefaultEveryNodeDownloadsTheReleaseAndNothingIsDownloadedHere(t *testing.T) {
	f := newFleet(map[string]string{"10.0.0.1": "0.3.0", "10.0.0.2": "0.3.0", "10.0.0.3": "0.3.0"})
	r, out := f.runner(Options{Yes: true})

	if err := r.run(context.Background()); err != nil {
		t.Fatalf("run: %v\n%s", err, out)
	}

	if len(f.fetched) != 0 {
		t.Errorf("the archive was downloaded on this machine for %v", f.fetched)
	}
	for _, h := range []string{"10.0.0.1", "10.0.0.2", "10.0.0.3"} {
		rel := f.files[h]
		if rel.Archive != "" || rel.Download == nil {
			t.Fatalf("%s was given %+v, want a download and no upload", h, rel)
		}
		if rel.Download.URL != "https://releases.example/targets/nightly/orama-amd64" || rel.Download.Length != 300<<20 ||
			rel.Download.SHA256 != strings.Repeat("b", 64) || rel.MetadataDir != "/w/metadata" || string(rel.Root) != `{"root":true}` {
			t.Errorf("%s was given %+v: want the signed URL, length and digest, the metadata and the root", h, rel)
		}
	}
	if f.accepted != 1 {
		t.Errorf("the rollback record was raised %d times, want once after the nodes verified the release", f.accepted)
	}
	if !strings.Contains(out.String(), "each node downloads it from https://releases.example itself") {
		t.Errorf("the output does not say the nodes download the release:\n%s", out)
	}
}

func TestRun_uploadReleaseDownloadsHereAndUploadsToEveryNode(t *testing.T) {
	f := newFleet(map[string]string{"10.0.0.1": "0.3.0", "10.0.0.2": "0.3.0", "10.0.0.3": "0.3.0"})
	r, out := f.runner(Options{Yes: true, UploadRelease: true})

	if err := r.run(context.Background()); err != nil {
		t.Fatalf("run: %v\n%s", err, out)
	}

	if len(f.resolved) != 0 || len(f.fetched) != 1 {
		t.Errorf("resolved %v and fetched %v, want one download here", f.resolved, f.fetched)
	}
	for h, rel := range f.files {
		if rel.Archive != "/w/a.tar.gz" || rel.Download != nil {
			t.Errorf("%s was given %+v, want the archive uploaded", h, rel)
		}
	}
	if f.accepted != 0 {
		t.Errorf("an upload raised the record again (%d)", f.accepted)
	}
}

func TestRun_aDryRunWithUploadReleaseDoesNotDownloadTheArchive(t *testing.T) {
	f := newFleet(map[string]string{"10.0.0.1": "0.3.0", "10.0.0.2": "0.3.0", "10.0.0.3": "0.3.0"})
	r, _ := f.runner(Options{DryRun: true, UploadRelease: true})

	if err := r.run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}

	if len(f.fetched) != 0 || len(f.resolved) != 1 {
		t.Errorf("fetched %v and resolved %v: a dry run reads the metadata only", f.fetched, f.resolved)
	}
}

func TestRun_aNodeThatRefusesLeavesTheRollbackRecordAlone(t *testing.T) {
	f := newFleet(map[string]string{"10.0.0.1": "0.3.0", "10.0.0.2": "0.3.0", "10.0.0.3": "0.3.0"})
	f.stageErr["10.0.0.1"] = errors.New("the download has sha256 c, the signed metadata says b")
	r, _ := f.runner(Options{Yes: true})

	if err := r.run(context.Background()); err == nil {
		t.Fatal("a node's refusal was not reported")
	}

	if f.accepted != 0 || len(f.rolled) != 0 {
		t.Errorf("accepted %d and rolled %v after a node refused the archive", f.accepted, f.rolled)
	}
}

func TestRun_aNodeThatCannotReachTheRepositoryIsToldAboutUploadRelease(t *testing.T) {
	for name, c := range map[string]struct {
		err  string
		hint bool
	}{
		"a host that does not resolve":  {"run on 10.0.0.2: exit status 6 (stderr: curl: (6) Could not resolve host: releases.example)", true},
		"a repository that answers 404": {"run on 10.0.0.2: exit status 22 (stderr: curl: (22) The requested URL returned error: 404)", false},
	} {
		f := newFleet(map[string]string{"10.0.0.1": "0.3.0", "10.0.0.2": "0.3.0", "10.0.0.3": "0.3.0"})
		f.stageErr["10.0.0.2"] = errors.New(c.err)
		r, _ := f.runner(Options{Yes: true})

		err := r.run(context.Background())

		if err == nil {
			t.Fatalf("%s: no error", name)
		}
		if got := strings.Contains(err.Error(), "--upload-release"); got != c.hint {
			t.Errorf("%s: the --upload-release hint shown = %v, want %v: %v", name, got, c.hint, err)
		}
	}
}
