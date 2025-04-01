# @debros/netowrk

Core networking functionality for the Debros decentralized network. This package provides essential IPFS, libp2p, and OrbitDB functionality to build decentralized applications on the Debros network.

## Features

- Pre-configured IPFS/libp2p node setup
- Service discovery for peer-to-peer communication
- OrbitDB integration for distributed databases
- Consistent logging across network components
- Secure key generation

## Installation

```bash
npm install @debros/network
```

## Basic Usage

```typescript
import { initConfig, initIpfs, initOrbitDB, logger } from "@debros/network";

// Initialize with custom configuration (optional)
const config = initConfig({
  env: {
    fingerprint: "my-unique-node-id",
    port: 8080,
  },
  ipfs: {
    bootstrapNodes: "node1,node2,node3",
  },
});

// Start the network node
async function startNode() {
  try {
    // Initialize IPFS
    const ipfs = await initIpfs();

    // Initialize OrbitDB with the IPFS instance
    const orbitdb = await initOrbitDB({ getHelia: () => ipfs });

    // Create/open a database
    const db = await orbitDB("myDatabase", "feed");

    logger.info("Node started successfully");

    return { ipfs, orbitdb, db };
  } catch (error) {
    logger.error("Failed to start node:", error);
    throw error;
  }
}

startNode();
```

## Configuration

The package provides sensible defaults but can be customized:

```typescript
import { initConfig } from "@debros/network";

const customConfig = initConfig({
  env: {
    fingerprint: "unique-fingerprint",
    port: 9000,
  },
  ipfs: {
    blockstorePath: "./custom-blockstore",
    serviceDiscovery: {
      topic: "my-custom-topic",
      heartbeatInterval: 10000,
    },
  },
  orbitdb: {
    directory: "./custom-orbitdb",
  },
});
```

## Documentation

### Core Modules

- **IPFS Service**: Setup and manage IPFS nodes
- **OrbitDB Service**: Distributed database management
- **Config**: Network configuration
- **Logger**: Consistent logging

### Main Exports

- `initConfig`: Configure the network node
- `ipfsService`: IPFS node management
- `orbitDBService`: OrbitDB operations
- `logger`: Logging utilities
