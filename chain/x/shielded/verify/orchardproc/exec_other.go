//go:build !linux

package orchardproc

import "os"

// execPathOf is the path the process is started from. Only Linux can start a process from an open
// file (through /proc), so elsewhere, which means a developer's machine and never a chain node,
// the file is started by name and a swap of the file between the hash and the start is not
// prevented. Production nodes are Linux.
func execPathOf(_ *os.File, path string) string { return path }
