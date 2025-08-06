package database

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/rqlite/gorqlite"
	"go.uber.org/zap"

	"git.debros.io/DeBros/network/pkg/config"
)

// RQLiteManager manages an RQLite node instance
type RQLiteManager struct {
	config     *config.DatabaseConfig
	dataDir    string
	logger     *zap.Logger
	cmd        *exec.Cmd
	connection *gorqlite.Connection
}

// NewRQLiteManager creates a new RQLite manager
func NewRQLiteManager(cfg *config.DatabaseConfig, dataDir string, logger *zap.Logger) *RQLiteManager {
	return &RQLiteManager{
		config:  cfg,
		dataDir: dataDir,
		logger:  logger,
	}
}

// Start starts the RQLite node
func (r *RQLiteManager) Start(ctx context.Context) error {
	// Create data directory
	rqliteDataDir := filepath.Join(r.dataDir, "rqlite")
	if err := os.MkdirAll(rqliteDataDir, 0755); err != nil {
		return fmt.Errorf("failed to create RQLite data directory: %w", err)
	}

	// Build RQLite command
	args := []string{
		"-http-addr", fmt.Sprintf("localhost:%d", r.config.RQLitePort),
		"-raft-addr", fmt.Sprintf("localhost:%d", r.config.RQLiteRaftPort),
	}

	// Add join address if specified (for non-bootstrap or secondary bootstrap nodes)
	if r.config.RQLiteJoinAddress != "" {
		args = append(args, "-join", r.config.RQLiteJoinAddress)
	}

	// Add data directory as positional argument
	args = append(args, rqliteDataDir)

	r.logger.Info("Starting RQLite node",
		zap.String("data_dir", rqliteDataDir),
		zap.Int("http_port", r.config.RQLitePort),
		zap.Int("raft_port", r.config.RQLiteRaftPort),
		zap.String("join_address", r.config.RQLiteJoinAddress),
	)

	// Start RQLite process
	r.cmd = exec.CommandContext(ctx, "rqlited", args...)
	r.cmd.Stdout = os.Stdout
	r.cmd.Stderr = os.Stderr

	if err := r.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start RQLite: %w", err)
	}

	// Wait for RQLite to be ready
	if err := r.waitForReady(ctx); err != nil {
		r.cmd.Process.Kill()
		return fmt.Errorf("RQLite failed to become ready: %w", err)
	}

	// Create connection
	conn, err := gorqlite.Open(fmt.Sprintf("http://localhost:%d", r.config.RQLitePort))
	if err != nil {
		r.cmd.Process.Kill()
		return fmt.Errorf("failed to connect to RQLite: %w", err)
	}
	r.connection = conn

	// Wait for RQLite to establish leadership (for bootstrap nodes)
	if r.config.RQLiteJoinAddress == "" {
		if err := r.waitForLeadership(ctx); err != nil {
			r.cmd.Process.Kill()
			return fmt.Errorf("RQLite failed to establish leadership: %w", err)
		}
	}

	r.logger.Info("RQLite node started successfully")
	return nil
}

// waitForReady waits for RQLite to be ready to accept connections
func (r *RQLiteManager) waitForReady(ctx context.Context) error {
	url := fmt.Sprintf("http://localhost:%d/status", r.config.RQLitePort)
	client := &http.Client{Timeout: 2 * time.Second}

	for i := 0; i < 30; i++ {
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

	return fmt.Errorf("RQLite did not become ready within timeout")
}

// waitForLeadership waits for RQLite to establish leadership (for bootstrap nodes)
func (r *RQLiteManager) waitForLeadership(ctx context.Context) error {
	r.logger.Info("Waiting for RQLite to establish leadership...")

	for i := 0; i < 30; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Try a simple query to check if leadership is established
		if r.connection != nil {
			_, err := r.connection.QueryOne("SELECT 1")
			if err == nil {
				r.logger.Info("RQLite leadership established")
				return nil
			}
			r.logger.Debug("Waiting for leadership", zap.Error(err))
		}

		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("RQLite failed to establish leadership within timeout")
}

// GetConnection returns the RQLite connection
func (r *RQLiteManager) GetConnection() *gorqlite.Connection {
	return r.connection
}

// Stop stops the RQLite node
func (r *RQLiteManager) Stop() error {
	if r.connection != nil {
		r.connection.Close()
	}

	if r.cmd != nil && r.cmd.Process != nil {
		r.logger.Info("Stopping RQLite node")
		return r.cmd.Process.Kill()
	}

	return nil
}
