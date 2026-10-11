//go:build linux

package orchardproc

import (
	"fmt"
	"os"
)

// execPathOf is the path the process is started from: the open file itself, through /proc, which
// is how fexecve is built on Linux. The path is looked up in the child after the fork, where the
// file is still open, so no name on disk is resolved a second time and a swap of the file at path
// after the hash changes nothing. (A writer with write permission on the file's own inode could
// still change its bytes in that window; the installer makes the binary root's and read-only for
// the chain account.)
func execPathOf(f *os.File, _ string) string {
	return fmt.Sprintf("/proc/self/fd/%d", f.Fd())
}
