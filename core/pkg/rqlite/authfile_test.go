package rqlite

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstallAuthFile_missing(t *testing.T) {
	dir := t.TempDir()
	if _, err := InstallAuthFile("", dir); err == nil {
		t.Fatal("empty path must refuse to start")
	}
	if _, err := InstallAuthFile(filepath.Join(dir, "nope.json"), dir); err == nil {
		t.Fatal("missing file must refuse to start")
	}
	empty := filepath.Join(dir, "empty.json")
	if err := os.WriteFile(empty, []byte("  \n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := InstallAuthFile(empty, dir); err == nil {
		t.Fatal("empty file must refuse to start")
	}
}

func TestInstallAuthFile_copies(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.json")
	body := `[{"username":"orama","password":"deadbeef","perms":["all"]}]`
	if err := os.WriteFile(src, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	dataDir := filepath.Join(dir, "data")
	dest, err := InstallAuthFile(src, dataDir)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != body {
		t.Fatalf("copied %q", got)
	}
}
