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
  - [6. Test Anchat Messaging](#6-test-anchat-messaging)
- [Deployment](#deployment)
  - [Production Installation Script](#production-installation-script)
    - [One-Command Installation](#one-command-installation)
    - [What the Script Does](#what-the-script-does)
    - [Directory Structure](#directory-structure)
    - [Node Types](#node-types)
    - [Service Management](#service-management)
    - [Configuration Files](#configuration-files)
    - [Security Features](#security-features)
    - [Network Discovery](#network-discovery)
    - [Updates and Maintenance](#updates-and-maintenance)
    - [Monitoring and Troubleshooting](#monitoring-and-troubleshooting)
- [Environment Configuration](#environment-configuration)
  - [Bootstrap Peers Configuration](#bootstrap-peers-configuration)
    - [Setup for Development](#setup-for-development)
    - [Configuration Files](#configuration-files-1)
    - [Multiple Bootstrap Peers](#multiple-bootstrap-peers)
    - [Checking Configuration](#checking-configuration)
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
- [Anchat - Decentralized Messaging Application](#anchat---decentralized-messaging-application)
  - [Features](#features-1)
  - [Quick Start with Anchat](#quick-start-with-anchat)
  - [Anchat Commands](#anchat-commands)
  - [Anchat Configuration](#anchat-configuration)
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
- **Automatic Peer Discovery**: Bootstrap nodes help new peers join the network
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

- **4001-4003**: LibP2P communication
- **5001-5003**: RQLite HTTP API
- **7001-7003**: RQLite Raft consensus

Ensure these ports are available or configure firewall rules accordingly.

## Quick Start

### 1. Clone and Setup Environment

```bash
# Clone the repository
git clone https://git.debros.io/DeBros/network-cluster.git
cd network-cluster

# Copy environment configuration
cp .env.example .env
```

### 2. Generate Bootstrap Identity (Development Only)

For development, you need to generate a consistent bootstrap peer identity:

```bash
# Generate bootstrap peer identity
go run scripts/generate-bootstrap-identity.go

# This will create data/bootstrap/identity.key and show the peer ID
# Copy the peer ID and update your .env files
```

**Important:** After generating the bootstrap identity, update both `.env` files:

```bash
# Update main .env file
nano .env
# Update BOOTSTRAP_PEERS with the generated peer ID

# Update Anchat .env file
nano anchat/.env
# Update BOOTSTRAP_PEERS with the same peer ID
```

### 3. Build the Project

```bash
# Build all network executables
make build

# Build Anchat application
cd anchat
make build
cd ..

# Or build everything at once
make build && make build-anchat
```

### 4. Start the Network

**Terminal 1 - Bootstrap Node:**

```bash
make run-bootstrap
# This starts the bootstrap node on port 4001
```

**Terminal 2 - Regular Node:**

```bash
make run-node
# This automatically connects to bootstrap peers from .env
# No need to specify bootstrap manually anymore!
```

**Terminal 3 - Another Node (optional):**

```bash
# For additional nodes, use different ports
go run cmd/node/main.go -data ./data/node2 -port 4003
```

### 5. Test with CLI

```bash
# Check current bootstrap configuration
make show-bootstrap

# Check network health
./bin/cli health

# Test storage operations
./bin/cli storage put test-key "Hello Network"
./bin/cli storage get test-key

# List connected peers
./bin/cli peers
```

### 6. Test Anchat Messaging

```bash
# Terminal 1 - First user
cd anchat
./bin/anchat

# Terminal 2 - Second user
cd anchat
./bin/anchat
```

## Deployment

### Production Installation Script

For production deployments on Linux servers, we provide an automated installation script that handles all dependencies, configuration, and service setup.

#### One-Command Installation

```bash
# Download and run the installation script
curl -sSL https://raw.githubusercontent.com/DeBrosOfficial/debros-network/main/scripts/install-debros-network.sh | bash
```

#### What the Script Does

1. **System Setup**:

   - Detects OS (Ubuntu/Debian/CentOS/RHEL/Fedora)
   - Installs Go 1.21+ with architecture detection
   - Installs system dependencies (git, make, build tools)
   - Checks port availability (4001-4003, 5001-5003, 7001-7003)

2. **Configuration Wizard**:

   - Node type selection (bootstrap vs regular node)
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
│   ├── node              # Regular node executable
│   └── cli               # CLI tools
├── configs/               # Configuration files
│   ├── bootstrap.yaml    # Bootstrap node config
│   └── node.yaml         # Regular node config
├── keys/                  # Identity keys (secure 700 permissions)
│   ├── bootstrap/
│   │   └── identity.key
│   └── node/
│       └── identity.key
├── data/                  # Runtime data
│   ├── bootstrap/
│   │   ├── rqlite/       # RQLite database files
│   │   └── storage/      # P2P storage data
│   └── node/
│       ├── rqlite/
│       └── storage/
├── logs/                  # Application logs
│   ├── bootstrap.log
│   └── node.log
└── src/                   # Source code (for updates)
```

#### Node Types

**Bootstrap Node**:

- Network entry point that other nodes connect to
- Runs on ports: 4001 (P2P), 5001 (RQLite), 7001 (Raft)
- Should be deployed on stable, publicly accessible servers
- Acts as initial seed for peer discovery

**Regular Node**:

- Connects to bootstrap peers automatically (hardcoded in code)
- Runs on ports: 4002 (P2P), 5002 (RQLite), 7002 (Raft)
- Participates in DHT for peer discovery and data replication
- Can be deployed on any server or VPS

#### Service Management

After installation, manage your node with these commands:

```bash
# Check service status
sudo systemctl status debros-bootstrap  # or debros-node

# Start/stop/restart service
sudo systemctl start debros-bootstrap
sudo systemctl stop debros-bootstrap
sudo systemctl restart debros-bootstrap

# View real-time logs
sudo journalctl -u debros-bootstrap.service -f

# Enable/disable auto-start
sudo systemctl enable debros-bootstrap
sudo systemctl disable debros-bootstrap

# Use CLI tools
/opt/debros/bin/cli health
/opt/debros/bin/cli peers
/opt/debros/bin/cli storage put key value
```

#### Configuration Files

The script generates YAML configuration files:

**Bootstrap Node (`/opt/debros/configs/bootstrap.yaml`)**:

```yaml
node:
  data_dir: "/opt/debros/data/bootstrap"
  key_file: "/opt/debros/keys/bootstrap/identity.key"
  listen_addresses:
    - "/ip4/0.0.0.0/tcp/4001"
  solana_wallet: "YOUR_WALLET_ADDRESS"

database:
  rqlite_port: 5001
  rqlite_raft_port: 7001

logging:
  level: "info"
  file: "/opt/debros/logs/bootstrap.log"
```

**Regular Node (`/opt/debros/configs/node.yaml`)**:

```yaml
node:
  data_dir: "/opt/debros/data/node"
  key_file: "/opt/debros/keys/node/identity.key"
  listen_addresses:
    - "/ip4/0.0.0.0/tcp/4002"
  solana_wallet: "YOUR_WALLET_ADDRESS"

database:
  rqlite_port: 5002
  rqlite_raft_port: 7002

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
- **Network Isolation**: Each node type uses different ports to avoid conflicts

#### Network Discovery

- **Bootstrap Peers**: Hardcoded in the application for automatic connection
- **DHT Discovery**: Nodes automatically join Kademlia DHT for peer discovery
- **Peer Exchange**: Connected nodes share information about other peers
- **No Manual Configuration**: Regular nodes connect automatically without user intervention

#### Updates and Maintenance

```bash
# Update to latest version (re-run the installation script)
curl -sSL https://raw.githubusercontent.com/DeBrosOfficial/debros-network/main/scripts/install-debros-network.sh | bash

# Manual source update
cd /opt/debros/src
sudo -u debros git pull
sudo -u debros make build
sudo cp bin/* /opt/debros/bin/
sudo systemctl restart debros-bootstrap  # or debros-node

# Backup configuration and keys
sudo cp -r /opt/debros/configs /backup/
sudo cp -r /opt/debros/keys /backup/
```

#### Monitoring and Troubleshooting

```bash
# Check if ports are open
sudo netstat -tuln | grep -E "(4001|4002|5001|5002|7001|7002)"

# Check service logs
sudo journalctl -u debros-bootstrap.service --since "1 hour ago"

# Check network connectivity
/opt/debros/bin/cli health
/opt/debros/bin/cli peers

# Check disk usage
du -sh /opt/debros/data/*

# Process information
ps aux | grep debros
```

For more advanced configuration options and development setup, see the sections below.

## Environment Configuration

### Bootstrap Peers Configuration

The network uses `.env` files to configure bootstrap peers automatically. This eliminates the need to manually specify bootstrap peer addresses when starting nodes.

#### Setup for Development

1. **Copy example configuration:**

   ```bash
   cp .env.example .env
   cp anchat/.env.example anchat/.env
   ```

2. **Generate bootstrap identity:**

   ```bash
   go run scripts/generate-bootstrap-identity.go
   ```

3. **Update .env files with the generated peer ID:**

   ```bash
   # Main network .env
   BOOTSTRAP_PEERS=/ip4/127.0.0.1/tcp/4001/p2p/YOUR_GENERATED_PEER_ID

   # Anchat .env
   BOOTSTRAP_PEERS=/ip4/127.0.0.1/tcp/4001/p2p/YOUR_GENERATED_PEER_ID
   ```

#### Configuration Files

**Main Network (.env):**

```bash
# Bootstrap Node Configuration for Development
BOOTSTRAP_PEERS=/ip4/127.0.0.1/tcp/4001/p2p/12D3KooWN3AQHuxAzXfu98tiFYw7W3N2SyDwdxDRANXJp3ktVf8j
BOOTSTRAP_PORT=4001
ENVIRONMENT=development
```

**Anchat Application (anchat/.env):**

```bash
# Anchat Bootstrap Configuration
BOOTSTRAP_PEERS=/ip4/127.0.0.1/tcp/4001/p2p/12D3KooWN3AQHuxAzXfu98tiFYw7W3N2SyDwdxDRANXJp3ktVf8j
BOOTSTRAP_PORT=4001
ENVIRONMENT=development
ANCHAT_LOG_LEVEL=info
ANCHAT_DATABASE_NAME=anchattestingdb1
```

#### Multiple Bootstrap Peers

For production or redundancy, you can specify multiple bootstrap peers:

```bash
BOOTSTRAP_PEERS=/ip4/bootstrap1.example.com/tcp/4001/p2p/12D3KooWPeer1,/ip4/bootstrap2.example.com/tcp/4001/p2p/12D3KooWPeer2,/ip4/127.0.0.1/tcp/4001/p2p/12D3KooWLocalPeer
```

#### Checking Configuration

```bash
# View current bootstrap configuration
make show-bootstrap

# Check which .env file is being used
cat .env
cat anchat/.env
```

## CLI Commands

The CLI and nodes now automatically load bootstrap peers from `.env` files - no manual configuration needed!

### Network Operations

```bash
./bin/cli health                    # Check network health
./bin/cli status                    # Get network status
./bin/cli peers                     # List connected peers
```

### Storage Operations

```bash
./bin/cli storage put <key> <value> # Store data
./bin/cli storage get <key>         # Retrieve data
./bin/cli storage list [prefix]     # List keys
```

### Database Operations

```bash
./bin/cli query "SELECT * FROM table"              # Execute SQL
./bin/cli query "CREATE TABLE users (id INTEGER)"  # DDL operations
```

### Pub/Sub Messaging

```bash
./bin/cli pubsub publish <topic> <message>     # Send message
./bin/cli pubsub subscribe <topic> [duration]  # Listen for messages
./bin/cli pubsub topics                        # List active topics
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
network-cluster/
├── cmd/
│   ├── bootstrap/          # Bootstrap node
│   ├── node/              # Regular network node
│   └── cli/               # Command-line interface
├── pkg/
│   ├── client/            # Client library
│   ├── node/              # Node implementation
│   ├── database/          # RQLite integration
│   ├── storage/           # Storage service
│   ├── constants/         # Bootstrap configuration
│   └── config/            # System configuration
├── anchat/                # Anchat messaging application
│   ├── cmd/cli/          # Anchat CLI
│   ├── pkg/
│   │   ├── chat/         # Chat functionality
│   │   ├── crypto/       # Encryption
│   │   └── constants/    # Anchat bootstrap config
│   ├── .env              # Anchat environment config
│   └── .env.example      # Anchat config template
├── scripts/
│   └── generate-bootstrap-identity.go  # Bootstrap ID generator
├── .env                   # Main environment config
├── .env.example          # Main config template
├── bin/                  # Built executables
├── data/                 # Runtime data directories
└── Makefile             # Build and run commands
```

### Building and Testing

```bash
# Build all network executables
make build

# Build Anchat application
cd anchat && make build && cd ..
# or
make build-anchat

# Show current bootstrap configuration
make show-bootstrap

# Run bootstrap node (uses .env automatically)
make run-bootstrap

# Run regular node (uses .env automatically - no bootstrap flag needed!)
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
   cp anchat/.env.example anchat/.env

   # Generate consistent bootstrap identity
   go run scripts/generate-bootstrap-identity.go

   # Update both .env files with the generated peer ID
   ```

2. **Build Everything:**

   ```bash
   make build        # Build network components
   make build-anchat # Build Anchat application
   ```

3. **Start Development Cluster:**

   ```bash
   # Terminal 1: Bootstrap node
   make run-bootstrap

   # Terminal 2: Regular node (auto-connects via .env)
   make run-node

   # Terminal 3: Test with CLI
   ./bin/cli health
   ./bin/cli peers

   # Terminal 4 & 5: Test Anchat
   cd anchat && ./bin/anchat
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
   cp anchat/.env.example anchat/.env

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
    "network/pkg/client"
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

## Anchat - Decentralized Messaging Application

Anchat is a demonstration application built on the network that provides decentralized, encrypted messaging capabilities.

### Features

- **Decentralized Messaging**: No central servers, messages flow through the P2P network
- **Wallet-based Authentication**: Connect using Solana wallet addresses
- **Encrypted Communications**: End-to-end encryption for private messages
- **Room-based Chat**: Create and join chat rooms
- **Network Auto-discovery**: Automatically finds and connects to other Anchat users

### Quick Start with Anchat

1. **Setup Environment:**

   ```bash
   # Ensure main network is configured
   cp .env.example .env
   cp anchat/.env.example anchat/.env

   # Generate bootstrap identity and update .env files
   go run scripts/generate-bootstrap-identity.go
   ```

2. **Build Anchat:**

   ```bash
   cd anchat
   make build
   ```

3. **Start Network Infrastructure:**

   ```bash
   # Terminal 1: Bootstrap node
   make run-bootstrap

   # Terminal 2: Regular node (optional but recommended)
   make run-node
   ```

4. **Start Anchat Clients:**

   ```bash
   # Terminal 3: First user
   cd anchat
   ./bin/anchat

   # Terminal 4: Second user
   cd anchat
   ./bin/anchat
   ```

### Anchat Commands

```bash
# Room management
/list                    # List available rooms
/join <room>            # Join a room
/leave                  # Leave current room
/create <room> [desc]   # Create a new room

# Messaging
<message>               # Send message to current room
/msg <user> <message>   # Send private message
/me <action>           # Send action message

# User management
/users                  # List users in current room
/nick <username>       # Change username
/who                   # Show your user info

# System
/help                  # Show all commands
/debug                 # Show debug information
/quit                  # Exit Anchat
```

### Anchat Configuration

Anchat uses its own bootstrap configuration in `anchat/.env`:

```bash
# Anchat-specific environment variables
BOOTSTRAP_PEERS=/ip4/127.0.0.1/tcp/4001/p2p/YOUR_BOOTSTRAP_PEER_ID
BOOTSTRAP_PORT=4001
ENVIRONMENT=development
ANCHAT_LOG_LEVEL=info
ANCHAT_DATABASE_NAME=anchattestingdb1
```

The Anchat application also includes hardcoded fallback bootstrap peers in `anchat/pkg/constants/bootstrap.go` for reliability.

## Troubleshooting

### Common Issues

**Bootstrap peer not found / Peer ID mismatch:**

- Generate a new bootstrap identity: `go run scripts/generate-bootstrap-identity.go`
- Update both `.env` and `anchat/.env` with the new peer ID
- Restart the bootstrap node: `make run-bootstrap`
- Check configuration: `make show-bootstrap`

**Nodes can't connect:**

- Verify `.env` files have the correct bootstrap peer ID
- Check that the bootstrap node is running: `ps aux | grep bootstrap`
- Verify firewall settings and port availability (4001, 5001, 7001)
- Try restarting with clean data: `make clean && make run-bootstrap`

**Storage operations fail:**

- Ensure at least one node is running and connected
- Check network health: `./bin/cli health`
- Verify RQLite is properly installed: `rqlited --version`
- Check for port conflicts: `netstat -an | grep -E "(4001|5001|7001)"`

**Anchat clients can't discover each other:**

- Ensure both clients use the same bootstrap peer ID in `anchat/.env`
- Verify the bootstrap node is running
- Check that both clients successfully connect to bootstrap
- Look for "peer id mismatch" errors in the logs

### Debug Commands

```bash
# Check current configuration
make show-bootstrap
cat .env
cat anchat/.env

# Check running processes
ps aux | grep -E "(bootstrap|node|rqlite)"

# Check port usage
netstat -an | grep -E "(4001|4002|4003|5001|5002|5003|7001|7002|7003)"

# Check bootstrap peer info
cat data/bootstrap/peer.info

# Clean and restart everything
make clean
make run-bootstrap  # In one terminal
make run-node      # In another terminal
```

### Environment-specific Issues

**Development Environment:**

- Always run `go run scripts/generate-bootstrap-identity.go` first
- Update `.env` files with the generated peer ID
- Use `make run-node` instead of manual bootstrap specification

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
grep -E "^[A-Z_]+=.*" anchat/.env
```

### Logs and Data

- Node logs: Console output from each running process
- Data directories: `./data/bootstrap/`, `./data/node/`, etc.
- RQLite data: `./data/<node>/rqlite/`
- Peer info: `./data/<node>/peer.info`
- Bootstrap identity: `./data/bootstrap/identity.key`
- Environment config: `./.env`, `./anchat/.env`

## License

MIT License - see LICENSE file for details.
