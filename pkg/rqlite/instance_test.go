package rqlite

import (
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestRQLiteInstance_Create(t *testing.T) {
	logger := zap.NewNop()
	ports := PortPair{HTTPPort: 5001, RaftPort: 7001}

	instance := NewRQLiteInstance(
		"testdb",
		ports,
		"/tmp/test_data",
		"127.0.0.1",
		"127.0.0.1",
		logger,
	)

	if instance.DatabaseName != "testdb" {
		t.Errorf("Expected database name 'testdb', got '%s'", instance.DatabaseName)
	}

	if instance.Ports.HTTPPort != 5001 {
		t.Errorf("Expected HTTP port 5001, got %d", instance.Ports.HTTPPort)
	}

	if instance.Ports.RaftPort != 7001 {
		t.Errorf("Expected Raft port 7001, got %d", instance.Ports.RaftPort)
	}

	if instance.Status != StatusInitializing {
		t.Errorf("Expected status initializing, got %s", instance.Status)
	}

	expectedDataDir := "/tmp/test_data/testdb/rqlite"
	if instance.DataDir != expectedDataDir {
		t.Errorf("Expected data dir '%s', got '%s'", expectedDataDir, instance.DataDir)
	}
}

func TestRQLiteInstance_IsIdle(t *testing.T) {
	logger := zap.NewNop()
	ports := PortPair{HTTPPort: 5001, RaftPort: 7001}

	instance := NewRQLiteInstance(
		"testdb",
		ports,
		"/tmp/test_data",
		"127.0.0.1",
		"127.0.0.1",
		logger,
	)

	// Set LastQuery to old timestamp
	instance.LastQuery = time.Now().Add(-2 * time.Minute)

	// Check with 1 minute timeout - should be idle
	if !instance.IsIdle(1 * time.Minute) {
		t.Error("Expected instance to be idle after 2 minutes with 1 minute timeout")
	}

	// Check with 3 minute timeout - should NOT be idle
	if instance.IsIdle(3 * time.Minute) {
		t.Error("Expected instance to NOT be idle with 3 minute timeout")
	}

	// Update LastQuery to now
	instance.LastQuery = time.Now()

	// Should not be idle anymore
	if instance.IsIdle(1 * time.Minute) {
		t.Error("Expected instance to NOT be idle after updating LastQuery")
	}

	// Zero timeout should always return false (hibernation disabled)
	instance.LastQuery = time.Now().Add(-10 * time.Hour)
	if instance.IsIdle(0) {
		t.Error("Expected IsIdle to return false when timeout is 0 (disabled)")
	}
}

func TestRQLiteInstance_GetConnection(t *testing.T) {
	logger := zap.NewNop()
	ports := PortPair{HTTPPort: 5001, RaftPort: 7001}

	instance := NewRQLiteInstance(
		"testdb",
		ports,
		"/tmp/test_data",
		"127.0.0.1",
		"127.0.0.1",
		logger,
	)

	// Set LastQuery to old time
	oldTime := time.Now().Add(-1 * time.Hour)
	instance.LastQuery = oldTime

	// GetConnection should update LastQuery
	_ = instance.GetConnection()

	if instance.LastQuery.Before(oldTime.Add(59 * time.Minute)) {
		t.Error("Expected GetConnection to update LastQuery timestamp")
	}
}

// Note: Start/Stop tests require rqlite binary and are more suitable for integration tests
// They would look like this:
//
// func TestRQLiteInstance_StartStop(t *testing.T) {
//     if testing.Short() {
//         t.Skip("Skipping integration test")
//     }
//
//     logger := zap.NewNop()
//     ports := PortPair{HTTPPort: 15001, RaftPort: 17001}
//
//     instance := NewRQLiteInstance(
//         "testdb",
//         ports,
//         "/tmp/test_rqlite_start_stop",
//         "127.0.0.1",
//         "127.0.0.1",
//         logger,
//     )
//
//     ctx := context.Background()
//     err := instance.Start(ctx, true, "")
//     if err != nil {
//         t.Fatalf("Failed to start instance: %v", err)
//     }
//
//     // Verify HTTP endpoint is responsive
//     resp, err := http.Get(fmt.Sprintf("http://localhost:%d/status", ports.HTTPPort))
//     if err != nil {
//         t.Fatalf("HTTP endpoint not responsive: %v", err)
//     }
//     resp.Body.Close()
//
//     // Stop instance
//     err = instance.Stop()
//     if err != nil {
//         t.Fatalf("Failed to stop instance: %v", err)
//     }
//
//     // Verify process terminated
//     time.Sleep(1 * time.Second)
//     _, err = http.Get(fmt.Sprintf("http://localhost:%d/status", ports.HTTPPort))
//     if err == nil {
//         t.Error("Expected HTTP endpoint to be unreachable after stop")
//     }
// }
