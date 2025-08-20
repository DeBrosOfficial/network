# DeBros Network - Distributed P2P Database System

A robust, decentralized peer-to-peer network built in Go, providing distributed SQL database, key-value storage, pub/sub messaging, and resilient peer management. Designed for applications needing reliable, scalable, and secure data sharing without centralized infrastructure.

---

## Table of Contents

- [Features](#features)
- [Architecture Overview](#architecture-overview)
- [System Requirements](#system-requirements)
- [Quick Start](#quick-start)
- [Deployment & Installation](#deployment--installation)
- [Configuration](#configuration)
- [CLI Usage](#cli-usage)
- [HTTP Gateway](#http-gateway)
- [Development](#development)
- [Troubleshooting](#troubleshooting)
- [License](#license)

---

## Features

- **Distributed SQL Database:** RQLite-backed, Raft-consensus, ACID transactions, automatic failover.
- **Key-Value Storage:** Namespaced, replicated, CRUD operations, prefix queries.
- **Pub/Sub Messaging:** Topic-based, real-time, namespaced, automatic cleanup.
- **Peer Discovery & Management:** Nodes discover peers, bootstrap support, health monitoring.
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
│ ┌─────────────┐ ┌─────────────┐ ┌────────────────────────┐ │
│ │  Database   │ │   Storage   │ │        PubSub         │ │
│ │   Client    │ │   Client    │ │        Client         │ │
│ └─────────────┘ └─────────────┘ └────────────────────────┘ │
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

- **Node:** Full P2P participant, runs services, handles peer discovery, database, storage, pubsub.
- **Client:** Lightweight, connects only to bootstrap peers, consumes services, no peer discovery.

---

## System Requirements

### Software

- **Go:** 1.21+ (recommended)
- **RQLite:** 8.x (distributed SQLite)
- **Git:** For source management
- **Make:** For build automation (recommended)

### Hardware

- **Minimum:** 2 CPU cores, 4GB RAM, 10GB disk, stable internet
- **Recommended:** 4+ cores, 8GB+ RAM, 50GB+ SSD, low-latency network

### Network Ports

- **4001:** LibP2P P2P communication
- **5001:** RQLite HTTP API
- **7001:** RQLite Raft consensus

---

## Quick Start

### 1. Clone and Setup

```bash
git clone https://git.debros.io/DeBros/network.git
cd network
```

### 2. Build All Executables

```bash
make build
```

### 3. Start a Bootstrap Node

```bash
make run-node
# Or manually:
go run ./cmd/node --config configs/bootstrap.yaml
```

### 4. Start Additional Nodes

```bash
make run-node2
# Or manually:
go run ./cmd/node --config configs/node.yaml
```

### 5. Test with CLI

```bash
./bin/network-cli health
./bin/network-cli peers
./bin/network-cli storage put test-key "Hello Network"
./bin/network-cli storage get test-key
./bin/network-cli pubsub publish notifications "Hello World"
./bin/network-cli pubsub subscribe notifications 10s
```

---

## Deployment & Installation

### Automated Production Install

Run the install script for a secure, production-ready setup:

```bash
curl -sSL https://git.debros.io/DeBros/network/raw/branch/main/scripts/install-debros-network.sh | sudo bash
```

**What the Script Does:**
- Detects OS, installs Go, RQLite, dependencies
- Creates `debros` system user, secure directory structure
- Generates LibP2P identity keys
- Clones source, builds binaries
- Sets up systemd service (`debros-node`)
- Configures firewall (UFW) for required ports
- Generates YAML config in `/opt/debros/configs/node.yaml`

**Directory Structure:**
```
/opt/debros/
├── bin/           # Binaries
├── configs/       # YAML configs
├── keys/          # Identity keys
├── data/          # RQLite DB, storage
├── logs/          # Node logs
├── src/           # Source code
```

**Service Management:**
```bash
sudo systemctl status debros-node
sudo systemctl start debros-node
sudo systemctl stop debros-node
sudo systemctl restart debros-node
sudo journalctl -u debros-node.service -f
```

---

## Configuration

### Example Configuration Files

#### `configs/bootstrap.yaml`

```yaml
node:
  id: ""
  listen_addresses:
    - "/ip4/0.0.0.0/tcp/4001"
  data_dir: "./data/bootstrap"
  max_connections: 100
  disable_anonrc: true

database:
  data_dir: "./data/db"
  replication_factor: 3
  shard_count: 16
  max_database_size: 1073741824
  backup_interval: 24h
  rqlite_port: 5001
  rqlite_raft_port: 7001
  rqlite_join_address: "" # Bootstrap node does not join

discovery:
  bootstrap_peers: []
  discovery_interval: 15s
  bootstrap_port: 4001
  http_adv_address: "127.0.0.1"
  raft_adv_address: ""

security:
  enable_tls: false
  private_key_file: ""
  certificate_file: ""
  auth_enabled: false

logging:
  level: "info"
  format: "console"
  output_file: ""
```

#### `configs/node.yaml`

```yaml
node:
  id: "node2"
  listen_addresses:
    - "/ip4/0.0.0.0/tcp/4002"
  data_dir: "./data/node2"
  max_connections: 50
  disable_anonrc: true

database:
  data_dir: "./data/db"
  replication_factor: 3
  shard_count: 16
  max_database_size: 1073741824
  backup_interval: 24h
  rqlite_port: 5002
  rqlite_raft_port: 7002
  rqlite_join_address: "http://127.0.0.1:5001"

discovery:
  bootstrap_peers:
    - "/ip4/127.0.0.1/tcp/4001/p2p/<YOUR_BOOTSTRAP_PEER_ID>"
  discovery_interval: 15s
  bootstrap_port: 4002
  http_adv_address: "127.0.0.1"
  raft_adv_address: ""

security:
  enable_tls: false
  private_key_file: ""
  certificate_file: ""
  auth_enabled: false

logging:
  level: "info"
  format: "console"
  output_file: ""
```

### Flags & Environment Variables

- **Flags**: Override config at startup (`--data`, `--p2p-port`, `--rqlite-http-port`, etc.)
- **Env Vars**: Override config and flags (`NODE_ID`, `RQLITE_PORT`, `BOOTSTRAP_PEERS`, etc.)
- **Precedence**: Flags > Env Vars > YAML > Defaults

### Bootstrap & Database Endpoints

- **Bootstrap peers**: Set in config or via `BOOTSTRAP_PEERS` env var.
- **Database endpoints**: Set in config or via `RQLITE_NODES` env var.
- **Development mode**: Use `NETWORK_DEV_LOCAL=1` for localhost defaults.

---

## CLI Usage

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
--timeout 30s                 # Set operation timeout
--bootstrap <multiaddr>       # Override bootstrap peer
--production                  # Use production bootstrap peers
--disable-anonrc              # Disable anonymous routing (Tor/SOCKS5)
```

### Authentication

The CLI features an enhanced authentication system with automatic wallet detection and multi-wallet support:

- **Automatic Authentication:** No manual auth commands required - authentication happens automatically when operations need credentials
- **Multi-Wallet Management:** Seamlessly switch between multiple wallet credentials
- **Persistent Sessions:** Wallet credentials are automatically saved and restored between sessions
- **Enhanced User Experience:** Streamlined authentication flow with better error handling and user feedback

When using operations that require authentication (storage, database, pubsub), the CLI will automatically:
1. Check for existing valid credentials
2. Prompt for wallet authentication if needed
3. Handle signature verification
4. Persist credentials for future use

**Example with automatic authentication:**
```bash
# First time - will prompt for wallet authentication
./bin/network-cli storage put user:123 "John Doe"

# Subsequent calls - uses saved credentials automatically
./bin/network-cli storage get user:123
./bin/network-cli pubsub publish notifications "Hello World"
```

---

## HTTP Gateway

The DeBros Network includes a powerful HTTP/WebSocket gateway that provides a modern REST API and WebSocket interface over the P2P network, featuring an enhanced authentication system with multi-wallet support.

### Quick Start

```bash
make run-gateway
# Or manually:
go run ./cmd/gateway
```

### Configuration

The gateway can be configured via environment variables:

```bash
# Basic Configuration
export GATEWAY_ADDR="0.0.0.0:8080"
export GATEWAY_NAMESPACE="my-app"
export GATEWAY_BOOTSTRAP_PEERS="/ip4/127.0.0.1/tcp/4001/p2p/YOUR_PEER_ID"

# Authentication Configuration
export GATEWAY_REQUIRE_AUTH=true
export GATEWAY_API_KEYS="key1:namespace1,key2:namespace2"
```

### Enhanced Authentication System

The gateway features a significantly improved authentication system with the following capabilities:

#### Key Features
- **Automatic Authentication:** No manual auth commands required - authentication happens automatically when needed
- **Multi-Wallet Support:** Seamlessly manage multiple wallet credentials with automatic switching
- **Persistent Sessions:** Wallet credentials are automatically saved and restored
- **Enhanced User Experience:** Streamlined authentication flow with better error handling

#### Authentication Methods

**Wallet-Based Authentication (Ethereum EIP-191)**
- Uses `personal_sign` for secure wallet verification
- Supports multiple wallets with automatic detection
- Addresses are case-insensitive with normalized signature handling

**JWT Tokens**
- Issued by the gateway with configurable expiration
- JWKS endpoints available at `/v1/auth/jwks` and `/.well-known/jwks.json`
- Automatic refresh capability

**API Keys**
- Support for pre-configured API keys via `Authorization: Bearer <key>` or `X-API-Key` headers
- Optional namespace mapping for multi-tenant applications

### API Endpoints

#### Health & Status
```http
GET /health                 # Basic health check
GET /v1/health             # Detailed health status
GET /v1/status             # Network status
GET /v1/version            # Version information
```

#### Authentication (Public Endpoints)
```http
POST /v1/auth/challenge    # Generate wallet challenge
POST /v1/auth/verify       # Verify wallet signature
POST /v1/auth/register     # Register new wallet
POST /v1/auth/refresh      # Refresh JWT token
POST /v1/auth/logout       # Clear authentication
GET  /v1/auth/whoami       # Current auth status
POST /v1/auth/api-key      # Generate API key (authenticated)
```

#### Storage Operations
```http
POST /v1/storage/get       # Retrieve data
POST /v1/storage/put       # Store data
POST /v1/storage/delete    # Delete data
GET  /v1/storage/list      # List keys with optional prefix
GET  /v1/storage/exists    # Check key existence
```

#### Network Operations
```http
GET  /v1/network/status    # Network status
GET  /v1/network/peers     # Connected peers
POST /v1/network/connect   # Connect to peer
POST /v1/network/disconnect # Disconnect from peer
```

#### Pub/Sub Messaging

**WebSocket Interface**
```http
GET /v1/pubsub/ws?topic=<topic>  # WebSocket connection for real-time messaging
```

**REST Interface**
```http
POST /v1/pubsub/publish    # Publish message to topic
GET  /v1/pubsub/topics     # List active topics
```

### Security Features

- **Namespace Enforcement:** All operations are automatically prefixed with namespace for isolation
- **CORS Support:** Configurable CORS policies (permissive for development, configurable for production)
- **Transport Security:** All network communications use Noise/TLS encryption
- **Authentication Middleware:** Flexible authentication with support for multiple credential types

### Usage Examples

#### Wallet Authentication Flow
```bash
# 1. Get challenge (automatic)
curl -X POST http://localhost:8080/v1/auth/challenge

# 2. Sign challenge with wallet (handled by client)
# 3. Verify signature (automatic)
curl -X POST http://localhost:8080/v1/auth/verify \
  -H "Content-Type: application/json" \
  -d '{"wallet":"0x...","nonce":"...","signature":"0x..."}'
```

#### Storage Operations
```bash
# Store data
curl -X POST http://localhost:8080/v1/storage/put \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"key":"user:123","value":"eyJuYW1lIjoiSm9obiJ9"}'

# Retrieve data
curl -X POST http://localhost:8080/v1/storage/get \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"key":"user:123"}'
```

#### Real-time Messaging
```javascript
// WebSocket connection
const ws = new WebSocket('ws://localhost:8080/v1/pubsub/ws?topic=chat');

ws.onmessage = (event) => {
  console.log('Received:', event.data);
};

// Send message
ws.send('Hello, network!');
```

---

## Development
</text>


### Project Structure

```
network/
├── cmd/
│   ├── node/         # Network node executable
│   └── cli/          # Command-line interface
├── pkg/
│   ├── client/       # Client library
│   ├── node/         # Node implementation
│   ├── database/     # RQLite integration
│   ├── storage/      # Storage service
│   ├── pubsub/       # Pub/Sub messaging
│   ├── config/       # Centralized config
│   └── discovery/    # Peer discovery (node only)
├── scripts/          # Install, test scripts
├── configs/          # YAML configs
├── bin/              # Built executables
```

### Build & Test

```bash
make build           # Build all executables
make test            # Run unit tests
make clean           # Clean build artifacts
```

### Local Multi-Node Testing

```bash
scripts/test-multinode.sh
```

---

## Troubleshooting

### Common Issues

#### Bootstrap Connection Failed

- **Symptoms:** `Failed to connect to bootstrap peer`
- **Solutions:** Check node is running, firewall settings, peer ID validity.

#### Database Operations Timeout

- **Symptoms:** `Query timeout` or `No RQLite connection available`
- **Solutions:** Ensure RQLite ports are open, leader election completed, cluster join config correct.

#### Message Delivery Failures

- **Symptoms:** Messages not received by subscribers
- **Solutions:** Verify topic names, active subscriptions, network connectivity.

#### High Memory Usage

- **Symptoms:** Memory usage grows continuously
- **Solutions:** Unsubscribe when done, monitor connection pool, review message retention.

#### Authentication Issues

- **Symptoms:** `Authentication failed`, `Invalid wallet signature`, `JWT token expired`
- **Solutions:** 
  - Check wallet signature format (65-byte r||s||v hex)
  - Ensure nonce matches exactly during wallet verification
  - Verify wallet address case-insensitivity
  - Use refresh endpoint or re-authenticate for expired tokens
  - Clear credential cache if multi-wallet conflicts occur: `rm -rf ~/.debros/credentials`

#### Gateway Issues

- **Symptoms:** `Gateway connection refused`, `CORS errors`, `WebSocket disconnections`
- **Solutions:**
  - Verify gateway is running and accessible on configured port
  - Check CORS configuration for web applications
  - Ensure proper authentication headers for protected endpoints
  - Verify namespace configuration and enforcement

#### Database Migration Issues

- **Symptoms:** `Migration failed`, `SQL syntax error`, `Version conflict`
- **Solutions:**
  - Check SQL syntax in migration files
  - Ensure proper statement termination
  - Verify migration file naming and sequential order
  - Review migration logs for transaction rollbacks

### Debugging & Health Checks

```bash
export LOG_LEVEL=debug
./bin/network-cli health
./bin/network-cli peers
./bin/network-cli query "SELECT 1"
./bin/network-cli pubsub publish test "hello"
./bin/network-cli pubsub subscribe test 10s

# Test authentication flow
./bin/network-cli storage put test-key test-value

# Gateway health checks
curl http://localhost:8080/health
curl http://localhost:8080/v1/status
```

### Service Logs

```bash
# Node service logs
sudo journalctl -u debros-node.service --since "1 hour ago"

# Gateway service logs (if running as service)
sudo journalctl -u debros-gateway.service --since "1 hour ago"

# Application logs
tail -f ./logs/gateway.log
tail -f ./logs/node.log
```

---

## License

Distributed under the MIT License. See [LICENSE](LICENSE) for details.

---

## Further Reading

- [DeBros Network Documentation](https://docs.debros.io)
- [RQLite Documentation](https://github.com/rqlite/rqlite)
- [LibP2P Documentation](https://libp2p.io)

---

_This README reflects the latest architecture, configuration, and operational practices for the DeBros Network. For questions or contributions, please open an issue or pull request._
