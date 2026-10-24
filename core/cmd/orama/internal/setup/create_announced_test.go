package setup

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Orama-Network/orama/core/cmd/orama/internal/clierr"
	"github.com/Orama-Network/orama/core/pkg/netregistry"
)

const (
	announcedRepo = "https://releases.example"
	announcedRoot = `{"signed":{"_type":"announced-root"}}`
)

func (h *createHarness) announce(faucet bool) {
	h.deps.Networks = fakeNetworks{w: h.w, announced: true, faucet: faucet}
}

func TestRunCreate_anAnnouncedNetworkNeedsNoChainIDNoRootAndNoRepo(t *testing.T) {
	h := newCreateHarness(t)
	h.announce(true)
	opts := Options{IPs: fiveIPs, Yes: true, StorageGB: 10, Create: &CreateOptions{Name: "testnet", PublishDir: h.publishDir, TorAuthoritiesSet: true}}

	res := h.mustCreate(t, opts)

	m := res.Created.Manifest
	if m.ChainID != testChainID || m.ReleaseRepo != announcedRepo || m.Channel != "nightly" || m.MinVersion != "0.3.0" || !m.Faucet {
		t.Errorf("manifest = %+v: the chain id, repository, channel, version and faucet come from the announcement", m)
	}
	if m.ReleaseRootSHA256 != netregistry.Digest([]byte(announcedRoot)) {
		t.Errorf("release root sha256 %s: the root is the announcement's", m.ReleaseRootSHA256)
	}
	if len(m.Seeds) != 2 || m.Seeds[0] != "seed1.testnet.example" {
		t.Errorf("seeds = %v: the announcement's seeds, not the defaults", m.Seeds)
	}
	if !res.Created.Announced || !strings.Contains(strings.Join(res.Created.NextSteps(), "\n"), "replacing its announcement") {
		t.Errorf("the next steps do not say the copy replaces the announcement:\n%s", strings.Join(res.Created.NextSteps(), "\n"))
	}
	if !strings.Contains(strings.Join(res.Plan.Notes, "\n"), "announced in the registry") {
		t.Errorf("the plan does not say where the values came from:\n%s", strings.Join(res.Plan.Notes, "\n"))
	}
}

func TestRunCreate_aFlagOverridesTheAnnouncement(t *testing.T) {
	h := newCreateHarness(t)
	h.announce(true)
	opts := Options{IPs: fiveIPs, Yes: true, StorageGB: 10, Create: &CreateOptions{
		Name: "testnet", PublishDir: h.publishDir, ReleaseRoot: h.rootFile, ReleaseRepo: "https://mirror.example", Channel: "dev/x",
		Seeds: []string{"s1.override.example"}, NoFaucet: true, TorAuthoritiesSet: true,
	}}

	res := h.mustCreate(t, opts)

	m := res.Created.Manifest
	if m.ReleaseRepo != "https://mirror.example" || m.Channel != "dev/x" || m.Seeds[0] != "s1.override.example" || m.Faucet {
		t.Errorf("manifest = %+v: the flags win", m)
	}
	if m.ReleaseRootSHA256 == netregistry.Digest([]byte(announcedRoot)) {
		t.Error("--release-root was ignored for the announcement's root")
	}
}

func TestRunCreate_theAnnouncementDecidesTheFaucet(t *testing.T) {
	h := newCreateHarness(t)
	h.announce(false)
	opts := Options{IPs: fiveIPs, Yes: true, StorageGB: 10, Create: &CreateOptions{Name: "testnet", PublishDir: h.publishDir, TorAuthoritiesSet: true}}
	if res := h.mustCreate(t, opts); res.Created.Manifest.Faucet {
		t.Error("the announcement says no faucet, and the network got one")
	}
}

func TestRunCreate_theFullManifestIsPublishedOverTheAnnouncement(t *testing.T) {
	h := newCreateHarness(t)
	root, err := os.ReadFile(h.rootFile)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := netregistry.Announce(netregistry.AnnounceInput{
		Dir: h.publishDir, Name: "testnet", ChainID: testChainID, ReleaseRoot: root, Channel: "nightly", MinVersion: "0.3.0",
		ReleaseRepo: announcedRepo, Seeds: []string{"ns1.testnet.orama.network"}, Faucet: true,
	}); err != nil {
		t.Fatal(err)
	}

	res := h.mustCreate(t, h.createOpts(fiveIPs...))

	checkPublished(t, h, res)
	data, err := os.ReadFile(filepath.Join(h.publishDir, "testnet", netregistry.ManifestFile))
	if err != nil {
		t.Fatal(err)
	}
	if m, err := netregistry.ParseManifest(data); err != nil || m.Announced() {
		t.Errorf("the manifest on disk is still the announcement: %+v, %v", m, err)
	}
}

func TestResolveAnnounced_leavesAnythingElseAlone(t *testing.T) {
	for name, networks := range map[string]fakeNetworks{
		"a network the registry does not know": {unknown: true},
		"a network that is already created":    {},
	} {
		t.Run(name, func(t *testing.T) {
			o := Options{Create: &CreateOptions{Name: "testnet", ReleaseRepo: "https://mine.example"}}
			if err := ResolveAnnounced(context.Background(), &o, Deps{Networks: networks}); err != nil {
				t.Fatal(err)
			}
			if c := o.Create; c.Announced || c.ChainID != "" || c.AnnouncedRoot != nil || c.ReleaseRepo != "https://mine.example" {
				t.Errorf("create = %+v", c)
			}
		})
	}
}

func TestResolveAnnounced_doesNotTouchTheCallersOptions(t *testing.T) {
	mine := &CreateOptions{Name: "testnet"}
	o := Options{Create: mine}
	if err := ResolveAnnounced(context.Background(), &o, Deps{Networks: fakeNetworks{announced: true}}); err != nil {
		t.Fatal(err)
	}
	if mine.ChainID != "" || mine.Announced || o.Create.ChainID != testChainID {
		t.Errorf("the caller's options changed: %+v (resolved %+v)", mine, o.Create)
	}
}

func TestResolveAnnounced_noNameAndNoCreationAreNoOps(t *testing.T) {
	deps := Deps{Networks: failingNetworks{}}
	for name, o := range map[string]Options{"a join": {}, "a creation without a name": {Create: &CreateOptions{}}} {
		if err := ResolveAnnounced(context.Background(), &o, deps); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestResolveAnnounced_aRegistryThatCannotBeReadIsAnError(t *testing.T) {
	o := Options{Create: &CreateOptions{Name: "testnet"}}
	err := ResolveAnnounced(context.Background(), &o, Deps{Networks: failingNetworks{}})
	if err == nil || !strings.Contains(err.Error(), `announcement of the network "testnet"`) {
		t.Fatalf("got %v", err)
	}
}

// failingNetworks cannot read the registry.
type failingNetworks struct{ fakeNetworks }

func (failingNetworks) Resolve(context.Context, string) (*netregistry.Network, error) {
	return nil, errors.New("registry unreadable")
}

func TestCreateOptions_withoutAnAnnouncementTheRootAndChainIDAreStillRequired(t *testing.T) {
	o := createOptions(nIPs(1)...)
	o.Create.ReleaseRoot = ""
	if err := o.Normalize(); err == nil || !strings.Contains(err.Error(), "--release-root") {
		t.Errorf("got %v", err)
	}
	o = createOptions(nIPs(1)...)
	o.Create.ChainID = ""
	if err := o.Normalize(); err == nil || !strings.Contains(err.Error(), "--chain-id") {
		t.Errorf("got %v", err)
	}
}

func TestRun_anAnnouncedNetworkCannotBeJoined(t *testing.T) {
	for name, tweak := range map[string]func(*Options){
		"a full join":  func(*Options) {},
		"cluster only": func(o *Options) { o.ClusterOnly, o.Name, o.StorageGB = true, "", 0 },
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness()
			h.deps.Networks = fakeNetworks{w: h.w, announced: true}
			opts := h.opts(ip1)
			tweak(&opts)
			_, err := run(t, h, opts)
			if err == nil || err.Error() != "testnet has not been created yet; its creator runs orama setup --create-network testnet" {
				t.Fatalf("got %v", err)
			}
			if h.w.index("enroll ") >= 0 {
				t.Errorf("a machine was contacted:\n%s", strings.Join(h.w.entries(), "\n"))
			}
		})
	}
}

func TestRun_aClusterOnlyNetworkIsJoinedWithoutTheGlobalLayer(t *testing.T) {
	h := newHarness()
	h.deps.Networks = fakeNetworks{w: h.w, clusterOnly: true}
	_, err := run(t, h, h.opts(ip1))
	if clierr.CodeOf(err) != clierr.CodeUsage || !strings.Contains(err.Error(), "testnet is a cluster-only network") || !strings.Contains(err.Error(), "--cluster-only") {
		t.Fatalf("a full join of a cluster-only network: %v (code %d), want a usage error naming --cluster-only", err, clierr.CodeOf(err))
	}
	if h.w.index("enroll ") >= 0 {
		t.Errorf("a machine was contacted:\n%s", strings.Join(h.w.entries(), "\n"))
	}

	h = newHarness()
	h.deps.Networks = fakeNetworks{w: h.w, clusterOnly: true}
	opts := h.opts(ip1)
	opts.ClusterOnly, opts.Name, opts.StorageGB = true, "", 0
	res := mustRun(t, h, opts)
	if res.Plan.ChainID != "" || res.Operator != "" {
		t.Errorf("plan chain %q, operator %q: a cluster-only network has neither", res.Plan.ChainID, res.Operator)
	}
	if summary := res.Plan.Summary()[0]; summary != "network testnet (cluster-only, no chain, channel nightly)" {
		t.Errorf("summary = %q", summary)
	}
	for _, chainStep := range []string{"genesis of a cluster-only network", "trust point"} {
		if h.w.index(chainStep) >= 0 {
			t.Errorf("the run reached for the chain (%s):\n%s", chainStep, strings.Join(h.w.entries(), "\n"))
		}
	}
}

func TestPlanFor_aClusterOnlyNetworkNeedsClusterOnly(t *testing.T) {
	h := newHarness()
	h.deps.Networks = fakeNetworks{w: h.w, clusterOnly: true}
	if _, err := PlanFor(context.Background(), h.opts(ip1), h.deps); !errors.Is(err, netregistry.ErrClusterOnly) {
		t.Fatalf("got %v, want ErrClusterOnly", err)
	}
	opts := h.opts(ip1)
	opts.ClusterOnly, opts.Name, opts.StorageGB = true, "", 0
	if _, err := PlanFor(context.Background(), opts, h.deps); err != nil {
		t.Fatalf("a cluster-only plan: %v", err)
	}
}

func TestPlanFor_anAnnouncedNetworkCannotBeJoined(t *testing.T) {
	h := newHarness()
	h.deps.Networks = fakeNetworks{w: h.w, announced: true}
	_, err := PlanFor(context.Background(), h.opts(ip1), h.deps)
	if !errors.Is(err, netregistry.ErrNotCreated) {
		t.Fatalf("got %v", err)
	}
}
