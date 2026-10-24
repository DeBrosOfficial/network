// Package hostpin holds the CLI's own SSH sessions to the host keys
// 'orama node setup' pinned in ~/.orama/known_hosts. A host with no pinned key
// is refused before any SSH key is fetched: the CLI never trusts a host on
// first use.
package hostpin

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/Orama-Network/orama/core/cmd/orama/internal/clierr"
	"github.com/Orama-Network/orama/core/pkg/inspector"
)

const (
	// pinnedHostsDir and pinnedHostsFile are ~/.orama/known_hosts, the pin
	// 'orama node setup' keeps.
	pinnedHostsDir  = ".orama"
	pinnedHostsFile = "known_hosts"
)

// Require points node at the host-key pin setup keeps and refuses a host with
// no entry in it. ssh would refuse too under strict checking, but only after
// the wallet key was fetched, and with a message that does not say how to pin
// the host. A node that already names its own known_hosts keeps it.
func Require(node *inspector.Node) error {
	if node.KnownHostsFile == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("find the home directory for %s: %w", pinnedHostsFile, err)
		}
		node.KnownHostsFile = filepath.Join(home, pinnedHostsDir, pinnedHostsFile)
	}
	if _, err := os.Stat(node.KnownHostsFile); err != nil {
		return unpinned(node, fmt.Errorf("%s cannot be read: %w", node.KnownHostsFile, err))
	}
	keygen, err := exec.LookPath("ssh-keygen")
	if err != nil {
		return fmt.Errorf("ssh-keygen not found in PATH, needed to look %s up in %s: %w", node.Host, node.KnownHostsFile, err)
	}
	out, err := exec.Command(keygen, "-F", node.Host, "-f", node.KnownHostsFile).Output()
	var exit *exec.ExitError
	switch {
	case err == nil && len(out) > 0:
		return nil
	case err == nil || errors.As(err, &exit) && exit.ExitCode() == 1:
		return unpinned(node, fmt.Errorf("it has no entry for %s", node.Host))
	default:
		return fmt.Errorf("look up %s in %s: %w", node.Host, node.KnownHostsFile, err)
	}
}

// RequireAll is Require for every node, refusing at the first unpinned one.
func RequireAll(nodes []inspector.Node) error {
	for i := range nodes {
		if err := Require(&nodes[i]); err != nil {
			return err
		}
	}
	return nil
}

// unpinned is the refusal for a host whose key was never pinned.
func unpinned(node *inspector.Node, why error) error {
	return clierr.Failure("Host key verification failed: %s is not pinned (%v). orama never trusts a host on first use; "+
		"pin its key with 'orama node setup --ip %s ...', which writes it to %s",
		node.Host, why, node.Host, node.KnownHostsFile)
}
