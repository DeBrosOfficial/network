# AI Context - DeBros Network Cluster

## Table of Contents
- [Project Overview](#project-overview)
- [Product Requirements Document (PRD)](#product-requirements-document-prd)
- [Architecture Overview](#architecture-overview)
- [Codebase Structure](#codebase-structure)
- [Key Components](#key-components)
- [Network Protocol](#network-protocol)
- [Data Flow](#data-flow)
- [Build & Development](#build--development)
- [API Reference](#api-reference)
- [Troubleshooting](#troubleshooting)

## Project Overview

**DeBros Network Cluster** is a decentralized peer-to-peer (P2P) network built in Go that provides distributed database operations, key-value storage, pub/sub messaging, and peer discovery. The system is designed for applications that need resilient, distributed data management without relying on centralized infrastructure.

## Product Requirements Document (PRD)

### Vision
Create a robust, decentralized network platform that enables applications to seamlessly share data, communicate, and discover peers in a distributed environment.

### Core Requirements

#### Functional Requirements
1. **Distributed Database Operations**
   - SQL query execution across network nodes
   - ACID transactions with eventual consistency
   - Schema management and table operations
   - Multi-node resilience with automatic failover

2. **Key-Value Storage**
   - Distributed storage with namespace isolation
   - CRUD operations with consistency guarantees
   - Prefix-based querying and key enumeration
   - Data replication across network participants

3. **Pub/Sub Messaging**
   - Topic-based publish/subscribe communication
   - Real-time message delivery with ordering guarantees
   - Subscription management with automatic cleanup
   - Namespace isolation per application

4. **Peer Discovery & Management**
   - Automatic peer discovery using DHT (Distributed Hash Table)
   - Bootstrap node support for network joining
   - Connection health monitoring and recovery
   - Peer exchange for network growth

5. **Application Isolation**
   - Namespace-based multi-tenancy
   - Per-application data segregation
   - Independent configuration and lifecycle management

#### Non-Functional Requirements
1. **Reliability**: 99.9% uptime with automatic failover
2. **Scalability**: Support 100+ nodes with linear performance
3. **Security**: End-to-end encryption for sensitive data
4. **Performance**: <100ms latency for local operations
5. **Developer Experience**: Simple client API with comprehensive examples

### Success Metrics
- Network uptime > 99.9%
- Peer discovery time < 30 seconds
- Database operation latency < 500ms
- Message delivery success rate > 99.5%

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                    DeBros Network Cluster                   │
├─────────────────────────────────────────────────────────────┤
│                     Application Layer                       │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐ │
│  │   Anchat    │  │ Custom App  │  │    CLI Tools        │ │
│  │   (Chat)    │  │             │  │                     │ │
│  └─────────────┘  └─────────────┘  └─────────────────────┘ │
├─────────────────────────────────────────────────────────────┤
│                      Client API                             │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐ │
│  │  Database   │  │   Storage   │  │      PubSub         │ │
│  │   Client    │  │   Client    │  │      Client         │ │
│  └─────────────┘  └─────────────┘  └─────────────────────┘ │
├─────────────────────────────────────────────────────────────┤
│                    Network Layer                            │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐ │
│  │  Discovery  │  │   PubSub    │  │     Consensus       │ │
│  │   Manager   │  │   Manager   │  │     (RQLite)        │ │
│  └─────────────┘  └─────────────┘  └─────────────────────┘ │
├─────────────────────────────────────────────────────────────┤
│                  Transport Layer                            │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐ │
│  │   LibP2P    │  │     DHT     │  │      RQLite         │ │
│  │   Host      │  │  Kademlia   │  │    Database         │ │
│  └─────────────┘  └─────────────┘  └─────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

### Key Design Principles
1. **Modularity**: Each component can be developed and tested independently
2. **Fault Tolerance**: Network continues operating even with node failures
3. **Consistency**: Strong consistency for database operations, eventual consistency for discovery
4. **Security**: Defense in depth with multiple security layers
5. **Performance**: Optimized for common operations with caching and connection pooling

## Codebase Structure

```
debros-testing/
├── cmd/                          # Executables
│   ├── bootstrap/main.go         # Bootstrap node (network entry point)
│   ├── node/main.go              # Regular network node
│   └── cli/main.go               # Command-line interface
├── pkg/                          # Core packages
│   ├── client/                   # Client API and implementations
│   │   ├── client.go             # Main client implementation
│   │   ├── implementations.go    # Database, storage, network implementations
│   │   └── interface.go          # Public API interfaces
│   ├── config/                   # Configuration management
│   │   └── config.go             # Node and client configuration
│   ├── constants/                # System constants
│   │   └── bootstrap.go          # Bootstrap node constants
│   ├── database/                 # Database layer
│   │   ├── adapter.go            # Database adapter interface
│   │   └── rqlite.go             # RQLite implementation
│   ├── discovery/                # Peer discovery
│   │   └── discovery.go          # DHT-based peer discovery
│   ├── node/                     # Node implementation
│   │   └── node.go               # Network node logic
│   ├── pubsub/                   # Publish/Subscribe messaging
│   │   ├── manager.go            # Core pub/sub logic
│   │   ├── adapter.go            # Client interface adapter
│   │   └── types.go              # Shared types
│   └── storage/                  # Distributed storage
│       ├── client.go             # Storage client
│       ├── protocol.go           # Storage protocol
│       └── service.go            # Storage service
├── anchat/                       # Example chat application
│   ├── cmd/cli/main.go           # Chat CLI
│   └── pkg/
│       ├── chat/manager.go       # Chat message management
│       └── crypto/crypto.go      # End-to-end encryption
├── examples/                     # Usage examples
│   └── basic_usage.go            # Basic API usage
├── configs/                      # Configuration files
│   ├── bootstrap.yaml            # Bootstrap node config
│   └── node.yaml                 # Regular node config
├── data/                         # Runtime data
│   ├── bootstrap/                # Bootstrap node data
│   └── node/                     # Regular node data
└── scripts/                      # Utility scripts
    └── test-multinode.sh         # Multi-node testing
```

## Key Components

### 1. Network Client (`pkg/client/`)
The main entry point for applications to interact with the network.

**Core Interfaces:**
- `NetworkClient`: Main client interface
- `DatabaseClient`: SQL database operations
- `StorageClient`: Key-value storage operations
- `PubSubClient`: Publish/subscribe messaging
- `NetworkInfo`: Network status and peer information

**Key Features:**
- Automatic connection management with retry logic
- Namespace isolation per application
- Health monitoring and status reporting
- Graceful shutdown and cleanup

### 2. Peer Discovery (`pkg/discovery/`)
Handles automatic peer discovery and network topology management.

**Discovery Strategies:**
- **DHT-based**: Uses Kademlia DHT for efficient peer routing
- **Peer Exchange**: Learns about new peers from existing connections
- **Bootstrap**: Connects to known bootstrap nodes for network entry

**Configuration:**
- Discovery interval (default: 10 seconds)
- Maximum concurrent connections (default: 3)
- Connection timeout and retry policies

### 3. Pub/Sub System (`pkg/pubsub/`)
Provides reliable, topic-based messaging with ordering guarantees.

**Features:**
- Topic-based routing with wildcard support
- Namespace isolation per application
- Automatic subscription management
- Message deduplication and ordering

**Message Flow:**
1. Client subscribes to topic with handler
2. Publisher sends message to topic
3. Network propagates message to all subscribers
4. Handlers process messages asynchronously

### 4. Database Layer (`pkg/database/`)
Distributed SQL database built on RQLite (Raft-based SQLite).

**Capabilities:**
- ACID transactions with strong consistency
- Automatic leader election and failover
- Multi-node replication with conflict resolution
- Schema management and migrations

**Query Types:**
- Read operations: Served from any node
- Write operations: Routed to leader node
- Transactions: Atomic across multiple statements

### 5. Storage System (`pkg/storage/`)
Distributed key-value store with eventual consistency.

**Operations:**
- `Put(key, value)`: Store value with key
- `Get(key)`: Retrieve value by key
- `Delete(key)`: Remove key-value pair
- `List(prefix, limit)`: Enumerate keys with prefix
- `Exists(key)`: Check key existence

## Network Protocol

### Connection Establishment
1. **Bootstrap Connection**: New nodes connect to bootstrap peers
2. **DHT Bootstrap**: Initialize Kademlia DHT for routing
3. **Peer Discovery**: Discover additional peers through DHT
4. **Service Registration**: Register available services (database, storage, pubsub)

### Message Types
- **Control Messages**: Node status, heartbeats, topology updates
- **Database Messages**: SQL queries, transactions, schema operations
- **Storage Messages**: Key-value operations, replication data
- **PubSub Messages**: Topic subscriptions, published content

### Security Model
- **Transport Security**: All connections use TLS/Noise encryption
- **Peer Authentication**: Cryptographic peer identity verification
- **Message Integrity**: Hash-based message authentication codes
- **Namespace Isolation**: Application-level access control

## Data Flow

### Database Operation Flow
```
Client App → DatabaseClient → RQLite Leader → Raft Consensus → All Nodes
     ↑                                                              ↓
     └─────────────────── Query Result ←─────────────────────────────┘
```

### Storage Operation Flow
```
Client App → StorageClient → DHT Routing → Target Nodes → Replication
     ↑                                                         ↓
     └─────────────── Response ←─────────────────────────────────┘
```

### PubSub Message Flow
```
Publisher → PubSub Manager → Topic Router → All Subscribers → Message Handlers
```

## Build & Development

### Prerequisites
- Go 1.19+ 
- Make
- Git

### Build Commands
```bash
# Build all executables
make build

# Run tests
make test

# Clean build artifacts
make clean

# Start bootstrap node
make start-bootstrap

# Start regular node
make start-node
```

### Development Workflow
1. **Local Development**: Use `make start-bootstrap` + `make start-node`
2. **Testing**: Run `make test` for unit tests
3. **Integration Testing**: Use `scripts/test-multinode.sh`
4. **Configuration**: Edit `configs/*.yaml` files

### Configuration Files

#### Bootstrap Node (`configs/bootstrap.yaml`)
```yaml
node:
  data_dir: "./data/bootstrap"
  listen_addresses:
    - "/ip4/0.0.0.0/tcp/4001"
    - "/ip4/0.0.0.0/udp/4001/quic"
database:
  rqlite_port: 5001
  rqlite_raft_port: 7001
```

#### Regular Node (`configs/node.yaml`)
```yaml
node:
  data_dir: "./data/node"
  listen_addresses:
    - "/ip4/0.0.0.0/tcp/4002"
discovery:
  bootstrap_peers:
    - "/ip4/127.0.0.1/tcp/4001/p2p/{BOOTSTRAP_PEER_ID}"
  discovery_interval: "10s"
database:
  rqlite_port: 5002
  rqlite_raft_port: 7002
  rqlite_join_address: "http://localhost:5001"
```

## API Reference

### Client Creation
```go
import "network/pkg/client"

config := client.DefaultClientConfig("my-app")
config.BootstrapPeers = []string{"/ip4/127.0.0.1/tcp/4001/p2p/{PEER_ID}"}

client, err := client.NewClient(config)
if err != nil {
    log.Fatal(err)
}

err = client.Connect()
if err != nil {
    log.Fatal(err)
}
defer client.Disconnect()
```

### Database Operations
```go
// Create table
err := client.Database().CreateTable(ctx, `
    CREATE TABLE users (
        id INTEGER PRIMARY KEY,
        name TEXT NOT NULL,
        email TEXT UNIQUE
    )
`)

// Insert data
result, err := client.Database().Query(ctx, 
    "INSERT INTO users (name, email) VALUES (?, ?)",
    "Alice", "alice@example.com")

// Query data
result, err := client.Database().Query(ctx,
    "SELECT id, name, email FROM users WHERE name = ?", "Alice")
```

### Storage Operations
```go
// Store data
err := client.Storage().Put(ctx, "user:123", []byte(`{"name":"Alice"}`))

// Retrieve data
data, err := client.Storage().Get(ctx, "user:123")

// List keys
keys, err := client.Storage().List(ctx, "user:", 10)

// Check existence
exists, err := client.Storage().Exists(ctx, "user:123")
```

### PubSub Operations
```go
// Subscribe to messages
handler := func(topic string, data []byte) error {
    fmt.Printf("Received on %s: %s\n", topic, string(data))
    return nil
}
err := client.PubSub().Subscribe(ctx, "notifications", handler)

// Publish message
err := client.PubSub().Publish(ctx, "notifications", []byte("Hello, World!"))

// List subscribed topics
topics, err := client.PubSub().ListTopics(ctx)
```

### Network Information
```go
// Get network status
status, err := client.Network().GetStatus(ctx)
fmt.Printf("Node ID: %s, Peers: %d\n", status.NodeID, status.PeerCount)

// Get connected peers
peers, err := client.Network().GetPeers(ctx)
for _, peer := range peers {
    fmt.Printf("Peer: %s, Connected: %v\n", peer.ID, peer.Connected)
}

// Connect to specific peer
err := client.Network().ConnectToPeer(ctx, "/ip4/192.168.1.100/tcp/4002/p2p/{PEER_ID}")
```

## Troubleshooting

### Common Issues

#### 1. Bootstrap Connection Failed
**Symptoms**: `Failed to connect to bootstrap peer`
**Solutions**:
- Verify bootstrap node is running and accessible
- Check firewall settings and port availability
- Validate peer ID in bootstrap address

#### 2. Database Operations Timeout
**Symptoms**: `Query timeout` or `No RQLite connection available`
**Solutions**:
- Ensure RQLite ports are not blocked
- Check if leader election has completed
- Verify cluster join configuration

#### 3. Message Delivery Failures
**Symptoms**: Messages not received by subscribers
**Solutions**:
- Verify topic names match exactly
- Check subscription is active before publishing
- Ensure network connectivity between peers

#### 4. High Memory Usage
**Symptoms**: Memory usage grows continuously
**Solutions**:
- Check for subscription leaks (unsubscribe when done)
- Monitor connection pool size
- Review message retention policies

### Debug Mode
Enable debug logging by setting environment variable:
```bash
export LOG_LEVEL=debug
```

### Health Checks
```go
health, err := client.Health()
if health.Status != "healthy" {
    log.Printf("Unhealthy: %+v", health.Checks)
}
```

### Network Diagnostics
```bash
# Check node connectivity
./bin/network-cli peers

# Verify database status
./bin/network-cli query "SELECT 1"

# Test pub/sub
./bin/network-cli pubsub publish test "hello"
./bin/network-cli pubsub subscribe test 10s
```

---

## Example Application: Anchat

The `anchat/` directory contains a complete example application demonstrating how to build a decentralized chat system using the DeBros network. It showcases:

- User registration with Solana wallet integration
- End-to-end encrypted messaging
- IRC-style chat rooms
- Real-time message delivery
- Persistent chat history

This serves as both a practical example and a reference implementation for building applications on the DeBros network platform.

---

*This document provides comprehensive context for AI systems to understand the DeBros Network Cluster project architecture, implementation details, and usage patterns.*