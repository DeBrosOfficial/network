package install

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/Orama-Network/orama/core/pkg/clustertrust"
	"github.com/Orama-Network/orama/core/pkg/rootfs"
)

// A deployment calls its gateway at ORAMA_GATEWAY_URL over TLS, with whatever
// roots its runtime trusts. On a cluster issuing from Let's Encrypt staging
// those are not enough, so install and upgrade write the roots the cluster's
// ACME directory chains to (clustertrust.NodePath), and the gateway points
// every deployment's runtime at them (pkg/deployments/process). On a cluster
// whose certificates the system roots verify the file is removed, so a cluster
// moved from staging to production stops trusting staging.

// clusterTrustMode is world-readable: the file holds public certificates, and
// every deployment runs as a user of its own.
const clusterTrustMode = 0o644

// installClusterTrust writes, or removes, path under root for acmeDirectory
// (tls.acme_ca; "" is Let's Encrypt production). known is false for an ACME
// directory whose roots this release does not carry: then the file is removed
// and deployments trust the system roots only.
func installClusterTrust(root rootfs.Root, path, acmeDirectory string) (known bool, err error) {
	roots, known, err := clustertrust.ExtraRoots(acmeDirectory)
	if err != nil {
		return known, err
	}
	if roots == nil {
		if err := root.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return known, fmt.Errorf("remove %s: %w", path, err)
		}
		return known, nil
	}
	if err := root.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return known, fmt.Errorf("create %s: %w", filepath.Dir(path), err)
	}
	if err := root.WriteFile(path, roots, clusterTrustMode); err != nil {
		return known, fmt.Errorf("write %s: %w", path, err)
	}
	return known, nil
}
