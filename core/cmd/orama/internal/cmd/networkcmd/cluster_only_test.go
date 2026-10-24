package networkcmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/Orama-Network/orama/core/cmd/orama/internal/clierr"
	"github.com/Orama-Network/orama/core/cmd/orama/internal/printer"
	"github.com/Orama-Network/orama/core/pkg/netregistry"
)

func clusterOnly(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := &cobra.Command{Use: "orama", SilenceUsage: true, SilenceErrors: true}
	printer.Register(root)
	maint := &cobra.Command{Use: "maint"}
	group := &cobra.Command{Use: "network"}
	group.AddCommand(newClusterOnlyCmd())
	maint.AddCommand(group)
	root.AddCommand(maint)
	var out strings.Builder
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(append([]string{"maint", "network", "cluster-only"}, args...))
	err := root.Execute()
	return out.String(), err
}

func clusterOnlyArgs(t *testing.T, tmp, name string) []string {
	t.Helper()
	return []string{
		"--dir", filepath.Join(tmp, "networks"), "--name", name,
		"--release-repo", "https://releases.example.org", "--release-root", writeFile(t, tmp, "root.json", pubRoot),
		"--channel", "nightly", "--min-version", "0.3.6",
	}
}

func TestClusterOnly_writesAManifestTheRegistryReads(t *testing.T) {
	tmp := t.TempDir()
	out, err := clusterOnly(t, clusterOnlyArgs(t, tmp, "devnet")...)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Wrote cluster-only network devnet", "channel nightly", "no chain", "sync-networks", "--network devnet"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	dir := filepath.Join(tmp, "networks", "devnet")
	data, err := os.ReadFile(filepath.Join(dir, netregistry.ManifestFile))
	if err != nil {
		t.Fatal(err)
	}
	m, err := netregistry.ParseManifest(data)
	if err != nil || !m.ClusterOnly() || m.Channel != "nightly" || m.MinVersion != "0.3.6" || m.ReleaseRootSHA256 != netregistry.Digest([]byte(pubRoot)) {
		t.Fatalf("manifest = %+v, %v", m, err)
	}
	if root, err := os.ReadFile(filepath.Join(dir, netregistry.ReleaseRootFile)); err != nil || string(root) != pubRoot {
		t.Errorf("release-root.json = %q, %v", root, err)
	}
}

func TestClusterOnly_aNetworkWithAChainIsAConflict(t *testing.T) {
	tmp := t.TempDir()
	if _, err := announce(t, announceArgs(t, tmp)...); err != nil {
		t.Fatal(err)
	}
	_, err := clusterOnly(t, clusterOnlyArgs(t, tmp, "pubnet")...)
	if clierr.CodeOf(err) != clierr.CodeConflict || !strings.Contains(err.Error(), "orama-pubnet-testnet-1") {
		t.Fatalf("error = %v (code %d), want a conflict naming the chain", err, clierr.CodeOf(err))
	}
}

func TestClusterOnly_refusals(t *testing.T) {
	tmp := t.TempDir()
	good := clusterOnlyArgs(t, tmp, "devnet")
	with := func(flag, value string) []string {
		return append(append([]string{}, good...), flag, value)
	}
	for name, args := range map[string][]string{
		"no flags":                 {},
		"a release root not found": with("--release-root", filepath.Join(tmp, "absent.json")),
		"a channel that is not published, stable": with("--channel", "stable"),
		"a release repo over http":                with("--release-repo", "http://releases.example.org"),
		"a min version that is not X.Y.Z":         with("--min-version", "v0.3"),
		"a chain flag":                            with("--chain-id", "orama-devnet-1"),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := clusterOnly(t, args...)
			if err == nil {
				t.Fatal("cluster-only accepted it")
			}
			if name != "no flags" && name != "a chain flag" && clierr.CodeOf(err) != clierr.CodeUsage {
				t.Errorf("code = %d, want a usage error: %v", clierr.CodeOf(err), err)
			}
		})
	}
}
