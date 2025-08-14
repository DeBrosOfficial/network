# AI Context - DeBros Network Cluster

## Table of Contents

- [Project Overview](#project-overview)
- [Architecture Overview](#architecture-overview)
- [Codebase Structure](#codebase-structure)
- [Key Components](#key-components)
- [Configuration System](#configuration-system)
- [Node vs Client Roles](#node-vs-client-roles)
- [Network Protocol & Data Flow](#network-protocol--data-flow)
- [Build & Development](#build--development)
- [API Reference](#api-reference)
- [Troubleshooting](#troubleshooting)
- [Example Application: Anchat](#example-application-anchat)

---

## Project Overview

**DeBros Network Cluster** is a decentralized peer-to-peer (P2P) system built in Go, providing distributed database operations, key-value storage, pub/sub messaging, and peer management. It is designed for resilient, distributed data management and communication, with a clear separation between full network nodes and lightweight clients.

---

## Architecture Overview

The architecture is modular and robust, supporting both full nodes (which run core services and participate in discovery) and lightweight clients (which connect to the network via bootstrap peers).

```
┌─────────────────────────────────────────────────────────────┐
│                DeBros Network Cluster                      │
├─────────────────────────────────────────────────────────────┤
│                  Application Layer                         │
│  ┌─────────────┐ ┌─────────────┐ ┌───────────────────────┐ │
│  │   Anchat    │ │ Custom App  │ │      CLI Tools        │ │
│  └─────────────┘ └─────────────┘ └───────────────────────┘ │
├─────────────────────────────────────────────────────────────┤
│                    Client API                              │
│  ┌─────────────┐ ┌─────────────┐ ┌───────────────────────┐ │
│  │  Database   │ │   Storage   │ │       PubSub          │ │
│  │   Client    │ │   Client    │ │       Client          │ │
│  └─────────────┘ └─────────────┘ └───────────────────────┘ │
├─────────────────────────────────────────────────────────────┤
│                    Network Layer                           │
│  ┌─────────────┐ ┌─────────────┐ ┌───────────────────────┐ │
│  │   Node      │ │   Discovery │ │      PubSub           │ │
│  │ (Full P2P)  │ │  Manager    │ │      Manager          │ │
│  └─────────────┘ └─────────────┘ └───────────────────────┘ │
├─────────────────────────────────────────────────────────────┤
│                  Database Layer (RQLite)                   │
│  ┌─────────────┐                                          │
│  │   RQLite    │                                          │
│  │ Consensus   │                                          │
│  └─────────────┘                                          │
└─────────────────────────────────────────────────────────────┘
```

**Key Principles:**
- **Modularity:** Each component is independently testable and replaceable.
- **Fault Tolerance:** Network continues operating with node failures.
- **Security:** End-to-end encryption, peer authentication, and namespace isolation.
- **Performance:** Optimized for common operations, with connection pooling and caching.

---

## Codebase Structure

```
network/
├── cmd/                 # Executables
│   ├── node/            # Network node (full participant)
│   │   └── main.go      # Node entrypoint
│   └── cli/             # Command-line interface
│       └── main.go      # CLI entrypoint
├── pkg/                 # Core packages
│   ├── client/          # Lightweight client API
│   ├── node/            # Full node implementation
│   ├── config/          # Centralized configuration management
│   ├── database/        # RQLite integration
│   ├── storage/         # Distributed key-value storage
│   ├── pubsub/          # Pub/Sub messaging
│   ├── discovery/       # Peer discovery (node only)
│   ├── logging/         # Structured and colored logging
│   └── anyoneproxy/     # Optional SOCKS5 proxy support
├── configs/             # YAML configuration files
│   ├── node.yaml        # Node config
│   └── bootstrap.yaml   # Bootstrap config (legacy, now unified)
├── scripts/             # Install and utility scripts
└── data/                # Runtime data (identity, db, logs)
```

---

## Key Components

### 1. **Network Client (`pkg/client/`)**
- **Role:** Lightweight P2P participant for apps and CLI.
- **Features:** Connects only to bootstrap peers, no peer discovery, provides Database, Storage, PubSub, and NetworkInfo interfaces.
- **Isolation:** Namespaced per application.

### 2. **Node (`pkg/node/`)**
- **Role:** Full P2P participant, runs core services (RQLite, storage, pubsub), handles peer discovery and network management.
- **Features:** Peer discovery, service registration, connection monitoring, and data replication.

### 3. **Configuration (`pkg/config/`)**
- **Centralized:** All config is managed via YAML files, with CLI flags and environment variables overriding as needed.
- **Unified:** Node and client configs share structure; bootstrap is just a node with no join address.

### 4. **Database Layer (`pkg/database/`)**
- **RQLite:** Distributed SQLite with Raft consensus, automatic leader election, and failover.
- **Client API:** SQL queries, transactions, schema management.

### 5. **Storage System (`pkg/storage/`)**
- **Distributed KV:** Namespace-isolated, CRUD operations, prefix queries, replication.

### 6. **Pub/Sub System (`pkg/pubsub/`)**
- **Messaging:** Topic-based, real-time delivery, automatic subscription management, namespace isolation.

### 7. **Discovery (`pkg/discovery/`)**
- **Node Only:** Handles peer discovery via peerstore and peer exchange. No DHT/Kademlia in client.

---

## Configuration System

- **Primary Source:** YAML files (`configs/node.yaml`)
- **Overrides:** CLI flags > Environment variables > YAML > Code defaults
- **Examples:**
  - `data_dir`, `key_file`, `listen_addresses`, `solana_wallet`
  - `rqlite_port`, `rqlite_raft_port`, `rqlite_join_address`
  - `bootstrap_peers`, `discovery_interval`
  - Logging: `level`, `file`

**Client Configuration Precedence:**
1. Explicit in `ClientConfig`
2. Environment variables (`RQLITE_NODES`, `BOOTSTRAP_PEERS`)
3. Library defaults (from config package)

---

## Node vs Client Roles

### **Node (`pkg/node/`)**
- Runs full network services (RQLite, storage, pubsub)
- Handles peer discovery and network topology
- Participates in consensus and replication
- Manages service lifecycle and monitoring

### **Client (`pkg/client/`)**
- Lightweight participant (does not run services)
- Connects only to known bootstrap peers
- No peer discovery or DHT
- Consumes network services via API (Database, Storage, PubSub, NetworkInfo)
- Used by CLI and application integrations

---

## Network Protocol & Data Flow

### **Connection Establishment**
- **Node:** Connects to bootstrap peers, discovers additional peers, registers services.
- **Client:** Connects only to bootstrap peers.

### **Message Types**
- **Control:** Node status, heartbeats, topology updates
- **Database:** SQL queries, transactions, schema ops
- **Storage:** KV operations, replication
- **PubSub:** Topic subscriptions, published messages

### **Security Model**
- **Transport:** Noise/TLS encryption for all connections
- **Authentication:** Peer identity verification
- **Isolation:** Namespace-based access control

### **Data Flow**
- **Database:** Client → DatabaseClient → RQLite Leader → Raft Consensus → All Nodes
- **Storage:** Client → StorageClient → Node → Replication
- **PubSub:** Client → PubSubClient → Node → Topic Router → Subscribers

---

## Build & Development

### **Prerequisites**
- Go 1.21+
- RQLite
- Git
- Make

### **Build Commands**
```bash
make build        # Build all executables
make test         # Run tests
make run-node     # Start node (auto-detects bootstrap vs regular)
```

### **Development Workflow**
- Use `make run-node` for local development.
- Edit YAML configs for node settings.
- Use CLI for network operations and testing.

---

## API Reference

### **Client Creation**
```go
import "git.debros.io/DeBros/network/pkg/client"

config := client.DefaultClientConfig("my-app")
config.BootstrapPeers = []string{"/ip4/127.0.0.1/tcp/4001/p2p/{PEER_ID}"}
client, err := client.NewClient(config)
err = client.Connect()
defer client.Disconnect()
```

### **Database Operations**
```go
result, err := client.Database().Query(ctx, "SELECT * FROM users")
err := client.Database().CreateTable(ctx, "CREATE TABLE ...")
```

### **Storage Operations**
```go
err := client.Storage().Put(ctx, "key", []byte("value"))
data, err := client.Storage().Get(ctx, "key")
```

### **PubSub Operations**
```go
err := client.PubSub().Subscribe(ctx, "topic", handler)
err := client.PubSub().Publish(ctx, "topic", []byte("msg"))
```

### **Network Information**
```go
status, err := client.Network().GetStatus(ctx)
peers, err := client.Network().GetPeers(ctx)
```

---

## Troubleshooting

### **Common Issues**
- **Bootstrap Connection Failed:** Check peer ID, port, firewall, and node status.
- **Database Timeout:** Ensure RQLite ports are open, leader election is complete, and join address is correct.
- **Message Delivery Failures:** Verify topic names, subscription status, and network connectivity.
- **High Memory Usage:** Unsubscribe from topics when done, monitor connection pool size.

### **Debugging**
- Enable debug logging: `export LOG_LEVEL=debug`
- Check service logs: `sudo journalctl -u debros-node.service -f`
- Use CLI for health and peer checks: `./bin/network-cli health`, `./bin/network-cli peers`

---

## Example Application: Anchat

The `anchat/` directory contains a full-featured decentralized chat app built on DeBros Network. Features include:
- Solana wallet integration
- End-to-end encrypted messaging
- Real-time pub/sub chat rooms
- Persistent history

---

_This document provides a modern, accurate context for understanding the DeBros Network Cluster architecture, configuration, and usage patterns. All details reflect the current codebase and best practices._