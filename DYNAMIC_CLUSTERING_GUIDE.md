# Dynamic Database Clustering - User Guide

## Overview

Dynamic Database Clustering enables on-demand creation of isolated, replicated rqlite database clusters with automatic resource management through hibernation. Each database runs as a separate 3-node cluster with its own data directory and port allocation.

## Key Features

✅ **Multi-Database Support** - Create unlimited isolated databases on-demand  
✅ **3-Node Replication** - Fault-tolerant by default (configurable)  
✅ **Auto Hibernation** - Idle databases hibernate to save resources  
✅ **Transparent Wake-Up** - Automatic restart on access  
✅ **App Namespacing** - Databases are scoped by application name  
✅ **Decentralized Metadata** - LibP2P pubsub-based coordination  
✅ **Failure Recovery** - Automatic node replacement on failures  
✅ **Resource Optimization** - Dynamic port allocation and data isolation  

## Configuration

### Node Configuration (`configs/node.yaml`)

```yaml
node:
  data_dir: "./data"
  listen_addresses:
    - "/ip4/0.0.0.0/tcp/4001"
  max_connections: 50

database:
  replication_factor: 3           # Number of replicas per database
  hibernation_timeout: 60s        # Idle time before hibernation
  max_databases: 100              # Max databases per node
  port_range_http_start: 5001     # HTTP port range start
  port_range_http_end: 5999       # HTTP port range end
  port_range_raft_start: 7001     # Raft port range start
  port_range_raft_end: 7999       # Raft port range end

discovery:
  bootstrap_peers:
    - "/ip4/127.0.0.1/tcp/4001/p2p/..."
  discovery_interval: 30s
  health_check_interval: 10s
```

### Key Configuration Options

#### `database.replication_factor` (default: 3)
Number of nodes that will host each database cluster. Minimum 1, recommended 3 for fault tolerance.

#### `database.hibernation_timeout` (default: 60s)
Time of inactivity before a database is hibernated. Set to 0 to disable hibernation.

#### `database.max_databases` (default: 100)
Maximum number of databases this node can host simultaneously.

#### `database.port_range_*`
Port ranges for dynamic allocation. Ensure ranges are large enough for `max_databases * 2` ports (HTTP + Raft per database).

## Client Usage

### Creating/Accessing Databases

```go
package main

import (
    "context"
    "github.com/DeBrosOfficial/network/pkg/client"
)

func main() {
    // Create client with app name for namespacing
    cfg := client.DefaultClientConfig("myapp")
    cfg.BootstrapPeers = []string{
        "/ip4/127.0.0.1/tcp/4001/p2p/...",
    }
    
    c, err := client.NewClient(cfg)
    if err != nil {
        panic(err)
    }
    
    // Connect to network
    if err := c.Connect(); err != nil {
        panic(err)
    }
    defer c.Disconnect()
    
    // Get database client (creates database if it doesn't exist)
    db, err := c.Database().Database("users")
    if err != nil {
        panic(err)
    }
    
    // Use the database
    ctx := context.Background()
    err = db.CreateTable(ctx, `
        CREATE TABLE users (
            id INTEGER PRIMARY KEY,
            name TEXT NOT NULL,
            email TEXT UNIQUE
        )
    `)
    
    // Query data
    result, err := db.Query(ctx, "SELECT * FROM users")
    // ...
}
```

### Database Naming

Databases are automatically namespaced by your application name:
- `client.Database("users")` → creates `myapp_users` internally
- This prevents name collisions between different applications

## Gateway API Usage

If you prefer HTTP/REST API access instead of the Go client, you can use the gateway endpoints:

### Base URL
```
http://gateway-host:8080/v1/database/
```

### Execute SQL (INSERT, UPDATE, DELETE, DDL)
```bash
POST /v1/database/exec
Content-Type: application/json

{
  "database": "users",
  "sql": "INSERT INTO users (name, email) VALUES (?, ?)",
  "args": ["Alice", "alice@example.com"]
}

Response:
{
  "rows_affected": 1,
  "last_insert_id": 1
}
```

### Query Data (SELECT)
```bash
POST /v1/database/query
Content-Type: application/json

{
  "database": "users",
  "sql": "SELECT * FROM users WHERE name LIKE ?",
  "args": ["A%"]
}

Response:
{
  "items": [
    {"id": 1, "name": "Alice", "email": "alice@example.com"}
  ],
  "count": 1
}
```

### Execute Transaction
```bash
POST /v1/database/transaction
Content-Type: application/json

{
  "database": "users",
  "queries": [
    "INSERT INTO users (name, email) VALUES ('Bob', 'bob@example.com')",
    "UPDATE users SET email = 'alice.new@example.com' WHERE name = 'Alice'"
  ]
}

Response:
{
  "success": true
}
```

### Get Schema
```bash
GET /v1/database/schema?database=users

# OR

POST /v1/database/schema
Content-Type: application/json

{
  "database": "users"
}

Response:
{
  "tables": [
    {
      "name": "users",
      "columns": ["id", "name", "email"]
    }
  ]
}
```

### Create Table
```bash
POST /v1/database/create-table
Content-Type: application/json

{
  "database": "users",
  "schema": "CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT, email TEXT)"
}

Response:
{
  "rows_affected": 0
}
```

### Drop Table
```bash
POST /v1/database/drop-table
Content-Type: application/json

{
  "database": "users",
  "table_name": "old_table"
}

Response:
{
  "rows_affected": 0
}
```

### List Databases
```bash
GET /v1/database/list

Response:
{
  "databases": ["users", "products", "orders"]
}
```

### Important Notes

1. **Authentication Required**: All endpoints require authentication (JWT or API key)
2. **Database Creation**: Databases are created automatically on first access
3. **Hibernation**: The gateway handles hibernation/wake-up transparently - you may experience a delay (< 8s) on first query to a hibernating database
4. **Timeouts**: Query timeout is 30s, transaction timeout is 60s
5. **Namespacing**: Database names are automatically prefixed with your app name
6. **Concurrent Access**: All endpoints are safe for concurrent use

## Database Lifecycle

### 1. Creation

When you first access a database:

1. **Request Broadcast** - Node broadcasts `DATABASE_CREATE_REQUEST`
2. **Node Selection** - Eligible nodes respond with available ports
3. **Coordinator Selection** - Deterministic coordinator (lowest peer ID) chosen
4. **Confirmation** - Coordinator selects nodes and broadcasts `DATABASE_CREATE_CONFIRM`
5. **Instance Startup** - Selected nodes start rqlite subprocesses
6. **Readiness** - Nodes report `active` status when ready

**Typical creation time: < 10 seconds**

### 2. Active State

- Database instances run as rqlite subprocesses
- Each instance tracks `LastQuery` timestamp
- Queries update the activity timestamp
- Metadata synced across all network nodes

### 3. Hibernation

After `hibernation_timeout` of inactivity:

1. **Idle Detection** - Nodes detect idle databases
2. **Idle Notification** - Nodes broadcast idle status
3. **Coordinated Shutdown** - When all nodes report idle, coordinator schedules shutdown
4. **Graceful Stop** - SIGTERM sent to rqlite processes
5. **Port Release** - Ports freed for reuse
6. **Status Update** - Metadata updated to `hibernating`

**Data persists on disk during hibernation**

### 4. Wake-Up

On first query to hibernating database:

1. **Detection** - Client/node detects `hibernating` status
2. **Wake Request** - Broadcast `DATABASE_WAKEUP_REQUEST`
3. **Port Allocation** - Reuse original ports or allocate new ones
4. **Instance Restart** - Restart rqlite with existing data
5. **Status Update** - Update to `active` when ready

**Typical wake-up time: < 8 seconds**

### 5. Failure Recovery

When a node fails:

1. **Health Detection** - Missed health checks trigger failure detection
2. **Replacement Request** - Surviving nodes broadcast `NODE_REPLACEMENT_NEEDED`
3. **Offers** - Healthy nodes with capacity offer to replace
4. **Selection** - First offer accepted (simple approach)
5. **Join Cluster** - New node joins existing Raft cluster
6. **Sync** - Data synced from existing members

## Data Management

### Data Directories

Each database gets its own data directory:
```
./data/
  ├── myapp_users/        # Database: users
  │   └── rqlite/
  │       ├── db.sqlite
  │       └── raft/
  ├── myapp_products/     # Database: products
  │   └── rqlite/
  └── myapp_orders/       # Database: orders
      └── rqlite/
```

### Orphaned Data Cleanup

On node startup, the system automatically:
- Scans data directories
- Checks against metadata
- Removes directories for:
  - Non-existent databases
  - Databases where this node is not a member

## Monitoring & Debugging

### Structured Logging

All operations are logged with structured fields:

```
INFO  Starting cluster manager node_id=12D3... max_databases=100
INFO  Received database create request database=myapp_users requester=12D3...
INFO  Database instance started database=myapp_users http_port=5001 raft_port=7001
INFO  Database is idle database=myapp_users idle_time=62s
INFO  Database hibernated successfully database=myapp_users
INFO  Received wakeup request database=myapp_users
INFO  Database woke up successfully database=myapp_users
```

### Health Checks

Nodes perform periodic health checks:
- Every `health_check_interval` (default: 10s)
- Tracks last-seen time for each peer
- 3 missed checks → node marked unhealthy
- Triggers replacement protocol for affected databases

## Best Practices

### 1. **Capacity Planning**

```yaml
# For 100 databases with 3-node replication:
database:
  max_databases: 100
  port_range_http_start: 5001
  port_range_http_end: 5200    # 200 ports (100 databases * 2)
  port_range_raft_start: 7001
  port_range_raft_end: 7200
```

### 2. **Hibernation Tuning**

- **High Traffic**: Set `hibernation_timeout: 300s` or higher
- **Development**: Set `hibernation_timeout: 30s` for faster cycles
- **Always-On DBs**: Set `hibernation_timeout: 0` to disable

### 3. **Replication Factor**

- **Development**: `replication_factor: 1` (single node, no replication)
- **Production**: `replication_factor: 3` (fault tolerant)
- **High Availability**: `replication_factor: 5` (survives 2 failures)

### 4. **Network Topology**

- Use at least 3 nodes for `replication_factor: 3`
- Ensure `max_databases * replication_factor <= total_cluster_capacity`
- Example: 3 nodes × 100 max_databases = 300 database instances total

## Troubleshooting

### Database Creation Fails

**Problem**: `insufficient nodes responded: got 1, need 3`

**Solution**:
- Ensure you have at least `replication_factor` nodes online
- Check `max_databases` limit on nodes
- Verify port ranges aren't exhausted

### Database Not Waking Up

**Problem**: Database stays in `waking` status

**Solution**:
- Check node logs for rqlite startup errors
- Verify rqlite binary is installed
- Check port conflicts (use different port ranges)
- Ensure data directory is accessible

### Orphaned Data

**Problem**: Disk space consumed by old databases

**Solution**:
- Orphaned data is automatically cleaned on node restart
- Manual cleanup: Delete directories from `./data/` that don't match metadata
- Check logs for reconciliation results

### Node Replacement Not Working

**Problem**: Failed node not replaced

**Solution**:
- Ensure remaining nodes have capacity (`CurrentDatabases < MaxDatabases`)
- Check network connectivity between nodes
- Verify health check interval is reasonable (not too aggressive)

## Advanced Topics

### Metadata Consistency

- **Vector Clocks**: Each metadata update includes vector clock for conflict resolution
- **Gossip Protocol**: Periodic metadata sync via checksums
- **Eventual Consistency**: All nodes eventually agree on database state

### Port Management

- Ports allocated randomly within configured ranges
- Bind-probing ensures ports are actually available
- Ports reused during wake-up when possible
- Failed allocations fall back to new random ports

### Coordinator Election

- Deterministic selection based on lexicographical peer ID ordering
- Lowest peer ID becomes coordinator
- No persistent coordinator state
- Re-election occurs for each database operation

## Migration from Legacy Mode

If upgrading from single-cluster rqlite:

1. **Backup Data**: Backup your existing `./data/rqlite` directory
2. **Update Config**: Remove deprecated fields:
   - `database.data_dir`
   - `database.rqlite_port`
   - `database.rqlite_raft_port`
   - `database.rqlite_join_address`
3. **Add New Fields**: Configure dynamic clustering (see Configuration section)
4. **Restart Nodes**: Restart all nodes with new configuration
5. **Migrate Data**: Create new database and import data from backup

## Future Enhancements

The following features are planned for future releases:

### **Advanced Metrics** (Future)
- Prometheus-style metrics export
- Per-database query counters
- Hibernation/wake-up latency histograms
- Resource utilization gauges

### **Performance Benchmarks** (Future)
- Automated benchmark suite
- Creation time SLOs
- Wake-up latency targets
- Query overhead measurements

### **Enhanced Monitoring** (Future)
- Dashboard for cluster visualization
- Database status API endpoint
- Capacity planning tools
- Alerting integration

## Support

For issues, questions, or contributions:
- GitHub Issues: https://github.com/DeBrosOfficial/network/issues
- Documentation: https://github.com/DeBrosOfficial/network/blob/main/DYNAMIC_DATABASE_CLUSTERING.md

## License

See LICENSE file for details.

