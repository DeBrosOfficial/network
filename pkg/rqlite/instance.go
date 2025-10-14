package rqlite

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
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

// Start starts the rqlite subprocess
func (ri *RQLiteInstance) Start(ctx context.Context, isLeader bool, joinAddr string) error {
	// Create data directory
	if err := os.MkdirAll(ri.DataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
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
	}

	// Add data directory as positional argument
	args = append(args, ri.DataDir)

	ri.logger.Info("Starting RQLite instance",
		zap.String("database", ri.DatabaseName),
		zap.Int("http_port", ri.HTTPPort),
		zap.Int("raft_port", ri.RaftPort),
		zap.String("data_dir", ri.DataDir),
		zap.Bool("is_leader", isLeader),
		zap.Strings("args", args))

	// Start RQLite process
	ri.Cmd = exec.Command("rqlited", args...)

	// Optionally capture stdout/stderr for debugging
	// ri.Cmd.Stdout = os.Stdout
	// ri.Cmd.Stderr = os.Stderr

	if err := ri.Cmd.Start(); err != nil {
		return fmt.Errorf("failed to start rqlited: %w", err)
	}

	// Wait for RQLite to be ready
	if err := ri.waitForReady(ctx); err != nil {
		ri.Stop()
		return fmt.Errorf("rqlited failed to become ready: %w", err)
	}

	// Create connection
	conn, err := gorqlite.Open(fmt.Sprintf("http://localhost:%d", ri.HTTPPort))
	if err != nil {
		ri.Stop()
		return fmt.Errorf("failed to connect to rqlited: %w", err)
	}
	ri.Connection = conn

	// Wait for SQL availability
	if err := ri.waitForSQLAvailable(ctx); err != nil {
		ri.Stop()
		return fmt.Errorf("rqlited SQL not available: %w", err)
	}

	ri.Status = StatusActive
	ri.LastQuery = time.Now()

	ri.logger.Info("RQLite instance started successfully",
		zap.String("database", ri.DatabaseName))

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
				return nil
			}
			if i%5 == 0 {
				ri.logger.Debug("Waiting for RQLite SQL availability",
					zap.String("database", ri.DatabaseName),
					zap.Error(err))
			}
		}
	}

	return fmt.Errorf("rqlited SQL not available within timeout")
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
