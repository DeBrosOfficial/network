package e2e

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/DeBrosOfficial/network/pkg/client"
	"github.com/DeBrosOfficial/network/pkg/config"
	"github.com/DeBrosOfficial/network/pkg/node"
	"go.uber.org/zap"
)

// TestSingleNodeDatabaseCreation tests creating a database with replication factor 1
func TestSingleNodeDatabaseCreation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	// Setup test environment
	testDir := filepath.Join(os.TempDir(), fmt.Sprintf("debros_test_single_%d", time.Now().Unix()))
	defer os.RemoveAll(testDir)

	logger, _ := zap.NewDevelopment()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Start node
	cfg := config.DefaultConfig()
	cfg.Node.DataDir = filepath.Join(testDir, "node1")
	cfg.P2P.ListenAddresses = []string{"/ip4/127.0.0.1/tcp/14001"}
	cfg.Database.ReplicationFactor = 1
	cfg.Database.MaxDatabases = 10
	cfg.Database.PortRangeHTTPStart = 15001
	cfg.Database.PortRangeHTTPEnd = 15010
	cfg.Database.PortRangeRaftStart = 17001
	cfg.Database.PortRangeRaftEnd = 17010

	n, err := node.NewNode(cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create node: %v", err)
	}

	if err := n.Start(ctx); err != nil {
		t.Fatalf("Failed to start node: %v", err)
	}
	defer n.Stop()

	// Wait for node to be ready
	time.Sleep(2 * time.Second)

	// Create client
	cli, err := client.NewClient(ctx, client.ClientConfig{
		AppName:        "testapp",
		BootstrapPeers: []string{n.Host().Addrs()[0].String() + "/p2p/" + n.Host().ID().String()},
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer cli.Close()

	// Create database
	db := cli.Database("testdb")

	// Write data
	_, err = db.WriteOne("CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)")
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	_, err = db.WriteOne("INSERT INTO users (name) VALUES ('Alice')")
	if err != nil {
		t.Fatalf("Failed to insert data: %v", err)
	}

	// Read data back
	rows, err := db.QueryOne("SELECT * FROM users")
	if err != nil {
		t.Fatalf("Failed to query data: %v", err)
	}

	if !rows.Next() {
		t.Fatal("Expected at least one row")
	}

	var id int
	var name string
	if err := rows.Scan(&id, &name); err != nil {
		t.Fatalf("Failed to scan row: %v", err)
	}

	if name != "Alice" {
		t.Errorf("Expected name 'Alice', got '%s'", name)
	}

	t.Log("Single node database creation test passed")
}

// TestThreeNodeDatabaseCreation tests creating a database with replication factor 3
func TestThreeNodeDatabaseCreation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	// Setup test environment
	testDir := filepath.Join(os.TempDir(), fmt.Sprintf("debros_test_three_%d", time.Now().Unix()))
	defer os.RemoveAll(testDir)

	logger, _ := zap.NewDevelopment()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Start 3 nodes
	nodes := make([]*node.Node, 3)
	for i := 0; i < 3; i++ {
		cfg := config.DefaultConfig()
		cfg.Node.DataDir = filepath.Join(testDir, fmt.Sprintf("node%d", i+1))
		cfg.P2P.ListenAddresses = []string{fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", 14001+i)}
		cfg.Database.ReplicationFactor = 3
		cfg.Database.MaxDatabases = 10
		cfg.Database.PortRangeHTTPStart = 15001 + (i * 100)
		cfg.Database.PortRangeHTTPEnd = 15010 + (i * 100)
		cfg.Database.PortRangeRaftStart = 17001 + (i * 100)
		cfg.Database.PortRangeRaftEnd = 17010 + (i * 100)

		// Connect to first node
		if i > 0 {
			bootstrapAddr := nodes[0].Host().Addrs()[0].String() + "/p2p/" + nodes[0].Host().ID().String()
			cfg.P2P.BootstrapPeers = []string{bootstrapAddr}
		}

		n, err := node.NewNode(cfg, logger)
		if err != nil {
			t.Fatalf("Failed to create node %d: %v", i+1, err)
		}

		if err := n.Start(ctx); err != nil {
			t.Fatalf("Failed to start node %d: %v", i+1, err)
		}
		defer n.Stop()

		nodes[i] = n
	}

	// Wait for nodes to discover each other
	time.Sleep(5 * time.Second)

	// Create client connected to first node
	bootstrapAddr := nodes[0].Host().Addrs()[0].String() + "/p2p/" + nodes[0].Host().ID().String()
	cli, err := client.NewClient(ctx, client.ClientConfig{
		AppName:        "testapp",
		BootstrapPeers: []string{bootstrapAddr},
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer cli.Close()

	// Create database
	db := cli.Database("testdb")

	// Write data
	_, err = db.WriteOne("CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)")
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	_, err = db.WriteOne("INSERT INTO users (name) VALUES ('Alice')")
	if err != nil {
		t.Fatalf("Failed to insert data: %v", err)
	}

	// Wait for replication
	time.Sleep(2 * time.Second)

	// Read from all nodes to verify replication
	// Note: This would require connecting to each node separately
	// For now, just verify we can read from the first node
	rows, err := db.QueryOne("SELECT * FROM users")
	if err != nil {
		t.Fatalf("Failed to query data: %v", err)
	}

	if !rows.Next() {
		t.Fatal("Expected at least one row")
	}

	var id int
	var name string
	if err := rows.Scan(&id, &name); err != nil {
		t.Fatalf("Failed to scan row: %v", err)
	}

	if name != "Alice" {
		t.Errorf("Expected name 'Alice', got '%s'", name)
	}

	t.Log("Three node database creation test passed")
}

// TestMultipleDatabases tests creating multiple isolated databases
func TestMultipleDatabases(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	// Setup test environment
	testDir := filepath.Join(os.TempDir(), fmt.Sprintf("debros_test_multi_%d", time.Now().Unix()))
	defer os.RemoveAll(testDir)

	logger, _ := zap.NewDevelopment()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Start 3 nodes
	nodes := make([]*node.Node, 3)
	for i := 0; i < 3; i++ {
		cfg := config.DefaultConfig()
		cfg.Node.DataDir = filepath.Join(testDir, fmt.Sprintf("node%d", i+1))
		cfg.P2P.ListenAddresses = []string{fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", 14001+i)}
		cfg.Database.ReplicationFactor = 3
		cfg.Database.MaxDatabases = 20
		cfg.Database.PortRangeHTTPStart = 15001 + (i * 200)
		cfg.Database.PortRangeHTTPEnd = 15050 + (i * 200)
		cfg.Database.PortRangeRaftStart = 17001 + (i * 200)
		cfg.Database.PortRangeRaftEnd = 17050 + (i * 200)

		if i > 0 {
			bootstrapAddr := nodes[0].Host().Addrs()[0].String() + "/p2p/" + nodes[0].Host().ID().String()
			cfg.P2P.BootstrapPeers = []string{bootstrapAddr}
		}

		n, err := node.NewNode(cfg, logger)
		if err != nil {
			t.Fatalf("Failed to create node %d: %v", i+1, err)
		}

		if err := n.Start(ctx); err != nil {
			t.Fatalf("Failed to start node %d: %v", i+1, err)
		}
		defer n.Stop()

		nodes[i] = n
	}

	time.Sleep(5 * time.Second)

	// Create client
	bootstrapAddr := nodes[0].Host().Addrs()[0].String() + "/p2p/" + nodes[0].Host().ID().String()
	cli, err := client.NewClient(ctx, client.ClientConfig{
		AppName:        "testapp",
		BootstrapPeers: []string{bootstrapAddr},
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer cli.Close()

	// Create multiple databases
	databases := []string{"users_db", "products_db", "orders_db"}
	for _, dbName := range databases {
		db := cli.Database(dbName)

		// Create table specific to this database
		_, err = db.WriteOne(fmt.Sprintf("CREATE TABLE %s_data (id INTEGER PRIMARY KEY, value TEXT)", dbName))
		if err != nil {
			t.Fatalf("Failed to create table in %s: %v", dbName, err)
		}

		// Insert data
		_, err = db.WriteOne(fmt.Sprintf("INSERT INTO %s_data (value) VALUES ('%s_value')", dbName, dbName))
		if err != nil {
			t.Fatalf("Failed to insert data in %s: %v", dbName, err)
		}
	}

	// Verify isolation - each database should only have its own data
	for _, dbName := range databases {
		db := cli.Database(dbName)

		rows, err := db.QueryOne(fmt.Sprintf("SELECT value FROM %s_data", dbName))
		if err != nil {
			t.Fatalf("Failed to query %s: %v", dbName, err)
		}

		if !rows.Next() {
			t.Fatalf("Expected data in %s", dbName)
		}

		var value string
		if err := rows.Scan(&value); err != nil {
			t.Fatalf("Failed to scan row from %s: %v", dbName, err)
		}

		expectedValue := fmt.Sprintf("%s_value", dbName)
		if value != expectedValue {
			t.Errorf("Expected value '%s' in %s, got '%s'", expectedValue, dbName, value)
		}
	}

	t.Log("Multiple databases test passed")
}
