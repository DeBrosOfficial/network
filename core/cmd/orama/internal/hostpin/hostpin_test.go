package hostpin

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Orama-Network/orama/core/cmd/orama/internal/clierr"
	"github.com/Orama-Network/orama/core/pkg/inspector"
)

const testHostKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl"

// pinHome points HOME at a directory whose ~/.orama/known_hosts holds lines,
// or holds nothing at all when lines is nil.
func pinHome(t *testing.T, lines []string) string {
	t.Helper()
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		t.Skip("ssh-keygen is not installed")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	if lines == nil {
		return home
	}
	dir := filepath.Join(home, ".orama")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "known_hosts"), []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return home
}

// Bug: orama ssh, then orama maint inspect, ran with
// StrictHostKeyChecking=accept-new, which pins whatever key a host offers on
// first contact.
func TestRequire_refusesAHostWithNoPin(t *testing.T) {
	for name, lines := range map[string][]string{
		"no known_hosts file":      nil,
		"another host is pinned":   {"10.9.9.9 " + testHostKey},
		"an empty known_hosts":     {""},
		"only a comment is pinned": {"# 1.2.3.4 " + testHostKey},
	} {
		t.Run(name, func(t *testing.T) {
			home := pinHome(t, lines)
			node := inspector.Node{Host: "1.2.3.4", User: "root"}
			err := Require(&node)
			if err == nil {
				t.Fatal("a host with no pinned key was accepted")
			}
			if got := clierr.CodeOf(err); got != clierr.CodeFailure {
				t.Errorf("exit code %d, want %d", got, clierr.CodeFailure)
			}
			for _, want := range []string{"Host key verification failed", "1.2.3.4", "orama node setup", filepath.Join(home, ".orama", "known_hosts")} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("the refusal lacks %q: %v", want, err)
				}
			}
			if _, statErr := os.Stat(filepath.Join(home, ".ssh", "known_hosts")); statErr == nil {
				t.Error("a refused host was written to ~/.ssh/known_hosts")
			}
		})
	}
}

func TestRequire_acceptsAPinnedHost(t *testing.T) {
	home := pinHome(t, []string{"10.9.9.9 " + testHostKey, "1.2.3.4 " + testHostKey})
	node := inspector.Node{Host: "1.2.3.4", User: "root"}
	if err := Require(&node); err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, ".orama", "known_hosts"); node.KnownHostsFile != want {
		t.Errorf("KnownHostsFile = %q, want %q", node.KnownHostsFile, want)
	}
}

// A node that already names its pinned file (as setup's does) keeps it.
func TestRequire_keepsTheNodesOwnPin(t *testing.T) {
	pinHome(t, nil)
	own := filepath.Join(t.TempDir(), "run_known_hosts")
	if err := os.WriteFile(own, []byte("1.2.3.4 "+testHostKey+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	node := inspector.Node{Host: "1.2.3.4", User: "root", KnownHostsFile: own}
	if err := Require(&node); err != nil {
		t.Fatal(err)
	}
	if node.KnownHostsFile != own {
		t.Errorf("KnownHostsFile replaced: %q", node.KnownHostsFile)
	}
}

// orama maint inspect holds every node it collects from to the pin: one
// unpinned node refuses the run before any SSH key is fetched.
func TestRequireAll_refusesWhenOneNodeIsUnpinned(t *testing.T) {
	home := pinHome(t, []string{"1.2.3.4 " + testHostKey})
	nodes := []inspector.Node{{Host: "1.2.3.4", User: "root"}, {Host: "5.6.7.8", User: "root"}}
	err := RequireAll(nodes)
	if err == nil || !strings.Contains(err.Error(), "5.6.7.8 is not pinned") {
		t.Fatalf("err = %v, want 5.6.7.8 refused as unpinned", err)
	}
	if want := filepath.Join(home, ".orama", "known_hosts"); nodes[0].KnownHostsFile != want {
		t.Errorf("the pinned node's KnownHostsFile = %q, want %q", nodes[0].KnownHostsFile, want)
	}
}

func TestRequireAll_pointsEveryNodeAtThePin(t *testing.T) {
	home := pinHome(t, []string{"1.2.3.4 " + testHostKey, "5.6.7.8 " + testHostKey})
	nodes := []inspector.Node{{Host: "1.2.3.4", User: "root"}, {Host: "5.6.7.8", User: "root"}}
	if err := RequireAll(nodes); err != nil {
		t.Fatal(err)
	}
	for _, n := range nodes {
		opts := strings.Join(n.HostKeyOptions(), " ")
		if !strings.Contains(opts, "StrictHostKeyChecking=yes") || !strings.Contains(opts, filepath.Join(home, ".orama", "known_hosts")) {
			t.Errorf("%s host key options %q are not strict against the pin", n.Host, opts)
		}
	}
}
