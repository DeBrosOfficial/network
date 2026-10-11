package report

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestParseChainUpgrade(t *testing.T) {
	cases := []struct {
		name, body string
		want       *ChainUpgrade
		wantErr    bool
	}{
		{"scheduled", `{"plan":{"name":"v0-4-0","height":"1500","info":""}}`, &ChainUpgrade{Name: "v0-4-0", Height: 1500}, false},
		{"none: null plan", `{"plan":null}`, nil, false},
		{"none: empty object", `{}`, nil, false},
		{"none: empty name", `{"plan":{"name":"","height":"0"}}`, nil, false},
		{"malformed", `<html>`, nil, true},
		{"height that is not a number", `{"plan":{"name":"x","height":"soon"}}`, nil, true},
	}
	for _, c := range cases {
		got, err := parseChainUpgrade([]byte(c.body))
		if (err != nil) != c.wantErr {
			t.Errorf("%s: err = %v", c.name, err)
			continue
		}
		switch {
		case got == nil && c.want == nil:
		case got == nil || c.want == nil || *got != *c.want:
			t.Errorf("%s: got %+v, want %+v", c.name, got, c.want)
		}
	}
}

func stageFile(t *testing.T, home, name, file string) {
	t.Helper()
	dir := filepath.Join(home, "cosmovisor", "upgrades", name, "bin")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, file), []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestUpgradeStaged(t *testing.T) {
	home := t.TempDir()
	if staged, problem := upgradeStaged(home, "v0-4-0"); staged || problem != "" {
		t.Fatalf("nothing staged: %v %q", staged, problem)
	}
	stageFile(t, home, "v0-4-0", "oramad")
	if staged, problem := upgradeStaged(home, "v0-4-0"); staged || problem != "" {
		t.Fatalf("the verifier is missing, so cosmovisor would start a binary without it: %v %q", staged, problem)
	}
	stageFile(t, home, "v0-4-0", "orama-orchard-verifier")
	if staged, problem := upgradeStaged(home, "v0-4-0"); !staged || problem != "" {
		t.Fatalf("both staged: %v %q", staged, problem)
	}
	if staged, _ := upgradeStaged(home, "v0-5-0"); staged {
		t.Fatal("another plan's binary counted as staged")
	}
}

func TestUpgradeStaged_aNameCosmovisorCannotStageIsAProblem(t *testing.T) {
	staged, problem := upgradeStaged(t.TempDir(), "Bad/Name")
	if staged || problem == "" {
		t.Fatalf("got %v %q", staged, problem)
	}
}

func TestUpgradeStaged_aSymlinkIsNotAStagedBinary(t *testing.T) {
	home := t.TempDir()
	stageFile(t, home, "v1", "orama-orchard-verifier")
	bin := filepath.Join(home, "cosmovisor", "upgrades", "v1", "bin")
	if err := os.Symlink("/bin/sh", filepath.Join(bin, "oramad")); err != nil {
		t.Fatal(err)
	}
	if staged, problem := upgradeStaged(home, "v1"); staged || problem == "" {
		t.Fatalf("a symlink counted as staged: %v %q", staged, problem)
	}
}

func TestReadChainUpgrade(t *testing.T) {
	home := t.TempDir()
	old := chainHome
	chainHome = home
	t.Cleanup(func() { chainHome = old })
	stageFile(t, home, "v0-4-0", "oramad")
	stageFile(t, home, "v0-4-0", "orama-orchard-verifier")

	answer := `{"plan":{"name":"v0-4-0","height":"1500"}}`
	status := http.StatusOK
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != currentPlanPath {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(answer))
	}))
	t.Cleanup(srv.Close)

	r := &ChainReport{}
	readChainUpgrade(context.Background(), srv.URL, r)
	if r.Upgrade == nil || r.Upgrade.Name != "v0-4-0" || r.Upgrade.Height != 1500 || !r.Upgrade.Staged || r.UpgradeError != "" {
		t.Fatalf("staged plan: %+v / %q", r.Upgrade, r.UpgradeError)
	}

	answer = `{"plan":null}`
	readChainUpgrade(context.Background(), srv.URL, r)
	if r.Upgrade != nil || r.UpgradeError != "" {
		t.Fatalf("a plan that was withdrawn is still reported: %+v / %q", r.Upgrade, r.UpgradeError)
	}

	status, answer = http.StatusInternalServerError, `boom`
	readChainUpgrade(context.Background(), srv.URL, r)
	if r.Upgrade != nil || r.UpgradeError == "" {
		t.Fatalf("a failed query is not an absent plan: %+v / %q", r.Upgrade, r.UpgradeError)
	}
}
