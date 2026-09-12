package rqlite

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// InstallAuthFile copies the cluster rqlite auth JSON into the instance data
// directory so rqlited can read it. The systemd unit sets InaccessiblePaths on
// secrets/, so -auth must not point at that tree. Empty or missing source is
// a start error: rqlited is not allowed to come up without credentials.
func InstallAuthFile(src, dataDir string) (dest string, err error) {
	if strings.TrimSpace(src) == "" {
		return "", fmt.Errorf("rqlite auth file is required")
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return "", fmt.Errorf("read rqlite auth file %s: %w", src, err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return "", fmt.Errorf("rqlite auth file %s is empty", src)
	}
	if err := os.MkdirAll(dataDir, 0750); err != nil {
		return "", fmt.Errorf("create rqlite data dir %s: %w", dataDir, err)
	}
	dest = filepath.Join(dataDir, "rqlite-auth.json")
	if err := os.WriteFile(dest, data, 0600); err != nil {
		return "", fmt.Errorf("write rqlite auth file %s: %w", dest, err)
	}
	return dest, nil
}
