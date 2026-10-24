package sshcmd

import (
	"slices"
	"strings"
	"testing"

	"github.com/Orama-Network/orama/core/pkg/inspector"
)

func TestSSHArgs_checksTheHostKeyStrictlyAgainstThePin(t *testing.T) {
	node := inspector.Node{Host: "1.2.3.4", User: "root", SSHKey: "/tmp/key", KnownHostsFile: "/h/.orama/known_hosts"}
	args := sshArgs(node, "uptime")
	for _, want := range []string{"StrictHostKeyChecking=yes", "UserKnownHostsFile=/h/.orama/known_hosts", "IdentitiesOnly=yes", "root@1.2.3.4"} {
		if !slices.Contains(args, want) {
			t.Errorf("args %q lack %q", args, want)
		}
	}
	for _, a := range args {
		if strings.Contains(a, "accept-new") || strings.Contains(a, "StrictHostKeyChecking=no") {
			t.Errorf("args %q trust a host on first use", args)
		}
	}
	if args[len(args)-1] != "uptime" {
		t.Errorf("the remote command is not last: %q", args)
	}
	if bare := sshArgs(node, ""); bare[len(bare)-1] != "root@1.2.3.4" {
		t.Errorf("an interactive session carries a command: %q", bare)
	}
}
