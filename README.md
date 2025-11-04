# DeBros Network - Distributed P2P Database System

A robust, decentralized peer-to-peer network built in Go, providing distributed SQL database, key-value storage, pub/sub messaging, and resilient peer management. Designed for applications needing reliable, scalable, and secure data sharing without centralized infrastructure.

---

## Table of Contents

- [Features](#features)
- [Architecture Overview](#architecture-overview)
- [System Requirements](#system-requirements)
- [Quick Start](#quick-start)



---

## Features

- **Distributed SQL Database:** RQLite-backed, Raft-consensus, ACID transactions, automatic failover.
- **Pub/Sub Messaging:** Topic-based, real-time, namespaced, automatic cleanup.
- **Peer Discovery & Management:** Nodes discover peers, health monitoring.
- **Application Isolation:** Namespace-based multi-tenancy, per-app config.
- **Secure by Default:** Noise/TLS transport, peer identity, systemd hardening.
- **Simple Client API:** Lightweight Go client for apps and CLI tools.

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                  DeBros Network Cluster                    │
├─────────────────────────────────────────────────────────────┤
│                   Application Layer                        │
│ ┌─────────────┐ ┌─────────────┐ ┌────────────────────────┐ │
│ │   Anchat    │ │ Custom App  │ │      CLI Tools        │ │
│ └─────────────┘ └─────────────┘ └────────────────────────┘ │
├─────────────────────────────────────────────────────────────┤
│                      Client API                            │
│ ┌─────────────┐ ┌────────────────────────┐               │
│ │  Database   │ │        PubSub         │               │
│ │   Client    │ │        Client         │               │
│ └─────────────┘ └────────────────────────┘               │
├─────────────────────────────────────────────────────────────┤
│                    Network Node Layer                      │
│ ┌─────────────┐ ┌─────────────┐ ┌────────────────────────┐ │
│ │ Discovery   │ │   PubSub    │ │      Database         │ │
│ │  Manager    │ │   Manager   │ │    (RQLite)          │ │
│ └─────────────┘ └─────────────┘ └────────────────────────┘ │
├─────────────────────────────────────────────────────────────┤
│                  Transport Layer                           │
│ ┌─────────────┐ ┌─────────────┐ ┌────────────────────────┐ │
│ │   LibP2P    │ │   Noise/TLS │ │      RQLite           │ │
│ │   Host      │ │  Encryption │ │    Database           │ │
│ └─────────────┘ └─────────────┘ └────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

- **Node:** Full P2P participant, runs services, handles peer discovery, database, pubsub.
- **Client:** Lightweight, connects only to bootstrap peers, consumes services, no peer discovery.

---

## System Requirements

### Software

- **Go:** 1.21+ (recommended)
- **RQLite:** 8.x (distributed SQLite)
- **Git:** For source management
- **Make:** For build automation (recommended)
- **ANyONe Client:** For onion type routing

### Hardware

- **Minimum:** 2 CPU cores, 4GB RAM, 10GB disk, stable internet
- **Recommended:** 4+ cores, 8GB+ RAM, 50GB+ SSD, low-latency network

### Network Ports

- **4001:** LibP2P P2P communication
- **5001:** RQLite HTTP API
- **7001:** RQLite Raft consensus
- **9050:** ANyONe Client

### Filesystem Permissions

DeBros Network stores all configuration and data in `~/.debros/` directory. Ensure you have:

- **Read/Write access** to your home directory (`~`)
- **Available disk space**: At least 10GB for database and logs
- **No restrictive mount options**: The home directory must not be mounted read-only
- **Unix permissions**: Standard user permissions are sufficient (no root/sudo required)

#### Directory Structure

DeBros automatically creates the following directory structure:

```
~/.debros/
├── bootstrap.yaml          # Bootstrap node config
├── node.yaml               # Node config
├── gateway.yaml            # Gateway config
├── bootstrap/              # Bootstrap node data (auto-created)
│   ├── rqlite/             # RQLite database files
│   │   ├── db.sqlite       # Main database
│   │   ├── raft/           # Raft consensus data
│   │   └── rsnapshots/     # Raft snapshots
│   ├── peer.info           # Node multiaddr (created at startup)
│   └── identity.key        # Node private key (created at startup)
├── node/                   # Node data (auto-created)
│   ├── rqlite/             # RQLite database files
│   ├── raft/               # Raft data
│   ├── peer.info           # Node multiaddr (created at startup)
│   └── identity.key        # Node private key (created at startup)
└── node2/                  # Additional node configs (if running multiple)
    └── rqlite/             # RQLite database files
```

**Files Created at Startup:**
- `identity.key` - LibP2P private key for the node (generated once, reused)
- `peer.info` - The node's multiaddr (e.g., `/ip4/0.0.0.0/tcp/4001/p2p/12D3KooW...`)

**Automatic Creation**: The node automatically creates all necessary data directories when started. You only need to ensure:
1. `~/.debros/` is writable
2. Sufficient disk space available
3. Correct config files exist

**Permission Check:**

```bash
# Verify home directory is writable
touch ~/test-write && rm ~/test-write && echo "✓ Home directory is writable"

# Check available disk space
df -h ~
```

**If you get permission errors:**

```
Error: Failed to create/access config directory
Please ensure:
  1. Home directory is accessible
  2. You have write permissions to home directory
  3. Disk space is available
```

**Solution:**

- Ensure you're not running with overly restrictive umask: `umask` should show `0022` or similar
- Check home directory permissions: `ls -ld ~` should show your user as owner
- For sandboxed/containerized environments: Ensure `/home/<user>` is writable

---

## Quick Start

### 1. Clone and Setup

```bash
git clone https://github.com/DeBrosOfficial/network.git
cd network
```

### 2. Build All Executables

```bash
make build
```

### 3. Generate Configuration Files

```bash
# Generate all configs (bootstrap, node2, node3, gateway) with one command
./bin/network-cli config init
```

This creates:
- `~/.debros/bootstrap.yaml` - Bootstrap node
- `~/.debros/node2.yaml` - Regular node 2
- `~/.debros/node3.yaml` - Regular node 3
- `~/.debros/gateway.yaml` - HTTP Gateway

Plus auto-generated identities for each node.

### 4. Start the Complete Network Stack

```bash
make dev
```

This starts:
- Bootstrap node (P2P: 4001, RQLite HTTP: 5001, Raft: 7001)
- Node 2 (P2P: 4002, RQLite HTTP: 5002, Raft: 7002)
- Node 3 (P2P: 4003, RQLite HTTP: 5003, Raft: 7003)
- Gateway (HTTP: 6001)
- ANyONe Client (9050)

Logs stream to terminal. Press **Ctrl+C** to stop all processes.

### 5. Test with CLI (in another terminal)

```bash
./bin/network-cli health
./bin/network-cli peers
./bin/network-cli pubsub publish notifications "Hello World"
./bin/network-cli pubsub subscribe notifications 10s
```

---


### Running the Network

Once configs are generated, start the complete stack with:

```bash
make dev
```

Or start individual components (in separate terminals):

```bash
# Terminal 1 - Bootstrap node
go run-node

# Terminal 2 - Node 2
go run-node2

# Terminal 3 - Node 3
go run-node3 

# Terminal 4 - Gateway
go run-gateway 
```

### Running Multiple Nodes on the Same Machine

The default `make dev` creates a 3-node setup. For additional nodes, generate individual configs:

```bash
# Generate additional node configs with unique ports
./bin/network-cli config init --type node --name node4.yaml \
  --listen-port 4004 --rqlite-http-port 5004 --rqlite-raft-port 7004 \
  --join localhost:5001 \
  --bootstrap-peers "/ip4/127.0.0.1/tcp/4001/p2p/<BOOTSTRAP_ID>"

# Start the additional node
go run ./cmd/node --config node4.yaml
```

#### Key Points for Multiple Nodes

- **Each node needs unique ports**: P2P port, RQLite HTTP port, and RQLite Raft port must all be different
- **Join address**: Non-bootstrap nodes need `rqlite_join_address` pointing to the bootstrap or an existing node (use Raft port)
- **Bootstrap peers**: All nodes need the bootstrap node's multiaddr in `discovery.bootstrap_peers`
- **Config files**: Store all configs in `~/.debros/` with different filenames
- **--config flag**: Specify which config file to load

⚠️ **Common Mistake - Same Ports:**
If all nodes use the same ports (e.g., 5001, 7001), they will try to bind to the same addresses and fail to communicate. Verify each node has unique ports:

```bash
# Bootstrap
grep "rqlite_port\|rqlite_raft_port" ~/.debros/bootstrap.yaml
# Should show: rqlite_port: 5001, rqlite_raft_port: 7001

# Node 2
grep "rqlite_port\|rqlite_raft_port" ~/.debros/node2.yaml
# Should show: rqlite_port: 5002, rqlite_raft_port: 7002

# Node 3
grep "rqlite_port\|rqlite_raft_port" ~/.debros/node3.yaml
# Should show: rqlite_port: 5003, rqlite_raft_port: 7003
```

If ports are wrong, regenerate the config with `--force`:

```bash
./bin/network-cli config init --type node --name node.yaml \
  --listen-port 4002 --rqlite-http-port 5002 --rqlite-raft-port 7002 \
  --join localhost:5001 --bootstrap-peers '<bootstrap_multiaddr>' --force
```


### Authentication

The CLI features an enhanced authentication system with explicit command support and automatic wallet detection:

#### Explicit Authentication Commands

Use the `auth` command to manage your credentials:

```bash
# Authenticate with your wallet (opens browser for signature)
./bin/network-cli auth login

# Check if you're authenticated
./bin/network-cli auth whoami

# View detailed authentication info
./bin/network-cli auth status

# Clear all stored credentials
./bin/network-cli auth logout
```

Credentials are stored securely in `~/.debros/credentials.json` with restricted file permissions (readable only by owner).

#### Key Features

- **Explicit Authentication:** Use `auth login` command to authenticate with your wallet
- **Automatic Authentication:** Commands that require auth (query, pubsub, etc.) automatically prompt if needed
- **Multi-Wallet Management:** Seamlessly switch between multiple wallet credentials
- **Persistent Sessions:** Wallet credentials are automatically saved and restored between sessions
- **Enhanced User Experience:** Streamlined authentication flow with better error handling and user feedback

#### Automatic Authentication Flow

When using operations that require authentication (query, pubsub publish/subscribe), the CLI will automatically:

1. Check for existing valid credentials
2. Prompt for wallet authentication if needed
3. Handle signature verification
4. Persist credentials for future use

**Example with automatic authentication:**

```bash
# First time - will prompt for wallet authentication when needed
./bin/network-cli pubsub publish notifications "Hello World"
```

#### Environment Variables

You can override the gateway URL used for authentication:

```bash
export DEBROS_GATEWAY_URL="http://localhost:6001"
./bin/network-cli auth login
```

---

## License
