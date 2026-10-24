package setup

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/Orama-Network/orama/core/pkg/netregistry"
	"github.com/Orama-Network/orama/core/pkg/statesync"
)

// registryNetworks resolves --network from the networks the CLI knows: the
// registry built into the binary and the ones the operator added by URL.
type registryNetworks struct {
	load   func() (*netregistry.Registry, error)
	active func() (string, error)
	client *http.Client
}

// Resolve returns the named network; with no name, the network the active
// environment runs on, else the registry's only network.
func (r registryNetworks) Resolve(_ context.Context, name string) (*netregistry.Network, error) {
	if strings.HasPrefix(name, "https://") || strings.HasPrefix(name, "http://") {
		return nil, fmt.Errorf("--network %s is a URL: add the network first with `orama network add %s`, which shows the digest of its release root and asks you to confirm it, then pass its name", name, name)
	}
	reg, err := r.load()
	if err != nil {
		return nil, fmt.Errorf("load the network registry: %w", err)
	}
	if name == "" {
		if name, err = r.defaultName(reg); err != nil {
			return nil, err
		}
	}
	n, err := reg.Get(name)
	if err != nil {
		return nil, fmt.Errorf("%w: add a network with `orama network add <manifest-url>`", err)
	}
	return n, nil
}

func (r registryNetworks) defaultName(reg *netregistry.Registry) (string, error) {
	if active, err := r.active(); err == nil && active != "" {
		if _, err := reg.Get(active); err == nil {
			return active, nil
		}
	}
	names := chainNetworks(reg)
	switch len(names) {
	case 0:
		return "", fmt.Errorf("this CLI knows no network that runs a chain: add one with `orama network add <manifest-url>`, or name a cluster-only one with --network")
	case 1:
		return names[0], nil
	}
	return "", fmt.Errorf("this CLI knows %d networks (%s): choose one with --network", len(names), strings.Join(names, ", "))
}

// chainNetworks are the names of the registry's networks that name a chain. A
// cluster-only network is never the default: its machines join it as cluster
// nodes alone, which the operator asks for by naming it.
func chainNetworks(reg *netregistry.Registry) []string {
	var names []string
	for _, name := range reg.Names() {
		if n, err := reg.Get(name); err == nil && !n.Manifest.ClusterOnly() {
			names = append(names, name)
		}
	}
	return names
}

// Genesis fetches the genesis and checks it against the manifest's digest.
func (r registryNetworks) Genesis(ctx context.Context, n *netregistry.Network) ([]byte, error) {
	return n.FetchGenesis(ctx, r.client)
}

// seedTrust finds the state-sync trust point through the network's seeds.
type seedTrust struct{ client statesync.Doer }

func (s seedTrust) TrustPoint(ctx context.Context, n *netregistry.Network) (*statesync.TrustPoint, error) {
	return statesync.Resolve(ctx, n.Manifest.Seeds, n.Manifest.ChainID, s.client)
}
