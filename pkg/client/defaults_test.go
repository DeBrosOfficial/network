package client

import (
	"os"
	"testing"

	"github.com/multiformats/go-multiaddr"
)

func TestDefaultBootstrapPeersNonEmpty(t *testing.T) {
	old := os.Getenv("DEBROS_BOOTSTRAP_PEERS")
	t.Cleanup(func() { os.Setenv("DEBROS_BOOTSTRAP_PEERS", old) })
	_ = os.Setenv("DEBROS_BOOTSTRAP_PEERS", "") // ensure not set
	peers := DefaultBootstrapPeers()
	if len(peers) == 0 {
		t.Fatalf("expected non-empty default bootstrap peers")
	}
}

func TestDefaultDatabaseEndpointsEnvOverride(t *testing.T) {
	oldNodes := os.Getenv("RQLITE_NODES")
	t.Cleanup(func() { os.Setenv("RQLITE_NODES", oldNodes) })
	_ = os.Setenv("RQLITE_NODES", "db1.local:7001, https://db2.local:7443")
	endpoints := DefaultDatabaseEndpoints()
	if len(endpoints) != 2 {
		t.Fatalf("expected 2 endpoints from env, got %v", endpoints)
	}
}

func TestNormalizeEndpoints(t *testing.T) {
	in := []string{"db.local", "http://db.local:5001", "[::1]", "https://host:8443"}
	out := normalizeEndpoints(in)
	if len(out) != 4 {
		t.Fatalf("unexpected len: %v", out)
	}
	foundDefault := false
	for _, s := range out {
		if s == "http://db.local:5001" {
			foundDefault = true
		}
	}
	if !foundDefault {
		t.Fatalf("missing normalized default port: %v", out)
	}
}

func TestEndpointFromMultiaddr(t *testing.T) {
	ma, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/4001")
	if ep := endpointFromMultiaddr(ma, 5001); ep != "http://127.0.0.1:5001" {
		t.Fatalf("unexpected endpoint: %s", ep)
	}
}
