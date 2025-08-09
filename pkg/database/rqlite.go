package database

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
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

// waitForSQLAvailable waits until a simple query succeeds, indicating a leader is known and queries can be served.
func (r *RQLiteManager) waitForSQLAvailable(ctx context.Context) error {
    if r.connection == nil {
        return fmt.Errorf("no rqlite connection")
    }

    ticker := time.NewTicker(1 * time.Second)
    defer ticker.Stop()

    attempts := 0
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-ticker.C:
            attempts++
            _, err := r.connection.QueryOne("SELECT 1")
            if err == nil {
                r.logger.Info("RQLite SQL is available")
                return nil
            }
            if attempts%5 == 0 { // log every ~5s to reduce noise
                r.logger.Debug("Waiting for RQLite SQL availability", zap.Error(err))
            }
        }
    }
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

	// Determine advertise host based on configuration
	advertiseHost := "127.0.0.1" // default
	mode := strings.ToLower(r.config.AdvertiseMode)
	switch mode {
	case "localhost":
		advertiseHost = "127.0.0.1"
		r.logger.Info("Using localhost for RQLite advertising (dev mode)")
	case "ip":
		if ip, err := r.getExternalIP(); err == nil && ip != "" {
			advertiseHost = ip
			r.logger.Info("Using external IP for RQLite advertising (forced)", zap.String("ip", ip))
		} else {
			r.logger.Warn("Failed to get external IP, falling back to localhost", zap.Error(err))
		}
	default: // auto
		if ip, err := r.getExternalIP(); err == nil && ip != "" {
			advertiseHost = ip
			r.logger.Info("Using external IP for RQLite advertising (auto)", zap.String("ip", ip))
		} else {
			r.logger.Info("No external IP found, using localhost for RQLite advertising (auto)")
		}
	}

	// Build RQLite command
	args := []string{
		"-http-addr", fmt.Sprintf("0.0.0.0:%d", r.config.RQLitePort),
		"-raft-addr", fmt.Sprintf("0.0.0.0:%d", r.config.RQLiteRaftPort),
		// Auth disabled for testing
	}

	// Always set advertised addresses explicitly to avoid 0.0.0.0 announcements
	args = append(args, "-http-adv-addr", fmt.Sprintf("%s:%d", advertiseHost, r.config.RQLitePort))
	args = append(args, "-raft-adv-addr", fmt.Sprintf("%s:%d", advertiseHost, r.config.RQLiteRaftPort))

	// Add join address if specified (for non-bootstrap or secondary bootstrap nodes)
	if r.config.RQLiteJoinAddress != "" {
		r.logger.Info("Joining RQLite cluster", zap.String("join_address", r.config.RQLiteJoinAddress))

		// Normalize join address to host:port for rqlited -join
		joinArg := r.config.RQLiteJoinAddress
		if strings.HasPrefix(joinArg, "http://") {
			joinArg = strings.TrimPrefix(joinArg, "http://")
		} else if strings.HasPrefix(joinArg, "https://") {
			joinArg = strings.TrimPrefix(joinArg, "https://")
		}

		// Wait for join target to become reachable to avoid forming a separate cluster (wait indefinitely)
        if err := r.waitForJoinTarget(ctx, joinArg, 0); err != nil {
            r.logger.Warn("Join target did not become reachable within timeout; will still attempt to join",
                zap.String("join_address", r.config.RQLiteJoinAddress),
                zap.Error(err))
        }

		// Always add the join parameter in host:port form - let rqlited handle the rest
		args = append(args, "-join", joinArg)
	} else {
		r.logger.Info("No join address specified - starting as new cluster")
	}

	// Add data directory as positional argument
	args = append(args, rqliteDataDir)

	r.logger.Info("Starting RQLite node",
		zap.String("data_dir", rqliteDataDir),
		zap.Int("http_port", r.config.RQLitePort),
		zap.Int("raft_port", r.config.RQLiteRaftPort),
		zap.String("join_address", r.config.RQLiteJoinAddress),
		zap.String("advertise_host", advertiseHost),
		zap.Strings("full_args", args),
	)

	// Start RQLite process (not bound to ctx for graceful Stop handling)
	r.cmd = exec.Command("rqlited", args...)
	r.cmd.Stdout = os.Stdout
	r.cmd.Stderr = os.Stderr

	if err := r.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start RQLite: %w", err)
	}

	// Wait for RQLite to be ready
	if err := r.waitForReady(ctx); err != nil {
		if r.cmd != nil && r.cmd.Process != nil {
			_ = r.cmd.Process.Kill()
		}
		return fmt.Errorf("RQLite failed to become ready: %w", err)
	}

	// Create connection
	conn, err := gorqlite.Open(fmt.Sprintf("http://localhost:%d", r.config.RQLitePort))
	if err != nil {
		if r.cmd != nil && r.cmd.Process != nil {
			_ = r.cmd.Process.Kill()
		}
		return fmt.Errorf("failed to connect to RQLite: %w", err)
	}
	r.connection = conn

	// Leadership/SQL readiness gating
	//
	// Fresh bootstrap (no join, no prior state): wait for leadership so queries will work.
	// Existing state or joiners: wait for SQL availability (leader known) before proceeding,
	// so higher layers (storage) don't fail with 500 leader-not-found.
	if r.config.RQLiteJoinAddress == "" && !r.hasExistingState(rqliteDataDir) {
		if err := r.waitForLeadership(ctx); err != nil {
			if r.cmd != nil && r.cmd.Process != nil {
				_ = r.cmd.Process.Kill()
			}
			return fmt.Errorf("RQLite failed to establish leadership: %w", err)
		}
	} else {
		r.logger.Info("Waiting for RQLite SQL availability (leader discovery)")
		if err := r.waitForSQLAvailable(ctx); err != nil {
			if r.cmd != nil && r.cmd.Process != nil {
				_ = r.cmd.Process.Kill()
			}
			return fmt.Errorf("RQLite SQL not available: %w", err)
		}
	}

	r.logger.Info("RQLite node started successfully")
	return nil
}

// hasExistingState returns true if the rqlite data directory already contains files or subdirectories.
func (r *RQLiteManager) hasExistingState(rqliteDataDir string) bool {
	entries, err := os.ReadDir(rqliteDataDir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		// Any existing file or directory indicates prior state
		if e.Name() == "." || e.Name() == ".." {
			continue
		}
		return true
	}
	return false
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
		r.connection = nil
	}

	if r.cmd == nil || r.cmd.Process == nil {
		return nil
	}

	r.logger.Info("Stopping RQLite node (graceful)")
	// Try SIGTERM first
	if err := r.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		// Fallback to Kill if signaling fails
		_ = r.cmd.Process.Kill()
		return nil
	}

	// Wait up to 5 seconds for graceful shutdown
	done := make(chan error, 1)
	go func() { done <- r.cmd.Wait() }()

	select {
	case err := <-done:
		if err != nil && !errors.Is(err, os.ErrClosed) {
			r.logger.Warn("RQLite process exited with error", zap.Error(err))
		}
	case <-time.After(5 * time.Second):
		r.logger.Warn("RQLite did not exit in time; killing")
		_ = r.cmd.Process.Kill()
	}

	return nil
}

// waitForJoinTarget waits until the join target's HTTP status becomes reachable, or until timeout
func (r *RQLiteManager) waitForJoinTarget(ctx context.Context, joinAddress string, timeout time.Duration) error {
    var deadline time.Time
    if timeout > 0 {
        deadline = time.Now().Add(timeout)
    }
    var lastErr error

    for {
        if err := r.testJoinAddress(joinAddress); err == nil {
            r.logger.Info("Join target is reachable, proceeding with cluster join")
            return nil
        } else {
            lastErr = err
            r.logger.Debug("Join target not yet reachable; waiting...", zap.String("join_address", joinAddress), zap.Error(err))
        }

        // Check context
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-time.After(2 * time.Second):
        }

        if !deadline.IsZero() && time.Now().After(deadline) {
            break
        }
    }

    if lastErr == nil {
        lastErr = fmt.Errorf("join target not reachable within %s", timeout)
    }
    return lastErr
}

// getExternalIP attempts to get the external IP address of this machine
func (r *RQLiteManager) getExternalIP() (string, error) {
	// Method 1: Try using `ip route get` to find the IP used to reach the internet
	if output, err := exec.Command("ip", "route", "get", "8.8.8.8").Output(); err == nil {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.Contains(line, "src") {
				parts := strings.Fields(line)
				for i, part := range parts {
					if part == "src" && i+1 < len(parts) {
						ip := parts[i+1]
						if net.ParseIP(ip) != nil {
							r.logger.Debug("Found external IP via ip route", zap.String("ip", ip))
							return ip, nil
						}
					}
				}
			}
		}
	}

	// Method 2: Get all network interfaces and find non-localhost, non-private IPs
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if ip == nil || ip.IsLoopback() {
				continue
			}

			// Prefer public IPs over private IPs
			if ip.To4() != nil && !ip.IsPrivate() {
				r.logger.Debug("Found public IP", zap.String("ip", ip.String()))
				return ip.String(), nil
			}
		}
	}

	// Method 3: Fall back to private IPs if no public IP found
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if ip == nil || ip.IsLoopback() {
				continue
			}

			// Use any IPv4 address
			if ip.To4() != nil {
				r.logger.Debug("Found private IP", zap.String("ip", ip.String()))
				return ip.String(), nil
			}
		}
	}

	return "", fmt.Errorf("no suitable IP address found")
}

// testJoinAddress tests if a join address is reachable
func (r *RQLiteManager) testJoinAddress(joinAddress string) error {
	// Determine the HTTP status URL to probe.
	// If joinAddress contains a scheme, use it directly. Otherwise treat joinAddress
	// as host:port (Raft) and probe the standard HTTP API port 5001 on that host.
	client := &http.Client{Timeout: 5 * time.Second}

	var statusURL string
	if strings.HasPrefix(joinAddress, "http://") || strings.HasPrefix(joinAddress, "https://") {
		statusURL = strings.TrimRight(joinAddress, "/") + "/status"
	} else {
		// Extract host from host:port
		host := joinAddress
		if idx := strings.Index(joinAddress, ":"); idx != -1 {
			host = joinAddress[:idx]
		}
		statusURL = fmt.Sprintf("http://%s:%d/status", host, 5001)
	}

	r.logger.Debug("Testing join target via HTTP", zap.String("url", statusURL))
	resp, err := client.Get(statusURL)
	if err != nil {
		return fmt.Errorf("failed to connect to leader HTTP at %s: %w", statusURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("leader HTTP at %s returned status %d", statusURL, resp.StatusCode)
	}

	r.logger.Info("Leader HTTP reachable", zap.String("status_url", statusURL))
	return nil
}

