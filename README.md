# Network - Distributed P2P Database System v0.12.5-beta

A distributed peer-to-peer network built with Go and LibP2P, providing decentralized database capabilities with RQLite consensus and replication.

## Table of Contents

- [Features](#features)
- [System Requirements](#system-requirements)
  - [Software Dependencies](#software-dependencies)
  - [Installation](#installation)
    - [macOS](#macos)
    - [Ubuntu/Debian](#ubuntudebian)
    - [Windows](#windows)
  - [Hardware Requirements](#hardware-requirements)
  - [Network Ports](#network-ports)
- [Quick Start](#quick-start)
  - [1. Clone and Setup Environment](#1-clone-and-setup-environment)
  - [2. Generate Bootstrap Identity (Development Only)](#2-generate-bootstrap-identity-development-only)
  - [3. Build the Project](#3-build-the-project)
  - [4. Start the Network](#4-start-the-network)
  - [5. Test with CLI](#5-test-with-cli)
- [Deployment](#deployment)
  - [Production Installation Script](#production-installation-script)
    - [One-Command Installation](#one-command-installation)
    - [What the Script Does](#what-the-script-does)
    - [Directory Structure](#directory-structure)
    - [Node Setup](#node-setup)
    - [Service Management](#service-management)
    - [Configuration Files](#configuration-files)
    - [Security Features](#security-features)
    - [Network Discovery](#network-discovery)
    - [Updates and Maintenance](#updates-and-maintenance)
    - [Monitoring and Troubleshooting](#monitoring-and-troubleshooting)
- [Configuration](#configuration)
  - [Bootstrap and Ports (via flags)](#bootstrap-and-ports-via-flags)
- [CLI Commands](#cli-commands)
  - [Network Operations](#network-operations)
  - [Storage Operations](#storage-operations)
  - [Database Operations](#database-operations)
  - [Pub/Sub Messaging](#pubsub-messaging)
  - [CLI Options](#cli-options)
- [Development](#development)
  - [Project Structure](#project-structure)
  - [Building and Testing](#building-and-testing)
  - [Development Workflow](#development-workflow)
  - [Environment Setup](#environment-setup)
  - [Configuration System](#configuration-system)
- [Client Library Usage](#client-library-usage)
- [Troubleshooting](#troubleshooting)
  - [Common Issues](#common-issues)
  - [Debug Commands](#debug-commands)
  - [Environment-specific Issues](#environment-specific-issues)
  - [Configuration Validation](#configuration-validation)
  - [Logs and Data](#logs-and-data)
- [License](#license)

## Features

- **Peer-to-Peer Networking**: Built on LibP2P for robust P2P communication
- **Distributed Database**: RQLite-based distributed SQLite with Raft consensus
- **Automatic Peer Discovery**: Nodes help new peers join the network
- **CLI Tool**: Command-line interface for network operations and testing
- **Client Library**: Simple Go API for applications to interact with the network
- **Application Isolation**: Namespaced storage and messaging per application

## System Requirements

### Software Dependencies

- **Go**: Version 1.21 or later
- **RQLite**: Distributed SQLite database
- **Git**: For cloning the repository
- **Make**: For build automation (optional but recommended)

### Installation

#### macOS

```bash
# Install Homebrew if you don't have it
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"

# Install dependencies
brew install go rqlite git make

# Verify installation
go version    # Should show Go 1.21+
rqlited --version
```

#### Ubuntu/Debian

```bash
# Install Go (latest version)
sudo rm -rf /usr/local/go
wget https://go.dev/dl/go1.21.6.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.21.6.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin

# Install RQLite
wget https://github.com/rqlite/rqlite/releases/download/v8.43.0/rqlite-v8.43.0-linux-amd64.tar.gz
tar -xzf rqlite-v8.43.0-linux-amd64.tar.gz
sudo mv rqlite-v8.43.0-linux-amd64/rqlited /usr/local/bin/

# Install other dependencies
sudo apt update
sudo apt install git make

# Verify installation
go version
rqlited --version
```

#### Windows

```powershell
# Install Go from https://golang.org/dl/
# Install Git from https://git-scm.com/download/win
# Install RQLite from https://github.com/rqlite/rqlite/releases

# Or use Chocolatey
choco install golang git make
# Download RQLite manually from releases page
```

### Hardware Requirements

**Minimum:**

- CPU: 2 cores
- RAM: 4GB
- Storage: 10GB free space
- Network: Stable internet connection

**Recommended:**

- CPU: 4+ cores
- RAM: 8GB+
- Storage: 50GB+ SSD
- Network: Low-latency internet connection

### Network Ports

The system uses these ports by default:

- **4001**: LibP2P communication
- **5001**: RQLite HTTP API
- **7001**: RQLite Raft consensus

Ensure these ports are available or configure firewall rules accordingly.

## Quick Start

### 1. Clone and Setup Environment

```bash
# Clone the repository
git clone https://git.debros.io/DeBros/network.git
cd network
```

### 2. Generate Bootstrap Identity (Development Only)

For development, you need to generate a consistent bootstrap peer identity:

```bash
# Generate bootstrap peer identity
go run scripts/generate-bootstrap-identity.go

# This will create data/bootstrap/identity.key and show the peer ID (and multiaddr)
# Save the printed peer ID to use with the -bootstrap flag
```

**Important:** After generating the bootstrap identity, copy the printed multiaddr
or peer ID for use with the `-bootstrap` flag when starting regular nodes.

### 3. Build the Project

```bash
# Build all network executables
make build
```

### 4. Start the Network

**Terminal 1 - Bootstrap Node:**

```bash
# Start an explicit bootstrap node (LibP2P 4001, RQLite 5001/7001)
go run ./cmd/node -role bootstrap -data ./data/bootstrap
```

**Terminal 2 - Regular Node:**

```bash
# Replace <BOOTSTRAP_PEER_ID> with the ID printed by the identity generator
go run ./cmd/node \
  -role node \
  -id node2 \
  -data ./data/node2 \
  -bootstrap /ip4/127.0.0.1/tcp/4001/p2p/<BOOTSTRAP_PEER_ID> \
  -rqlite-http-port 5002 \
  -rqlite-raft-port 7002
```

**Terminal 3 - Another Node (optional):**

```bash
go run ./cmd/node \
  -role node \
  -id node3 \
  -data ./data/node3 \
  -bootstrap /ip4/127.0.0.1/tcp/4001/p2p/<BOOTSTRAP_PEER_ID> \
  -rqlite-http-port 5003 \
  -rqlite-raft-port 7003
```

### 5. Test with CLI

```bash
# Check current bootstrap configuration
make show-bootstrap

# Check network health
./bin/network-cli health

# Test storage operations
./bin/network-cli storage put test-key "Hello Network"
./bin/network-cli storage get test-key

# List connected peers
./bin/network-cli peers
```

## Deployment

### Production Installation Script

For production deployments on Linux servers, we provide an automated installation script that handles all dependencies, configuration, and service setup.

#### One-Command Installation

```bash
# Download and run the installation script
curl -sSL https://git.debros.io/DeBros/network/raw/branch/main/scripts/install-debros-network.sh | sudo bash
```

#### What the Script Does

1. **System Setup**:

   - Detects OS (Ubuntu/Debian/CentOS/RHEL/Fedora)
   - Installs Go 1.21+ with architecture detection
   - Installs system dependencies (git, make, build tools)
   - Checks port availability (4001, 5001, 7001)

2. **Configuration Wizard**:

   - Solana wallet address for node operator rewards
   - Installation directory (default: `/opt/debros`)
   - Automatic firewall configuration (UFW)

3. **Secure Installation**:

   - Creates dedicated `debros` system user
   - Sets up secure directory structure with proper permissions
   - Generates LibP2P identity keys with secure storage
   - Clones source code and builds binaries

4. **Service Management**:
   - Creates systemd service with security hardening
   - Enables automatic startup and restart on failure
   - Configures structured logging to systemd journal

#### Directory Structure

The script creates a production-ready directory structure:

```
/opt/debros/
├── bin/                    # Compiled binaries
│   ├── bootstrap          # Bootstrap node executable
│   ├── node              # Node executable
│   └── cli               # CLI tools
├── configs/               # Configuration files
│   └── node.yaml         # Node configuration
├── keys/                  # Identity keys (secure 700 permissions)
│   └── node/
│       └── identity.key
├── data/                  # Runtime data
│   └── node/
│       ├── rqlite/       # RQLite database files
│       └── storage/      # P2P storage data
├── logs/                  # Application logs
│   └── node.log
└── src/                   # Source code (for updates)
```

#### Node Setup

The installation script sets up a **network node**:

- Runs on ports: 4001 (P2P), 5001 (RQLite), 7001 (Raft)
- Participates in DHT for peer discovery and data replication
- Can be deployed on any server or VPS

For setup, please run these commands with adequate permissions:

- Ensure you have elevated privileges or run as a user with the necessary permissions for server setup.
- Follow the installation steps correctly to ensure a smooth deployment.

#### Service Management

After installation, manage your node with these commands:

```bash
# Check service status
sudo systemctl status debros-node

# Start/stop/restart service
sudo systemctl start debros-node
sudo systemctl stop debros-node
sudo systemctl restart debros-node

# View real-time logs
sudo journalctl -u debros-node.service -f

# Enable/disable auto-start
sudo systemctl enable debros-node
sudo systemctl disable debros-node

# Use CLI tools
/opt/debros/bin/network-cli health
/opt/debros/bin/network-cli peers
/opt/debros/bin/network-cli storage put key value
```

#### Configuration Files

The script generates YAML configuration files:

**Node Configuration (`/opt/debros/configs/node.yaml`)**:

```yaml
node:
  data_dir: "/opt/debros/data/node"
  key_file: "/opt/debros/keys/node/identity.key"
  listen_addresses:
    - "/ip4/0.0.0.0/tcp/4001"
  solana_wallet: "YOUR_WALLET_ADDRESS"

database:
  rqlite_port: 5001
  rqlite_raft_port: 7001

logging:
  level: "info"
  file: "/opt/debros/logs/node.log"
```

#### Security Features

The installation script implements production security best practices:

- **Dedicated User**: Runs as `debros` system user (not root)
- **File Permissions**: Key files have 600 permissions, directories have proper ownership
- **Systemd Security**: Service runs with `NoNewPrivileges`, `PrivateTmp`, `ProtectSystem=strict`
- **Firewall**: Automatic UFW configuration for required ports
- **Network Isolation**: Proper port management to avoid conflicts

#### Network Discovery

- **Network Peers**: Hardcoded in the application for automatic connection
- **DHT Discovery**: Nodes automatically join Kademlia DHT for peer discovery
- **Peer Exchange**: Connected nodes share information about other peers
- **No Manual Configuration**: Nodes connect automatically without user intervention

#### Updates and Maintenance

```bash
# Update to latest version (re-run the installation script)
curl -sSL https://git.debros.io/DeBros/network/raw/branch/main/scripts/install-debros-network.sh | bash

# Manual source update
cd /opt/debros/src
sudo -u debros git pull
sudo -u debros make build
sudo cp bin/* /opt/debros/bin/
sudo systemctl restart debros-node

# Backup configuration and keys
sudo cp -r /opt/debros/configs /backup/
sudo cp -r /opt/debros/keys /backup/
```

#### Monitoring and Troubleshooting

```bash
# Check if ports are open
sudo netstat -tuln | grep -E "(4001|5001|7001)"

# Check service logs
sudo journalctl -u debros-node.service --since "1 hour ago"

# Check network connectivity
/opt/debros/bin/network-cli health
/opt/debros/bin/network-cli peers

# Check disk usage
du -sh /opt/debros/data/*

# Process information
ps aux | grep debros
```

For more advanced configuration options and development setup, see the sections below.

## Configuration

### Bootstrap and Ports (via flags)

- **Bootstrap node**: `-role bootstrap`
- **Regular node**: `-role node -bootstrap <multiaddr>`
- **RQLite ports**: `-rqlite-http-port` (default 5001), `-rqlite-raft-port` (default 7001)

Examples are shown in Quick Start above for local multi-node on a single machine.

### Environment Variables

Precedence: CLI flags > Environment variables > Code defaults. Set any of the following in your shell or `.env`:

- NODE_ID: custom node identifier (e.g. "node2")
- NODE_TYPE: "bootstrap" or "node"
- NODE_LISTEN_ADDRESSES: comma-separated multiaddrs (e.g. "/ip4/0.0.0.0/tcp/4001,/ip4/0.0.0.0/udp/4001/quic")
- DATA_DIR: node data directory (default `./data`)
- MAX_CONNECTIONS: max peer connections (int)

- DB_DATA_DIR: database data directory (default `./data/db`)
- REPLICATION_FACTOR: int (default 3)
- SHARD_COUNT: int (default 16)
- MAX_DB_SIZE: e.g. "1g", "512m", or bytes
- BACKUP_INTERVAL: Go duration (e.g. "24h")
- RQLITE_HTTP_PORT: int (default 5001)
- RQLITE_RAFT_PORT: int (default 7001)
- RQLITE_JOIN_ADDRESS: host:port for Raft join (regular nodes)
- RQLITE_NODES: comma/space-separated DB endpoints (e.g. "http://n1:5001,http://n2:5001"). Used by client if `ClientConfig.DatabaseEndpoints` is empty.
- RQLITE_PORT: default DB HTTP port for constructing library defaults (fallback 5001)
- NETWORK_DEV_LOCAL: when truthy (1/true/yes/on), client defaults use localhost for DB endpoints; default bootstrap peers also return localhost values.
- LOCAL_BOOTSTRAP_MULTIADDR: when set with NETWORK_DEV_LOCAL, overrides default bootstrap with a specific local multiaddr (e.g. `/ip4/127.0.0.1/tcp/4001/p2p/<ID>`)
- ADVERTISE_MODE: "auto" | "localhost" | "ip"

- BOOTSTRAP_PEERS: comma-separated multiaddrs for bootstrap peers
- ENABLE_MDNS: true/false
- ENABLE_DHT: true/false
- DHT_PREFIX: string (default `/network/kad/1.0.0`)
- DISCOVERY_INTERVAL: duration (e.g. "5m")

- ENABLE_TLS: true/false
- PRIVATE_KEY_FILE: path
- CERT_FILE: path
- AUTH_ENABLED: true/false

- LOG_LEVEL: "debug" | "info" | "warn" | "error"
- LOG_FORMAT: "json" | "console"
- LOG_OUTPUT_FILE: path (empty = stdout)

### Centralized Flag/Env Mapping

Flag and environment variable mapping is centralized in `cmd/node/configmap.go` via `MapFlagsAndEnvToConfig`.
This enforces precedence (flags > env > defaults) consistently across the node startup path.

### Centralized Defaults: Bootstrap & Database

- The network library is the single source of truth for defaults.
- Bootstrap peers: `pkg/constants/bootstrap.go` exposed via `client.DefaultBootstrapPeers()`.
- Database HTTP endpoints: derived from bootstrap peers via `client.DefaultDatabaseEndpoints()`.

#### Database Endpoints Precedence

When the client connects to RQLite, endpoints are resolved with this precedence:

1. `ClientConfig.DatabaseEndpoints` (explicitly set by the app)
2. `RQLITE_NODES` environment variable (comma/space separated), e.g. `http://x:5001,http://y:5001`
3. `client.DefaultDatabaseEndpoints()` (constructed from default bootstrap peers)

Notes:

- Default DB port is 5001. Override with `RQLITE_PORT` when constructing defaults.
- Endpoints are normalized to include scheme and port; duplicates are removed.

#### Client Usage Example

```go
cfg := client.DefaultClientConfig("my-app")
// Optional: override bootstrap peers
cfg.BootstrapPeers = []string{"/ip4/127.0.0.1/tcp/4001/p2p/<PEER_ID>"}
// Optional: prefer explicit DB endpoints
cfg.DatabaseEndpoints = []string{"http://127.0.0.1:5001"}

cli, err := client.NewClient(cfg)
// cli.Connect() will prefer cfg.DatabaseEndpoints, then RQLITE_NODES, then defaults
```

#### Development Mode (localhost-only)

To force localhost defaults for both database endpoints and bootstrap peers:

```bash
export NETWORK_DEV_LOCAL=1
# Optional: specify a local bootstrap peer multiaddr with peer ID
export LOCAL_BOOTSTRAP_MULTIADDR="/ip4/127.0.0.1/tcp/4001/p2p/<BOOTSTRAP_PEER_ID>"
# Optional: customize default DB port used in localhost endpoints
export RQLITE_PORT=5001
```

Notes:

- With `NETWORK_DEV_LOCAL`, `client.DefaultDatabaseEndpoints()` returns `http://127.0.0.1:$RQLITE_PORT`.
- `client.DefaultBootstrapPeers()` returns `LOCAL_BOOTSTRAP_MULTIADDR` if set, otherwise `/ip4/127.0.0.1/tcp/4001`.
- If you construct config via `client.DefaultClientConfig(...)`, DB endpoints are pinned to localhost and will override `RQLITE_NODES` automatically.

### Migration Guide for Apps (e.g., anchat)

- __Stop hardcoding endpoints__: Replace any hardcoded bootstrap peers and DB URLs with calls to
  `client.DefaultBootstrapPeers()` and, if needed, set `ClientConfig.DatabaseEndpoints`.
- __Prefer config over env__: Set `ClientConfig.DatabaseEndpoints` in your app config. If not set,
  the library will read `RQLITE_NODES` for backward compatibility.
- __Keep env compatibility__: Existing environments using `RQLITE_NODES` and `RQLITE_PORT` continue to work.
- __Minimal changes__: Most apps only need to populate `ClientConfig.DatabaseEndpoints` and/or rely on
  `client.DefaultDatabaseEndpoints()`; no other code changes required.

Example migration snippet:

```go
import netclient "git.debros.io/DeBros/network/pkg/client"

cfg := netclient.DefaultClientConfig("anchat")
// Use library defaults for bootstrap peers
cfg.BootstrapPeers = netclient.DefaultBootstrapPeers()
// Prefer explicit DB endpoints (can also leave empty to use env or defaults)
cfg.DatabaseEndpoints = []string{"http://127.0.0.1:5001"}

c, err := netclient.NewClient(cfg)
if err != nil { /* handle */ }
if err := c.Connect(); err != nil { /* handle */ }
defer c.Disconnect()
```

## CLI Commands

The CLI can still accept `--bootstrap <multiaddr>` to override discovery when needed.

### Network Operations

```bash
./bin/network-cli health                    # Check network health
./bin/network-cli status                    # Get network status
./bin/network-cli peers                     # List connected peers
```

### Storage Operations

```bash
./bin/network-cli storage put <key> <value> # Store data
./bin/network-cli storage get <key>         # Retrieve data
./bin/network-cli storage list [prefix]     # List keys
```

### Database Operations

```bash
./bin/network-cli query "SELECT * FROM table"              # Execute SQL
./bin/network-cli query "CREATE TABLE users (id INTEGER)"  # DDL operations
```

### Pub/Sub Messaging

```bash
./bin/network-cli pubsub publish <topic> <message>     # Send message
./bin/network-cli pubsub subscribe <topic> [duration]  # Listen for messages
./bin/network-cli pubsub topics                        # List active topics
```

### CLI Options

```bash
--format json                 # Output in JSON format
--timeout 30s                # Set operation timeout
--bootstrap <multiaddr>      # Override bootstrap peer
```

## Development

### Project Structure

```
network/
├── cmd/
│   ├── node/              # Network node (bootstrap via flag)
│   │   ├── main.go        # Entrypoint
│   │   └── configmap.go   # Centralized flags/env → config mapping
│   └── cli/               # Command-line interface
├── pkg/
│   ├── client/            # Client library
│   ├── node/              # Node implementation
│   ├── database/          # RQLite integration
│   ├── storage/           # Storage service
│   ├── constants/         # Bootstrap configuration
│   └── config/            # System configuration
├── scripts/               # Helper scripts (install, security, tests)
├── bin/                  # Built executables
```

### Building and Testing

```bash
# Build all network executables
make build

# Show current bootstrap configuration
make show-bootstrap

# Run node (auto-detects bootstrap vs regular based on .env)
make run-node

# Clean data directories
make clean

# Run tests
go test ./...

# Full development workflow
make dev
```

### Development Workflow

1. **Initial Setup:**

   ```bash
   # Copy environment templates
   cp .env.example .env

   # Generate consistent bootstrap identity
   go run scripts/generate-bootstrap-identity.go

   # Update .env files with the generated peer ID
   ```

2. **Build Everything:**

   ```bash
   make build        # Build network components
   ```

3. **Start Development Cluster:**

   ```bash
   # Terminal 1: Bootstrap node (auto-detected)
   make run-node

   # Terminal 2: Regular node (auto-connects via .env)
   make run-node

   # Terminal 3: Test with CLI
   ./bin/network-cli health
   ./bin/network-cli peers
   ```

### Environment Setup

1. **Install Dependencies:**

   ```bash
   # macOS
   brew install go rqlite git make

   # Ubuntu/Debian
   sudo apt install golang-go git make
   # Install RQLite from https://github.com/rqlite/rqlite/releases
   ```

2. **Verify Installation:**

   ```bash
   go version      # Should be 1.21+
   rqlited --version
   make --version
   ```

3. **Configure Environment:**

   ```bash
   # Setup .env files
   cp .env.example .env

   # Generate bootstrap identity
   go run scripts/generate-bootstrap-identity.go

   # Update .env files with generated peer ID
   ```

### Configuration System

The network uses a dual configuration system:

1. **Environment Variables (.env files):** Primary configuration method
2. **Hardcoded Constants:** Fallback when .env files are not found

#### Bootstrap Configuration Priority:

1. Command line flags (if provided)
2. Environment variables from `.env` files
3. Hardcoded constants in `pkg/constants/bootstrap.go`
4. Auto-discovery from running bootstrap nodes

This ensures the network can start even without configuration files, while allowing easy customization for different environments.

## Client Library Usage

```go
package main

import (
    "context"
    "log"
    "git.debros.io/DeBros/network/pkg/client"
)

func main() {
    // Create client (bootstrap peer discovered automatically)
    config := client.DefaultClientConfig("my-app")
    networkClient, err := client.NewClient(config)
    if err != nil {
        log.Fatal(err)
    }

    // Connect to network
    if err := networkClient.Connect(); err != nil {
        log.Fatal(err)
    }
    defer networkClient.Disconnect()

    // Use storage
    ctx := context.Background()
    storage := networkClient.Storage()

    err = storage.Put(ctx, "user:123", []byte("user data"))
    if err != nil {
        log.Fatal(err)
    }

    data, err := storage.Get(ctx, "user:123")
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("Retrieved: %s", string(data))
}
```

## Troubleshooting

### Common Issues

**Bootstrap peer not found / Peer ID mismatch:**

- Generate a new bootstrap identity: `go run scripts/generate-bootstrap-identity.go`
- Update `.env` with the new peer ID
- Restart the bootstrap node: `make run-node`
- Check configuration: `make show-bootstrap`

**Nodes can't connect:**

- Verify `.env` files have the correct bootstrap peer ID
- Check that the bootstrap node is running: `ps aux | grep node`
- Verify firewall settings and port availability (4001, 5001, 7001)
- Try restarting with clean data: `make clean && make run-node`

**Storage operations fail:**

- Ensure at least one node is running and connected
- Check network health: `./bin/cli health`
- Verify RQLite is properly installed: `rqlited --version`
- Check for port conflicts: `netstat -an | grep -E "(4001|5001|7001)"`

### Debug Commands

```bash
# Check current configuration
make show-bootstrap
cat .env

# Check running processes
ps aux | grep -E "(bootstrap|node|rqlite)"

# Check port usage
netstat -an | grep -E "(4001|5001|7001)"

# Check bootstrap peer info
cat data/bootstrap/peer.info

# Clean and restart everything
make clean
make run-node      # In one terminal (auto-detects as bootstrap)
make run-node      # In another terminal (runs as regular node)
```

### Environment-specific Issues

**Development Environment:**

- Always run `go run scripts/generate-bootstrap-identity.go` first
- Update `.env` files with the generated peer ID
- Use `make run-node` - the system auto-detects if it should run as bootstrap

**Production Environment:**

- Use stable, external bootstrap peer addresses
- Configure multiple bootstrap peers for redundancy
- Set `ENVIRONMENT=production` in `.env` files

### Configuration Validation

```bash
# Test bootstrap configuration loading
go run -c 'package main; import "fmt"; import "network/pkg/constants"; func main() { fmt.Printf("Bootstrap peers: %v\n", constants.GetBootstrapPeers()) }'

# Verify .env file syntax
grep -E "^[A-Z_]+=.*" .env
```

### Logs and Data

- Node logs: Console output from each running process
- Data directories: `./data/bootstrap/`, `./data/node/`, etc.
- RQLite data: `./data/<node>/rqlite/`
- Peer info: `./data/<node>/peer.info`
- Bootstrap identity: `./data/bootstrap/identity.key`
- Environment config: `./.env`

## License

MIT License - see LICENSE file for details.
