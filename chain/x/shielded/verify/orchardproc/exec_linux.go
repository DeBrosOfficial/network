//go:build linux

package orchardproc

import (
	"os/exec"
)

// StartsFromCheckedFile reports that this platform starts the verifier from the file whose hash
// was checked. Chains other than localnets and scripted devnets refuse to start without it
// (chain/app CheckNodeCanVerify).
const StartsFromCheckedFile = true

// configure makes cmd start the open, hashed file itself, through /proc, which is how fexecve is
// built on Linux. The file is passed to the child as descriptor 3 and the path is looked up there,
// after the fork, so no name on disk is resolved a second time and a swap of the file at the path
// after the hash changes nothing. (Passing it as an extra file rather than relying on the parent's
// close-on-exec descriptor also lets a script run: its interpreter opens the path after the exec.)
// A writer with write permission on the file's own inode could still change its bytes between the
// hash and the start; the installer makes the binary root's and read-only for the chain account.
func (b *pinnedBinary) configure(cmd *exec.Cmd) {
	cmd.Path = "/proc/self/fd/3"
	cmd.ExtraFiles = append(cmd.ExtraFiles, b.file)
}
