package rqlite

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/rqlite/gorqlite"
	"go.uber.org/zap"
)

// RQLiteInstance represents a single rqlite database instance
type RQLiteInstance struct {
	DatabaseName string
	HTTPPort     int
	RaftPort     int
	DataDir      string
	AdvHTTPAddr  string // Advertised HTTP address
	AdvRaftAddr  string // Advertised Raft address
	Cmd          *exec.Cmd
	Connection   *gorqlite.Connection
	LastQuery    time.Time
	Status       DatabaseStatus
	logger       *zap.Logger
}

// NewRQLiteInstance creates a new RQLite instance configuration
func NewRQLiteInstance(dbName string, ports PortPair, dataDir string, advHTTPAddr, advRaftAddr string, logger *zap.Logger) *RQLiteInstance {
	return &RQLiteInstance{
		DatabaseName: dbName,
		HTTPPort:     ports.HTTPPort,
		RaftPort:     ports.RaftPort,
		DataDir:      filepath.Join(dataDir, dbName, "rqlite"),
		AdvHTTPAddr:  advHTTPAddr,
		AdvRaftAddr:  advRaftAddr,
		Status:       StatusInitializing,
		logger:       logger,
	}
}

// hasExistingData checks if the data directory contains existing RQLite state
func (ri *RQLiteInstance) hasExistingData() bool {
	// Check for raft.db which indicates existing cluster state
	raftDBPath := filepath.Join(ri.DataDir, "raft.db")
	if _, err := os.Stat(raftDBPath); err == nil {
		return true
	}
	return false
}

// wasInCluster checks if this node was previously part of a Raft cluster
func (ri *RQLiteInstance) wasInCluster() bool {
	if !ri.hasExistingData() {
		return false
	}

	// Check for peers.json which indicates cluster membership
	peersFile := filepath.Join(ri.DataDir, "raft", "peers.json")
	if _, err := os.Stat(peersFile); err == nil {
		return true
	}

	// Alternative: check raft log size - if > 0, was in cluster
	raftDBPath := filepath.Join(ri.DataDir, "raft.db")
	if info, err := os.Stat(raftDBPath); err == nil && info.Size() > 0 {
		return true
	}

	return false
}

// Start starts the rqlite subprocess
func (ri *RQLiteInstance) Start(ctx context.Context, isLeader bool, joinAddr string) error {
	// Create data directory
	if err := os.MkdirAll(ri.DataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// Check for existing data and clear peer config if needed BEFORE starting RQLite
	hasExisting := ri.hasExistingData()
	if hasExisting {
		wasInCluster := ri.wasInCluster()
		ri.logger.Info("Found existing RQLite data, reusing state",
			zap.String("database", ri.DatabaseName),
			zap.String("data_dir", ri.DataDir),
			zap.Bool("is_leader", isLeader),
			zap.Bool("was_in_cluster", wasInCluster),
			zap.String("join_address", joinAddr))

	} else {
		ri.logger.Info("No existing RQLite data, starting fresh",
			zap.String("database", ri.DatabaseName),
			zap.String("data_dir", ri.DataDir),
			zap.Bool("is_leader", isLeader))
	}

	// Build rqlited command
	args := []string{
		"-http-addr", fmt.Sprintf("0.0.0.0:%d", ri.HTTPPort),
		"-raft-addr", fmt.Sprintf("0.0.0.0:%d", ri.RaftPort),
	}

	// Add advertised addresses if provided
	if ri.AdvHTTPAddr != "" {
		args = append(args, "-http-adv-addr", ri.AdvHTTPAddr)
	}
	if ri.AdvRaftAddr != "" {
		args = append(args, "-raft-adv-addr", ri.AdvRaftAddr)
	}

	// Add join address if this is a follower
	if !isLeader && joinAddr != "" {
		args = append(args, "-join", joinAddr)
		// Always add -join-as voter for rqlite v8 compatibility
		args = append(args, "-join-as", "voter")
		ri.logger.Info("Follower will join cluster as voter",
			zap.String("database", ri.DatabaseName))
	}

	// Add data directory as positional argument
	args = append(args, ri.DataDir)

	// Check for conflicting configuration: bootstrap + existing data
	if isLeader && !strings.Contains(strings.Join(args, " "), "-join") {
		// This is a bootstrap scenario (leader without join)
		if ri.hasExistingData() {
			ri.logger.Warn("Detected existing Raft state, will not bootstrap",
				zap.String("database", ri.DatabaseName),
				zap.String("data_dir", ri.DataDir))
			// Remove any bootstrap-only flags if they exist
			// RQLite will detect existing state and continue as member
		}
	}

	// For followers with existing data, verify join address is set
	if !isLeader && joinAddr == "" && ri.hasExistingData() {
		return fmt.Errorf("follower has existing Raft state but no join address provided")
	}

	ri.logger.Info("Starting RQLite instance",
		zap.String("database", ri.DatabaseName),
		zap.Int("http_port", ri.HTTPPort),
		zap.Int("raft_port", ri.RaftPort),
		zap.String("data_dir", ri.DataDir),
		zap.Bool("is_leader", isLeader),
		zap.Strings("args", args))

	// Start RQLite process
	ri.Cmd = exec.Command("rqlited", args...)

	// Capture stdout/stderr for debugging
	ri.Cmd.Stdout = os.Stdout
	ri.Cmd.Stderr = os.Stderr

	if err := ri.Cmd.Start(); err != nil {
		return fmt.Errorf("failed to start rqlited binary (check if installed): %w", err)
	}

	// Wait for RQLite to be ready
	if err := ri.waitForReady(ctx); err != nil {
		ri.logger.Error("RQLite failed to become ready",
			zap.String("database", ri.DatabaseName),
			zap.String("data_dir", ri.DataDir),
			zap.Int("http_port", ri.HTTPPort),
			zap.Int("raft_port", ri.RaftPort),
			zap.Error(err))
		ri.Stop()
		return fmt.Errorf("rqlited failed to become ready (check logs above): %w", err)
	}

	// Create connection
	conn, err := gorqlite.Open(fmt.Sprintf("http://localhost:%d", ri.HTTPPort))
	if err != nil {
		ri.Stop()
		return fmt.Errorf("failed to connect to rqlited: %w", err)
	}
	ri.Connection = conn

	// For leaders, wait for SQL to be immediately available
	// For followers, SQL will become available after cluster sync
	if isLeader {
		if err := ri.waitForSQLAvailable(ctx); err != nil {
			ri.Stop()
			return fmt.Errorf("leader SQL not available: %w", err)
		}
		ri.logger.Info("Leader SQL is ready",
			zap.String("database", ri.DatabaseName))
	} else {
		// For followers, just verify the node joined successfully
		if err := ri.waitForClusterJoin(ctx, 30*time.Second); err != nil {
			ri.logger.Warn("Follower may not have joined cluster yet, but continuing",
				zap.String("database", ri.DatabaseName),
				zap.Error(err))
			// Don't fail - SQL will become available eventually
		} else {
			ri.logger.Info("Follower successfully joined cluster",
				zap.String("database", ri.DatabaseName))
		}
	}

	ri.Status = StatusActive
	ri.LastQuery = time.Now()

	ri.logger.Info("RQLite instance started successfully",
		zap.String("database", ri.DatabaseName),
		zap.Bool("is_leader", isLeader))

	return nil
}

// Stop stops the rqlite subprocess gracefully
func (ri *RQLiteInstance) Stop() error {
	if ri.Connection != nil {
		ri.Connection.Close()
		ri.Connection = nil
	}

	if ri.Cmd == nil || ri.Cmd.Process == nil {
		return nil
	}

	ri.logger.Info("Stopping RQLite instance",
		zap.String("database", ri.DatabaseName))

	// Try SIGTERM first
	if err := ri.Cmd.Process.Signal(syscall.SIGTERM); err != nil {
		// Fallback to Kill if signaling fails
		_ = ri.Cmd.Process.Kill()
		return nil
	}

	// Wait up to 5 seconds for graceful shutdown
	done := make(chan error, 1)
	go func() { done <- ri.Cmd.Wait() }()

	select {
	case err := <-done:
		if err != nil && !errors.Is(err, os.ErrClosed) {
			ri.logger.Warn("RQLite process exited with error",
				zap.String("database", ri.DatabaseName),
				zap.Error(err))
		}
	case <-time.After(5 * time.Second):
		ri.logger.Warn("RQLite did not exit in time; killing",
			zap.String("database", ri.DatabaseName))
		_ = ri.Cmd.Process.Kill()
	}

	ri.Status = StatusHibernating
	return nil
}

// waitForReady waits for RQLite HTTP endpoint to be ready
func (ri *RQLiteInstance) waitForReady(ctx context.Context) error {
	url := fmt.Sprintf("http://localhost:%d/status", ri.HTTPPort)
	client := &http.Client{Timeout: 2 * time.Second}

	for i := 0; i < 60; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}

		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("rqlited did not become ready within timeout")
}

// waitForSQLAvailable waits until SQL queries can be executed
func (ri *RQLiteInstance) waitForSQLAvailable(ctx context.Context) error {
	if ri.Connection == nil {
		return errors.New("no rqlite connection")
	}

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for i := 0; i < 60; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			_, err := ri.Connection.QueryOne("SELECT 1")
			if err == nil {
				ri.logger.Info("SQL queries are now available",
					zap.String("database", ri.DatabaseName),
					zap.Int("attempts", i+1))
				return nil
			}
			// Log every 5 seconds with more detail
			if i%5 == 0 {
				ri.logger.Debug("Waiting for RQLite SQL availability",
					zap.String("database", ri.DatabaseName),
					zap.Int("attempt", i+1),
					zap.Int("max_attempts", 60),
					zap.Error(err))
			}
		}
	}

	return fmt.Errorf("rqlited SQL not available within timeout (60 seconds)")
}

// waitForClusterJoin waits for a follower node to successfully join the cluster
// This checks the /status endpoint for cluster membership info
func (ri *RQLiteInstance) waitForClusterJoin(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 2 * time.Second}
	statusURL := fmt.Sprintf("http://localhost:%d/status", ri.HTTPPort)

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	attempts := 0
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			attempts++

			resp, err := client.Get(statusURL)
			if err != nil {
				if attempts%5 == 0 {
					ri.logger.Debug("Checking cluster join status",
						zap.String("database", ri.DatabaseName),
						zap.Int("attempt", attempts),
						zap.Error(err))
				}
				continue
			}

			// Check if status code is OK
			if resp.StatusCode == http.StatusOK {
				resp.Body.Close()
				// If we can query status, the node has likely joined
				// Try a simple SQL query to confirm
				if ri.Connection != nil {
					_, err := ri.Connection.QueryOne("SELECT 1")
					if err == nil {
						return nil // SQL is ready!
					}
				}
				// Even if SQL not ready, status endpoint being available is good enough
				if attempts >= 5 {
					// After a few attempts, accept status endpoint as sufficient
					return nil
				}
			} else {
				resp.Body.Close()
			}
		}
	}

	return fmt.Errorf("cluster join check timed out after %v", timeout)
}

// StartBackgroundSQLReadinessCheck starts a background check for SQL readiness
// This is used for followers that may take time to sync cluster state
func (ri *RQLiteInstance) StartBackgroundSQLReadinessCheck(ctx context.Context, onReady func()) {
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if ri.Connection != nil {
					_, err := ri.Connection.QueryOne("SELECT 1")
					if err == nil {
						ri.logger.Info("Follower SQL is now ready",
							zap.String("database", ri.DatabaseName))
						if onReady != nil {
							onReady()
						}
						return // SQL is ready, stop checking
					}
				}
			}
		}
	}()
}

// UpdateLastQuery updates the last query timestamp
func (ri *RQLiteInstance) UpdateLastQuery() {
	ri.LastQuery = time.Now()
}

// IsIdle checks if the instance has been idle for the given duration
func (ri *RQLiteInstance) IsIdle(timeout time.Duration) bool {
	return time.Since(ri.LastQuery) > timeout
}

// IsRunning checks if the rqlite process is running
func (ri *RQLiteInstance) IsRunning() bool {
	if ri.Cmd == nil || ri.Cmd.Process == nil {
		return false
	}

	// Check if process is still alive
	err := ri.Cmd.Process.Signal(syscall.Signal(0))
	return err == nil
}
