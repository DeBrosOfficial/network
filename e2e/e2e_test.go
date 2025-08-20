//go:build e2e

package e2e

import (
	"context"
	"encoding/base64"
	"fmt"
	"os/exec"
	"testing"
	"time"

	"git.debros.io/DeBros/network/pkg/client"
	"git.debros.io/DeBros/network/pkg/config"
	"git.debros.io/DeBros/network/pkg/node"
)

func startNode(t *testing.T, id string, p2pPort, httpPort, raftPort int, dataDir string) *node.Node {
	// Ensure rqlited is available
	if _, err := exec.LookPath("rqlited"); err != nil {
		t.Skip("rqlited not found in PATH; skipping e2e")
	}

	t.Helper()
	cfg := config.DefaultConfig()
	cfg.Node.ID = id
	cfg.Node.ListenAddresses = []string{fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", p2pPort)}
	cfg.Node.DataDir = dataDir
	cfg.Database.RQLitePort = httpPort
	cfg.Database.RQLiteRaftPort = raftPort
	cfg.Database.RQLiteJoinAddress = ""
	cfg.Discovery.HttpAdvAddress = "127.0.0.1"
	cfg.Discovery.RaftAdvAddress = ""
	cfg.Discovery.BootstrapPeers = nil

	n, err := node.NewNode(cfg)
	if err != nil { t.Fatalf("new node: %v", err) }
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	if err := n.Start(ctx); err != nil { t.Fatalf("start node: %v", err) }
	t.Cleanup(func() { _ = n.Stop() })
	return n
}

func waitUntil(t *testing.T, d time.Duration, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if cond() { return }
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("timeout: %s", msg)
}

func TestE2E_Nodes_Client_DB_Storage(t *testing.T) {
	// Start single node
	n1 := startNode(t, "n1", 4001, 5001, 7001, t.TempDir()+"/n1")

	// Build bootstrap multiaddr with peer ID
	n1Addr := fmt.Sprintf("/ip4/127.0.0.1/tcp/4001/p2p/%s", n1.GetPeerID())

	// Create client and connect via bootstrap
	cliCfg := client.DefaultClientConfig("e2e")
	cliCfg.BootstrapPeers = []string{n1Addr}
	cliCfg.DatabaseEndpoints = []string{"http://127.0.0.1:5001"}
	cliCfg.APIKey = "ak_test:default"
	cliCfg.QuietMode = true
	c, err := client.NewClient(cliCfg)
	if err != nil { t.Fatalf("new client: %v", err) }
	if err := c.Connect(); err != nil { t.Fatalf("client connect: %v", err) }
	defer c.Disconnect()

	// Wait until client has at least one peer (bootstrap)
	waitUntil(t, 20*time.Second, func() bool {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		peers, err := c.Network().GetPeers(ctx)
		return err == nil && len(peers) >= 1
	}, "client did not connect to any peer")

	// Create kv table for storage service (best-effort)
	ctx := client.WithInternalAuth(context.Background())
	_, _ = c.Database().Query(ctx, `CREATE TABLE IF NOT EXISTS kv_storage (
		namespace TEXT NOT NULL,
		key TEXT NOT NULL,
		value BLOB NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (namespace, key)
	)`)

	// Storage put/get through P2P
	putCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.Storage().Put(putCtx, "e2e:key", []byte("hello")); err != nil {
		t.Fatalf("storage put: %v", err)
	}
	getCtx, cancel2b := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2b()
	val, err := c.Storage().Get(getCtx, "e2e:key")
	if err != nil { t.Fatalf("storage get: %v", err) }
	if string(val) != "hello" {
		// Some environments may return base64-encoded text; accept if it decodes to "hello"
		if dec, derr := base64.StdEncoding.DecodeString(string(val)); derr != nil || string(dec) != "hello" {
			t.Fatalf("unexpected value: %q", string(val))
		}
	}
}
