package netregistry

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ClusterOnlyInput is a network that runs no chain: clusters whose nodes follow
// one release channel, verified against one release root.
type ClusterOnlyInput struct {
	// Dir is the repository's networks directory.
	Dir string
	// Name is the network's name.
	Name string
	// ReleaseRoot is release-root.json; the manifest pins its digest.
	ReleaseRoot []byte
	// Channel, MinVersion and ReleaseRepo say where the network's releases come from.
	Channel     string
	MinVersion  string
	ReleaseRepo string
}

// ErrHasChain says a network cannot be made cluster-only because its manifest
// names a chain.
var ErrHasChain = errors.New("the network names a chain")

// PublishClusterOnly writes networks/<name>/ for a cluster-only network:
// release-root.json and, last, manifest.json with no chain id. It replaces an
// earlier cluster-only manifest of the same name. It refuses a network whose
// manifest names a chain, announced or created: writing over it would drop the
// chain id its joiners and signers are pinned to.
func PublishClusterOnly(in ClusterOnlyInput) (*Manifest, error) {
	dir := filepath.Join(in.Dir, in.Name)
	prev, err := readPublished(dir)
	if err != nil {
		return nil, err
	}
	if prev != nil && !prev.ClusterOnly() {
		return nil, fmt.Errorf("%w: %s is published with chain %s; a network with a chain is changed with orama maint network announce or publish",
			ErrHasChain, in.Name, prev.ChainID)
	}
	if !json.Valid(in.ReleaseRoot) {
		return nil, errors.New("the release root is not JSON")
	}
	m := &Manifest{
		Name: in.Name, Channel: in.Channel, MinVersion: in.MinVersion,
		ReleaseRepo: in.ReleaseRepo, ReleaseRootSHA256: Digest(in.ReleaseRoot),
	}
	manifest, err := m.Marshal()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create %s: %w", dir, err)
	}
	// The manifest goes last, so a reader that finds it finds the root it pins.
	for _, f := range []publishedFile{{ReleaseRootFile, in.ReleaseRoot}, {ManifestFile, manifest}} {
		if err := writeFileAtomic(filepath.Join(dir, f.name), f.data); err != nil {
			return nil, err
		}
	}
	return m, nil
}
