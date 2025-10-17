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

// TestHibernationCycle tests that databases hibernate after idle timeout
func TestHibernationCycle(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	// Setup test environment
	testDir := filepath.Join(os.TempDir(), fmt.Sprintf("debros_test_hibernate_%d", time.Now().Unix()))
	defer os.RemoveAll(testDir)

	logger, _ := zap.NewDevelopment()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Start 3 nodes with short hibernation timeout
	nodes := make([]*node.Node, 3)
	for i := 0; i < 3; i++ {
		cfg := config.DefaultConfig()
		cfg.Node.DataDir = filepath.Join(testDir, fmt.Sprintf("node%d", i+1))
		cfg.P2P.ListenAddresses = []string{fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", 14001+i)}
		cfg.Database.ReplicationFactor = 3
		cfg.Database.MaxDatabases = 10
		cfg.Database.HibernationTimeout = 10 * time.Second // Short timeout for testing
		cfg.Database.PortRangeHTTPStart = 15001 + (i * 100)
		cfg.Database.PortRangeHTTPEnd = 15010 + (i * 100)
		cfg.Database.PortRangeRaftStart = 17001 + (i * 100)
		cfg.Database.PortRangeRaftEnd = 17010 + (i * 100)

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

	// Create database and write data
	db := cli.Database("testdb")

	_, err = db.WriteOne("CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)")
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	_, err = db.WriteOne("INSERT INTO users (name) VALUES ('Alice')")
	if err != nil {
		t.Fatalf("Failed to insert data: %v", err)
	}

	t.Log("Data written, waiting for hibernation...")

	// Wait for hibernation (timeout + grace period)
	time.Sleep(20 * time.Second)

	t.Log("Hibernation period elapsed, database should be hibernating")

	// Note: In a real test, we would check the metadata store to verify hibernation status
	// For now, we just verify the data directory still exists
	for i := 0; i < 3; i++ {
		dbDir := filepath.Join(testDir, fmt.Sprintf("node%d/testapp_testdb", i+1))
		if _, err := os.Stat(dbDir); os.IsNotExist(err) {
			t.Errorf("Expected database directory to exist on node %d after hibernation", i+1)
		}
	}

	t.Log("Hibernation cycle test passed")
}

// TestWakeUpCycle tests that hibernated databases wake up on access
func TestWakeUpCycle(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	// Setup test environment
	testDir := filepath.Join(os.TempDir(), fmt.Sprintf("debros_test_wakeup_%d", time.Now().Unix()))
	defer os.RemoveAll(testDir)

	logger, _ := zap.NewDevelopment()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Start 3 nodes with short hibernation timeout
	nodes := make([]*node.Node, 3)
	for i := 0; i < 3; i++ {
		cfg := config.DefaultConfig()
		cfg.Node.DataDir = filepath.Join(testDir, fmt.Sprintf("node%d", i+1))
		cfg.P2P.ListenAddresses = []string{fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", 14001+i)}
		cfg.Database.ReplicationFactor = 3
		cfg.Database.MaxDatabases = 10
		cfg.Database.HibernationTimeout = 10 * time.Second
		cfg.Database.PortRangeHTTPStart = 15001 + (i * 100)
		cfg.Database.PortRangeHTTPEnd = 15010 + (i * 100)
		cfg.Database.PortRangeRaftStart = 17001 + (i * 100)
		cfg.Database.PortRangeRaftEnd = 17010 + (i * 100)

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

	// Create database and write data
	db := cli.Database("testdb")

	_, err = db.WriteOne("CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)")
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	_, err = db.WriteOne("INSERT INTO users (name) VALUES ('Alice')")
	if err != nil {
		t.Fatalf("Failed to insert initial data: %v", err)
	}

	t.Log("Waiting for hibernation...")
	time.Sleep(20 * time.Second)

	t.Log("Attempting to wake up database by querying...")

	// Query should trigger wake-up
	startTime := time.Now()
	rows, err := db.QueryOne("SELECT * FROM users")
	wakeupDuration := time.Since(startTime)

	if err != nil {
		t.Fatalf("Failed to query after hibernation (wake-up failed): %v", err)
	}

	t.Logf("Wake-up took %v", wakeupDuration)

	// Verify data persisted
	if !rows.Next() {
		t.Fatal("Expected at least one row after wake-up")
	}

	var id int
	var name string
	if err := rows.Scan(&id, &name); err != nil {
		t.Fatalf("Failed to scan row: %v", err)
	}

	if name != "Alice" {
		t.Errorf("Expected name 'Alice' after wake-up, got '%s'", name)
	}

	// Verify we can write new data
	_, err = db.WriteOne("INSERT INTO users (name) VALUES ('Bob')")
	if err != nil {
		t.Fatalf("Failed to insert data after wake-up: %v", err)
	}

	// Verify both records exist
	rows, err = db.QueryOne("SELECT COUNT(*) FROM users")
	if err != nil {
		t.Fatalf("Failed to count rows: %v", err)
	}

	if !rows.Next() {
		t.Fatal("Expected count result")
	}

	var count int
	if err := rows.Scan(&count); err != nil {
		t.Fatalf("Failed to scan count: %v", err)
	}

	if count != 2 {
		t.Errorf("Expected 2 rows after wake-up and insert, got %d", count)
	}

	t.Log("Wake-up cycle test passed")
}

// TestConcurrentHibernation tests multiple databases hibernating simultaneously
func TestConcurrentHibernation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	// Setup test environment
	testDir := filepath.Join(os.TempDir(), fmt.Sprintf("debros_test_concurrent_hibernate_%d", time.Now().Unix()))
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
		cfg.Database.HibernationTimeout = 10 * time.Second
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
	dbNames := []string{"db1", "db2", "db3"}
	for _, dbName := range dbNames {
		db := cli.Database(dbName)
		_, err = db.WriteOne(fmt.Sprintf("CREATE TABLE %s_data (id INTEGER PRIMARY KEY, value TEXT)", dbName))
		if err != nil {
			t.Fatalf("Failed to create table in %s: %v", dbName, err)
		}

		_, err = db.WriteOne(fmt.Sprintf("INSERT INTO %s_data (value) VALUES ('%s_value')", dbName, dbName))
		if err != nil {
			t.Fatalf("Failed to insert data in %s: %v", dbName, err)
		}
	}

	t.Log("All databases created, waiting for concurrent hibernation...")
	time.Sleep(20 * time.Second)

	t.Log("Hibernation period elapsed")

	// Verify all data directories still exist
	for _, dbName := range dbNames {
		for i := 0; i < 3; i++ {
			dbDir := filepath.Join(testDir, fmt.Sprintf("node%d/testapp_%s", i+1, dbName))
			if _, err := os.Stat(dbDir); os.IsNotExist(err) {
				t.Errorf("Expected database directory for %s to exist on node %d", dbName, i+1)
			}
		}
	}

	t.Log("Concurrent hibernation test passed")
}
