package install

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/Orama-Network/orama/core/pkg/clustertrust"
	"github.com/Orama-Network/orama/core/pkg/constants"
	"github.com/Orama-Network/orama/core/pkg/rootfs"
)

// A staging cluster gets the staging roots, readable by every deployment's
// user; moving it to production removes them again.
func TestInstallClusterTrust_followsTheACMEDirectory(t *testing.T) {
	etc := t.TempDir()
	path := filepath.Join(etc, "orama", "cluster-ca.pem")
	root := rootfs.At(etc)

	known, err := installClusterTrust(root, path, constants.LetsEncryptStagingACME)
	if err != nil || !known {
		t.Fatalf("staging: known %v, err %v", known, err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want, err := clustertrust.StagingRoots()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("the cluster trust file is not the staging roots")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != clusterTrustMode {
		t.Fatalf("mode %v, want %v", info.Mode().Perm(), fs.FileMode(clusterTrustMode))
	}

	for _, dir := range []string{constants.LetsEncryptProductionACME, ""} {
		known, err = installClusterTrust(root, path, dir)
		if err != nil || !known {
			t.Fatalf("%q: known %v, err %v", dir, known, err)
		}
		if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("%q: the staging roots are still trusted (%v)", dir, err)
		}
	}
}

// An operator's own ACME CA is not something the node can vouch for: nothing
// is trusted beyond the system roots, and the caller is told.
func TestInstallClusterTrust_unknownDirectoryTrustsNothingExtra(t *testing.T) {
	etc := t.TempDir()
	path := filepath.Join(etc, "orama", "cluster-ca.pem")
	known, err := installClusterTrust(rootfs.At(etc), path, "https://ca.example.com/acme/directory")
	if err != nil || known {
		t.Fatalf("known %v, err %v; want unknown, no error", known, err)
	}
	if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("a trust file was written for an unknown CA (%v)", err)
	}
}
