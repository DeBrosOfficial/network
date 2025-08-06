package database

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

// Get the external IP address for advertising
	externalIP, err := r.getExternalIP()
	if err != nil {
		r.logger.Warn("Failed to get external IP, using localhost", zap.Error(err))
		externalIP = "localhost"
	}
	r.logger.Info("Using external IP for RQLite advertising", zap.String("ip", externalIP))

	// Build RQLite command
	args := []string{
		"-http-addr", fmt.Sprintf("0.0.0.0:%d", r.config.RQLitePort),
		"-raft-addr", fmt.Sprintf("0.0.0.0:%d", r.config.RQLiteRaftPort),
		"-auth", "/opt/debros/configs/rqlite-users.json", // Enable authentication
	}

	// Add advertised addresses if we have an external IP
	if externalIP != "localhost" {
		args = append(args, "-http-adv-addr", fmt.Sprintf("%s:%d", externalIP, r.config.RQLitePort))
		args = append(args, "-raft-adv-addr", fmt.Sprintf("%s:%d", externalIP, r.config.RQLiteRaftPort))
	}

	// Add join address if specified (for non-bootstrap or secondary bootstrap nodes)
	if r.config.RQLiteJoinAddress != "" {
		r.logger.Info("Joining RQLite cluster", zap.String("join_address", r.maskCredentials(r.config.RQLiteJoinAddress)))
		
		// Check for authenticated join address with credentials
		joinAddress := r.config.RQLiteJoinAddress
		if !strings.Contains(joinAddress, "@") {
			// Try to load authentication credentials for cluster joining
			if authAddr := r.loadAuthenticatedJoinAddress(); authAddr != "" {
				joinAddress = authAddr
				r.logger.Info("Using authenticated cluster join address")
			}
		}
		
		// Validate join address format before using it
		if strings.HasPrefix(joinAddress, "http://") {
			// Test connectivity and log the results, but always attempt to join
			if err := r.testJoinAddress(r.stripCredentials(joinAddress)); err != nil {
				r.logger.Warn("Join address connectivity test failed, but will still attempt to join",
					zap.String("join_address", r.maskCredentials(joinAddress)),
					zap.Error(err))
			} else {
				r.logger.Info("Join address is reachable, proceeding with cluster join")
			}
			// Always add the join parameter - let RQLite handle retries
			args = append(args, "-join", joinAddress)
		} else {
			r.logger.Warn("Invalid join address format, skipping join", zap.String("address", r.maskCredentials(joinAddress)))
			return fmt.Errorf("invalid RQLite join address format: %s (must start with http://)", r.maskCredentials(joinAddress))
		}
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
		zap.String("external_ip", externalIP),
		zap.Strings("full_args", args),
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
	// Test connection to the join address with a short timeout
	client := &http.Client{Timeout: 5 * time.Second}
	
	// Try to connect to the status endpoint
	statusURL := joinAddress + "/status"
	r.logger.Debug("Testing join address", zap.String("url", statusURL))
	
	resp, err := client.Get(statusURL)
	if err != nil {
		return fmt.Errorf("failed to connect to join address %s: %w", joinAddress, err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("join address %s returned status %d", joinAddress, resp.StatusCode)
	}
	
	r.logger.Info("Join address is reachable", zap.String("address", joinAddress))
	return nil
}

// loadAuthenticatedJoinAddress loads authentication credentials and creates authenticated join URL
func (r *RQLiteManager) loadAuthenticatedJoinAddress() string {
	// Try to load authentication credentials from environment file
	if data, err := os.ReadFile("/opt/debros/configs/rqlite-env"); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "RQLITE_JOIN_ADDRESS_AUTH=") {
				authAddr := strings.TrimPrefix(line, "RQLITE_JOIN_ADDRESS_AUTH=")
				authAddr = strings.Trim(authAddr, `"`)
				if authAddr != "" {
					return authAddr
				}
			}
		}
	}
	
	// Fallback: try to construct from separate user/pass environment variables
	if data, err := os.ReadFile("/opt/debros/keys/rqlite-cluster-auth"); err == nil {
		lines := strings.Split(string(data), "\n")
		var user, pass string
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "RQLITE_CLUSTER_USER=") {
				user = strings.TrimPrefix(line, "RQLITE_CLUSTER_USER=")
				user = strings.Trim(user, `"`)
			} else if strings.HasPrefix(line, "RQLITE_CLUSTER_PASS=") {
				pass = strings.TrimPrefix(line, "RQLITE_CLUSTER_PASS=")
				pass = strings.Trim(pass, `"`)
			}
		}
		
		if user != "" && pass != "" && r.config.RQLiteJoinAddress != "" {
			// Extract base URL and add credentials
			baseURL := r.config.RQLiteJoinAddress
			if strings.HasPrefix(baseURL, "http://") {
				// Insert credentials: http://user:pass@host:port
				host := strings.TrimPrefix(baseURL, "http://")
				return fmt.Sprintf("http://%s:%s@%s", user, pass, host)
			}
		}
	}
	
	return "" // No credentials found
}

// maskCredentials masks authentication credentials in URLs for logging
func (r *RQLiteManager) maskCredentials(url string) string {
	if strings.Contains(url, "@") {
		// URL contains credentials: http://user:pass@host:port
		parts := strings.SplitN(url, "@", 2)
		if len(parts) == 2 {
			protocolAndCreds := parts[0]
			hostAndPort := parts[1]
			
			// Extract protocol
			protocolParts := strings.SplitN(protocolAndCreds, "://", 2)
			if len(protocolParts) == 2 {
				protocol := protocolParts[0]
				return fmt.Sprintf("%s://***:***@%s", protocol, hostAndPort)
			}
		}
	}
	return url
}

// stripCredentials removes authentication credentials from URLs for connectivity testing
func (r *RQLiteManager) stripCredentials(url string) string {
	if strings.Contains(url, "@") {
		// URL contains credentials: http://user:pass@host:port
		parts := strings.SplitN(url, "@", 2)
		if len(parts) == 2 {
			protocolAndCreds := parts[0]
			hostAndPort := parts[1]
			
			// Extract protocol
			protocolParts := strings.SplitN(protocolAndCreds, "://", 2)
			if len(protocolParts) == 2 {
				protocol := protocolParts[0]
				return fmt.Sprintf("%s://%s", protocol, hostAndPort)
			}
		}
	}
	return url
}
