// Package netregistry is the list of networks the CLI knows: a manifest names a
// network's chain, the digest of its genesis, the seeds to join through and the
// release root its software is verified against. A cluster-only network has no
// chain: its manifest names only where its releases come from.
//
// A network is trusted because its manifest was built into the CLI the operator
// installed, or because the operator confirmed a manifest they added by URL.
// Nothing a manifest points at is trusted on its own: the genesis and the
// release root are fetched or read, then checked against the digests the
// manifest carries.
package netregistry

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"regexp"
	"strings"

	"github.com/Orama-Network/orama/core/pkg/netclass"
	"github.com/Orama-Network/orama/core/pkg/tornet"
)

// File names of a published network, the same in networks/<name>/, in the
// embedded registry, on the website under /networks/<name>/ and in a custom
// network's store.
const (
	ManifestFile    = "manifest.json"
	GenesisFile     = "genesis.json"
	ReleaseRootFile = "release-root.json"
	// TorNetworkFile is the network's Tor network file (pkg/tornet), published beside the
	// manifest only when the manifest pins it.
	TorNetworkFile = "tor-network.json"
)

// Channels a manifest may name: the nightly feed, the main line, and a dev
// branch as dev/<branch>.
const (
	ChannelNightly   = "nightly"
	ChannelMain      = "main"
	channelDevPrefix = "dev/"
)

// maxManifestBytes bounds a manifest. A real one is under 2 KiB; the bound keeps
// a hostile URL from filling memory before validation runs.
const maxManifestBytes = 64 * 1024

var (
	nameRE      = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)
	sha256HexRE = regexp.MustCompile(`^[0-9a-f]{64}$`)
	semverRE    = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
	devBranchRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
	hostLabelRE = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)
)

// ErrGenesisMismatch and ErrRootMismatch say a file is not the one the manifest
// pins. Callers match them to refuse to continue.
var (
	ErrGenesisMismatch = errors.New("genesis does not match the manifest")
	ErrRootMismatch    = errors.New("release root does not match the manifest")
	// ErrTorNetworkMismatch says a Tor network file is not the one the manifest pins.
	ErrTorNetworkMismatch = errors.New("Tor network file does not match the manifest")
)

// ErrNotCreated says a network is announced: its manifest is published, with the
// chain id it will have, but nobody has created the chain yet, so there is no
// genesis to join. Callers match it to refuse a join.
var ErrNotCreated = errors.New("the network has not been created yet")

// notCreatedError is ErrNotCreated, naming the network and the command its
// creator runs.
type notCreatedError struct{ name string }

func (e *notCreatedError) Error() string {
	return fmt.Sprintf("%s has not been created yet; its creator runs orama setup --create-network %s", e.name, e.name)
}

func (e *notCreatedError) Is(target error) bool { return target == ErrNotCreated }

// ErrClusterOnly says a network is cluster-only: its manifest names a release
// channel and no chain, so there is no genesis to join and no chain id to sign
// for. Callers match it to refuse what needs a chain.
var ErrClusterOnly = errors.New("the network is cluster-only: it runs no chain")

// clusterOnlyError is ErrClusterOnly, naming the network.
type clusterOnlyError struct{ name string }

func (e *clusterOnlyError) Error() string {
	return fmt.Sprintf("%s is a cluster-only network: it runs no chain, only clusters that follow its release channel", e.name)
}

func (e *clusterOnlyError) Is(target error) bool { return target == ErrClusterOnly }

// Manifest is one network's published description. It is one of three kinds:
//
//   - created: a chain id and the genesis_sha256 of that chain;
//   - announced: a chain id and an empty genesis_sha256 — the network's name, the
//     chain id it will have, where its releases come from and the root they are
//     verified against, published before the chain exists. It is valid but not
//     joinable (see Announced);
//   - cluster-only: an empty chain_id, and so no genesis, seeds, faucet or Tor
//     network file: only the release repository, channel, minimum version and
//     release root its clusters follow (see ClusterOnly).
type Manifest struct {
	// Name is the network's short name: testnet, or an operator's own.
	Name string `json:"name"`
	// ChainID is the chain's id. A reset of the chain gets a new one. Empty for a
	// cluster-only network.
	ChainID string `json:"chain_id"`
	// GenesisSHA256 is the SHA-256 of the genesis.json that goes with ChainID,
	// in lowercase hex. Empty while the network is only announced.
	GenesisSHA256 string `json:"genesis_sha256"`
	// Seeds are DNS names of nodes a joiner reaches the chain through. Never IPs:
	// a seed is replaced without a new manifest. An announcement may carry none;
	// its creator then gets the default seeds.
	Seeds []string `json:"seeds"`
	// Channel is the release channel the network's nodes run.
	Channel string `json:"channel"`
	// MinVersion is the oldest orama version that may join, as X.Y.Z.
	MinVersion string `json:"min_version"`
	// ReleaseRepo is the https base URL of the network's release repository.
	ReleaseRepo string `json:"release_repo"`
	// ReleaseRootSHA256 is the SHA-256 of release-root.json, the TUF root the
	// network's releases are verified against, in lowercase hex.
	ReleaseRootSHA256 string `json:"release_root_sha256"`
	// Faucet says the network funds new operators from a faucet: a gateway of the network that
	// holds a faucet signer answers POST /v1/chain/faucet on the seeds.
	Faucet bool `json:"faucet"`
	// TorNetworkSHA256 is the SHA-256 of tor-network.json, the private Orama Tor network a
	// relay of this network joins, in lowercase hex. Empty when the network pins none: its
	// relays are then given the file by hand (orama setup --tor-network).
	TorNetworkSHA256 string `json:"tor_network_sha256,omitempty"`
}

// ParseManifest reads a manifest strictly: a field it does not know, a second
// document after the first, or any invalid value is an error.
func ParseManifest(data []byte) (*Manifest, error) {
	if len(data) > maxManifestBytes {
		return nil, fmt.Errorf("manifest is %d bytes; a manifest is under %d", len(data), maxManifestBytes)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var m Manifest
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, errors.New("parse manifest: trailing data after the manifest object")
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return &m, nil
}

// ClusterOnly reports whether the network is cluster-only: its manifest names no
// chain.
func (m *Manifest) ClusterOnly() bool { return m.ChainID == "" }

// Announced reports whether the network is announced and not yet created: its
// manifest names a chain and pins no genesis.
func (m *Manifest) Announced() bool { return !m.ClusterOnly() && m.GenesisSHA256 == "" }

// Created reports whether the network's chain exists: its manifest pins a genesis.
func (m *Manifest) Created() bool { return m.GenesisSHA256 != "" }

// CheckCreated returns an error matching ErrNotCreated while the network is only
// announced, and one matching ErrClusterOnly for a cluster-only network, which
// has no chain to create.
func (m *Manifest) CheckCreated() error {
	switch {
	case m.ClusterOnly():
		return &clusterOnlyError{name: m.Name}
	case m.Announced():
		return &notCreatedError{name: m.Name}
	}
	return nil
}

// Validate checks every field. The first problem is returned, naming the field.
// An announcement passes without a genesis digest, and may carry no seeds. A
// cluster-only manifest has no chain id, and then carries nothing of a chain.
func (m *Manifest) Validate() error {
	if !nameRE.MatchString(m.Name) {
		return fmt.Errorf("manifest name %q: use 1-32 characters of a-z, 0-9 and -, starting with a letter", m.Name)
	}
	if err := m.validateChain(); err != nil {
		return err
	}
	if err := validateChannel(m.Channel); err != nil {
		return err
	}
	if !semverRE.MatchString(m.MinVersion) {
		return fmt.Errorf("manifest min_version %q is not X.Y.Z", m.MinVersion)
	}
	if err := validateHTTPSURL("release_repo", m.ReleaseRepo); err != nil {
		return err
	}
	if !sha256HexRE.MatchString(m.ReleaseRootSHA256) {
		return fmt.Errorf("manifest release_root_sha256 %q is not 64 lowercase hex characters", m.ReleaseRootSHA256)
	}
	if m.TorNetworkSHA256 != "" && !sha256HexRE.MatchString(m.TorNetworkSHA256) {
		return fmt.Errorf("manifest tor_network_sha256 %q is not 64 lowercase hex characters (or leave it out)", m.TorNetworkSHA256)
	}
	return nil
}

// validateChain checks the chain fields: those of a created or announced
// network, or their absence on a cluster-only one.
func (m *Manifest) validateChain() error {
	if m.ClusterOnly() {
		return m.validateNoChain()
	}
	if !netclass.ValidChainID(m.ChainID) {
		return fmt.Errorf("manifest chain_id %q: use 1-48 characters of a-z, 0-9 and -, starting and ending with a letter or digit (empty makes a cluster-only network)", m.ChainID)
	}
	if m.Created() && !sha256HexRE.MatchString(m.GenesisSHA256) {
		return fmt.Errorf("manifest genesis_sha256 %q is not 64 lowercase hex characters (empty announces a network that is not created yet)", m.GenesisSHA256)
	}
	return validateSeeds(m.Seeds, m.Announced())
}

// validateNoChain refuses a cluster-only manifest that carries a field of a
// chain: each needs the chain the manifest does not name.
func (m *Manifest) validateNoChain() error {
	switch {
	case m.GenesisSHA256 != "":
		return errors.New("manifest genesis_sha256 is set but chain_id is empty: a genesis is a chain's, and a cluster-only network has none")
	case len(m.Seeds) > 0:
		return errors.New("manifest seeds are set but chain_id is empty: seeds are how a joiner reaches the chain, and a cluster-only network has none")
	case m.Faucet:
		return errors.New("manifest faucet is set but chain_id is empty: a faucet funds accounts on the chain, and a cluster-only network has none")
	case m.TorNetworkSHA256 != "":
		return errors.New("manifest tor_network_sha256 is set but chain_id is empty: a relay registers on the chain, and a cluster-only network has none")
	}
	return nil
}

func validateSeeds(seeds []string, optional bool) error {
	if len(seeds) == 0 && !optional {
		return errors.New("manifest seeds: at least one seed is required")
	}
	seen := map[string]bool{}
	for _, s := range seeds {
		if err := validateSeedName(s); err != nil {
			return err
		}
		if seen[s] {
			return fmt.Errorf("manifest seeds: %q is listed twice", s)
		}
		seen[s] = true
	}
	return nil
}

func validateSeedName(s string) error {
	if net.ParseIP(s) != nil {
		return fmt.Errorf("manifest seed %q is an IP address; seeds are DNS names", s)
	}
	labels := strings.Split(s, ".")
	if len(s) > 253 || len(labels) < 2 {
		return fmt.Errorf("manifest seed %q is not a DNS name with at least two labels", s)
	}
	for _, l := range labels {
		if !hostLabelRE.MatchString(l) {
			return fmt.Errorf("manifest seed %q: label %q is not a lowercase DNS label", s, l)
		}
	}
	return nil
}

func validateChannel(c string) error {
	if c == ChannelNightly || c == ChannelMain {
		return nil
	}
	if branch, ok := strings.CutPrefix(c, channelDevPrefix); ok && devBranchRE.MatchString(branch) {
		return nil
	}
	return fmt.Errorf("manifest channel %q is not %s, %s or %s<branch>", c, ChannelNightly, ChannelMain, channelDevPrefix)
}

func validateHTTPSURL(field, raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("manifest %s %q: %w", field, raw, err)
	}
	switch {
	case u.Scheme != "https":
		return fmt.Errorf("manifest %s %q must be an https:// URL", field, raw)
	case u.Hostname() == "":
		return fmt.Errorf("manifest %s %q has no host", field, raw)
	case u.User != nil:
		return fmt.Errorf("manifest %s %q must not carry a user or password", field, raw)
	case u.RawQuery != "" || u.Fragment != "":
		return fmt.Errorf("manifest %s %q must not carry a query or fragment", field, raw)
	}
	return nil
}

// Digest returns the lowercase hex SHA-256 of data.
func Digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// VerifyGenesis checks genesis against the manifest's genesis_sha256. An
// announced network has none to check against: ErrNotCreated; a cluster-only one
// has no chain: ErrClusterOnly.
func (m *Manifest) VerifyGenesis(genesis []byte) error {
	if err := m.CheckCreated(); err != nil {
		return err
	}
	if got := Digest(genesis); got != m.GenesisSHA256 {
		return fmt.Errorf("%w: network %s (chain %s) pins sha256 %s, the file is %s",
			ErrGenesisMismatch, m.Name, m.ChainID, m.GenesisSHA256, got)
	}
	return nil
}

// VerifyRoot checks root against the manifest's release_root_sha256.
func (m *Manifest) VerifyRoot(root []byte) error {
	if got := Digest(root); got != m.ReleaseRootSHA256 {
		return fmt.Errorf("%w: network %s pins sha256 %s, the file is %s",
			ErrRootMismatch, m.Name, m.ReleaseRootSHA256, got)
	}
	return nil
}

// VerifyTorNetwork checks the Tor network file against the manifest's tor_network_sha256, and
// that it is a network file a relay can join (pkg/tornet).
func (m *Manifest) VerifyTorNetwork(file []byte) error {
	if m.TorNetworkSHA256 == "" {
		return fmt.Errorf("network %s pins no Tor network file", m.Name)
	}
	if got := Digest(file); got != m.TorNetworkSHA256 {
		return fmt.Errorf("%w: network %s pins sha256 %s, the file is %s",
			ErrTorNetworkMismatch, m.Name, m.TorNetworkSHA256, got)
	}
	if _, err := tornet.ParseNetwork(file); err != nil {
		return fmt.Errorf("network %s pins a Tor network file a relay cannot use: %w", m.Name, err)
	}
	return nil
}

// Marshal renders the manifest the way it is published: indented, one field
// per line, a trailing newline, so two publishes of the same facts are
// byte-identical.
func (m *Manifest) Marshal() ([]byte, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}
	doc := *m
	if doc.Seeds == nil {
		doc.Seeds = []string{}
	}
	out, err := json.MarshalIndent(&doc, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal manifest: %w", err)
	}
	return append(out, '\n'), nil
}
