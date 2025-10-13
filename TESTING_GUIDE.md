# Dynamic Database Clustering - Testing Guide

This guide provides a comprehensive list of unit tests, integration tests, and manual tests needed to verify the dynamic database clustering feature.

## Unit Tests

### 1. Metadata Store Tests (`pkg/rqlite/metadata_test.go`)

```go
// Test cases to implement:

func TestMetadataStore_GetSetDatabase(t *testing.T)
  - Create store
  - Set database metadata
  - Get database metadata
  - Verify data matches

func TestMetadataStore_DeleteDatabase(t *testing.T)
  - Set database metadata
  - Delete database
  - Verify Get returns nil

func TestMetadataStore_ListDatabases(t *testing.T)
  - Add multiple databases
  - List all databases
  - Verify count and contents

func TestMetadataStore_ConcurrentAccess(t *testing.T)
  - Spawn multiple goroutines
  - Concurrent reads and writes
  - Verify no race conditions (run with -race)

func TestMetadataStore_NodeCapacity(t *testing.T)
  - Set node capacity
  - Get node capacity
  - Update capacity
  - List nodes
```

### 2. Vector Clock Tests (`pkg/rqlite/vector_clock_test.go`)

```go
func TestVectorClock_Increment(t *testing.T)
  - Create empty vector clock
  - Increment for node A
  - Verify counter is 1
  - Increment again
  - Verify counter is 2

func TestVectorClock_Merge(t *testing.T)
  - Create two vector clocks with different nodes
  - Merge them
  - Verify max values are preserved

func TestVectorClock_Compare(t *testing.T)
  - Test strictly less than case
  - Test strictly greater than case
  - Test concurrent case
  - Test identical case

func TestVectorClock_Concurrent(t *testing.T)
  - Create clocks with overlapping updates
  - Verify Compare returns 0 (concurrent)
```

### 3. Consensus Tests (`pkg/rqlite/consensus_test.go`)

```go
func TestElectCoordinator_SingleNode(t *testing.T)
  - Pass single node ID
  - Verify it's elected

func TestElectCoordinator_MultipleNodes(t *testing.T)
  - Pass multiple node IDs
  - Verify lowest lexicographical ID wins
  - Verify deterministic (same input = same output)

func TestElectCoordinator_EmptyList(t *testing.T)
  - Pass empty list
  - Verify error returned

func TestElectCoordinator_Deterministic(t *testing.T)
  - Run election multiple times with same inputs
  - Verify same coordinator each time
```

### 4. Port Manager Tests (`pkg/rqlite/ports_test.go`)

```go
func TestPortManager_AllocatePortPair(t *testing.T)
  - Create manager with port range
  - Allocate port pair
  - Verify HTTP and Raft ports different
  - Verify ports within range

func TestPortManager_ReleasePortPair(t *testing.T)
  - Allocate port pair
  - Release ports
  - Verify ports can be reallocated

func TestPortManager_Exhaustion(t *testing.T)
  - Allocate all available ports
  - Attempt one more allocation
  - Verify error returned

func TestPortManager_IsPortAllocated(t *testing.T)
  - Allocate ports
  - Check IsPortAllocated returns true
  - Release ports
  - Check IsPortAllocated returns false

func TestPortManager_AllocateSpecificPorts(t *testing.T)
  - Allocate specific ports
  - Verify allocation succeeds
  - Attempt to allocate same ports again
  - Verify error returned
```

### 5. RQLite Instance Tests (`pkg/rqlite/instance_test.go`)

```go
func TestRQLiteInstance_Create(t *testing.T)
  - Create instance configuration
  - Verify fields set correctly

func TestRQLiteInstance_IsIdle(t *testing.T)
  - Set LastQuery to old timestamp
  - Verify IsIdle returns true
  - Update LastQuery
  - Verify IsIdle returns false

// Integration test (requires rqlite binary):
func TestRQLiteInstance_StartStop(t *testing.T)
  - Create instance
  - Start instance
  - Verify HTTP endpoint responsive
  - Stop instance
  - Verify process terminated
```

### 6. Pubsub Message Tests (`pkg/rqlite/pubsub_messages_test.go`)

```go
func TestMarshalUnmarshalMetadataMessage(t *testing.T)
  - Create each message type
  - Marshal to bytes
  - Unmarshal back
  - Verify data preserved

func TestDatabaseCreateRequest_Marshal(t *testing.T)
func TestDatabaseCreateResponse_Marshal(t *testing.T)
func TestDatabaseCreateConfirm_Marshal(t *testing.T)
func TestDatabaseStatusUpdate_Marshal(t *testing.T)
// ... for all message types
```

### 7. Coordinator Tests (`pkg/rqlite/coordinator_test.go`)

```go
func TestCreateCoordinator_AddResponse(t *testing.T)
  - Create coordinator
  - Add responses
  - Verify response count

func TestCreateCoordinator_SelectNodes(t *testing.T)
  - Add more responses than needed
  - Call SelectNodes
  - Verify correct number selected
  - Verify deterministic selection

func TestCreateCoordinator_WaitForResponses(t *testing.T)
  - Create coordinator
  - Wait in goroutine
  - Add responses from another goroutine
  - Verify wait completes when enough responses

func TestCoordinatorRegistry(t *testing.T)
  - Register coordinator
  - Get coordinator
  - Remove coordinator
  - Verify lifecycle
```

## Integration Tests

### 1. Single Node Database Creation (`e2e/single_node_database_test.go`)

```go
func TestSingleNodeDatabaseCreation(t *testing.T)
  - Start 1 node
  - Set replication_factor = 1
  - Create database
  - Verify database active
  - Write data
  - Read data back
  - Verify data matches
```

### 2. Three Node Database Creation (`e2e/three_node_database_test.go`)

```go
func TestThreeNodeDatabaseCreation(t *testing.T)
  - Start 3 nodes
  - Set replication_factor = 3
  - Create database from node 1
  - Wait for all nodes to report active
  - Write data to node 1
  - Read from node 2
  - Verify replication worked
```

### 3. Multiple Databases (`e2e/multiple_databases_test.go`)

```go
func TestMultipleDatabases(t *testing.T)
  - Start 3 nodes
  - Create database "users"
  - Create database "products"
  - Create database "orders"
  - Verify all databases active
  - Write to each database
  - Verify data isolation
```

### 4. Hibernation Cycle (`e2e/hibernation_test.go`)

```go
func TestHibernationCycle(t *testing.T)
  - Start 3 nodes with hibernation_timeout=5s
  - Create database
  - Write initial data
  - Wait 10 seconds (no activity)
  - Verify status = hibernating
  - Verify processes stopped
  - Verify data persisted on disk

func TestWakeUpCycle(t *testing.T)
  - Create and hibernate database
  - Issue query
  - Wait for wake-up
  - Verify status = active
  - Verify data still accessible
  - Verify LastQuery updated
```

### 5. Node Failure and Recovery (`e2e/failure_recovery_test.go`)

```go
func TestNodeFailureDetection(t *testing.T)
  - Start 3 nodes
  - Create database
  - Kill one node (SIGKILL)
  - Wait for health checks to detect failure
  - Verify NODE_REPLACEMENT_NEEDED broadcast

func TestNodeReplacement(t *testing.T)
  - Start 4 nodes
  - Create database on nodes 1,2,3
  - Kill node 3
  - Wait for replacement
  - Verify node 4 joins cluster
  - Verify data accessible from node 4
```

### 6. Orphaned Data Cleanup (`e2e/cleanup_test.go`)

```go
func TestOrphanedDataCleanup(t *testing.T)
  - Start node
  - Manually create orphaned data directory
  - Restart node
  - Verify orphaned directory removed
  - Check logs for reconciliation message
```

### 7. Concurrent Operations (`e2e/concurrent_test.go`)

```go
func TestConcurrentDatabaseCreation(t *testing.T)
  - Start 5 nodes
  - Create 10 databases concurrently
  - Verify all successful
  - Verify no port conflicts
  - Verify proper distribution

func TestConcurrentHibernation(t *testing.T)
  - Create multiple databases
  - Let all go idle
  - Verify all hibernate correctly
  - No race conditions
```

## Manual Test Scenarios

### Test 1: Basic Flow - Three Node Cluster

**Setup:**
```bash
# Terminal 1: Bootstrap node
cd data/bootstrap
../../bin/node --data bootstrap --id bootstrap --p2p-port 4001

# Terminal 2: Node 2
cd data/node
../../bin/node --data node --id node2 --p2p-port 4002

# Terminal 3: Node 3
cd data/node2
../../bin/node --data node2 --id node3 --p2p-port 4003
```

**Test Steps:**
1. **Create Database**
   ```bash
   # Use client or API to create database "testdb"
   ```
   
2. **Verify Creation**
   - Check logs on all 3 nodes for "Database instance started"
   - Verify `./data/*/testdb/` directories exist on all nodes
   - Check different ports allocated on each node

3. **Write Data**
   ```sql
   CREATE TABLE users (id INT, name TEXT);
   INSERT INTO users VALUES (1, 'Alice');
   INSERT INTO users VALUES (2, 'Bob');
   ```

4. **Verify Replication**
   - Query from each node
   - Verify same data returned

**Expected Results:**
- All nodes show `status=active` for testdb
- Data replicated across all nodes
- Unique port pairs per node

---

### Test 2: Hibernation and Wake-Up

**Setup:** Same as Test 1 with database created

**Test Steps:**
1. **Check Activity**
   ```bash
   # In logs, verify "last_query" timestamps updating on queries
   ```

2. **Wait for Hibernation**
   - Stop issuing queries
   - Wait `hibernation_timeout` + 10s
   - Check logs for "Database is idle"
   - Verify "Coordinated shutdown message sent"
   - Verify "Database hibernated successfully"

3. **Verify Hibernation**
   ```bash
   # Check that rqlite processes are stopped
   ps aux | grep rqlite
   
   # Verify data directories still exist
   ls -la data/*/testdb/
   ```

4. **Wake Up**
   - Issue a query to the database
   - Watch logs for "Received wakeup request"
   - Verify "Database woke up successfully"
   - Verify query succeeds

**Expected Results:**
- Hibernation happens after idle timeout
- All 3 nodes hibernate coordinated
- Wake-up completes in < 8 seconds
- Data persists across hibernation cycle

---

### Test 3: Multiple Databases

**Setup:** 3 nodes running

**Test Steps:**
1. **Create Multiple Databases**
   ```
   Create: users_db
   Create: products_db
   Create: orders_db
   ```

2. **Verify Isolation**
   - Insert data in users_db
   - Verify data NOT in products_db
   - Verify data NOT in orders_db

3. **Check Port Allocation**
   ```bash
   # Verify different ports for each database
   netstat -tlnp | grep rqlite
   # OR
   ss -tlnp | grep rqlite
   ```

4. **Verify Data Directories**
   ```bash
   tree data/bootstrap/
   # Should show:
   # ├── users_db/
   # ├── products_db/
   # └── orders_db/
   ```

**Expected Results:**
- 3 separate database clusters
- Each with 3 nodes (9 total instances)
- Complete data isolation
- Unique port pairs for each instance

---

### Test 4: Node Failure and Recovery

**Setup:** 4 nodes running, database created on nodes 1-3

**Test Steps:**
1. **Verify Initial State**
   - Database active on nodes 1, 2, 3
   - Node 4 idle

2. **Simulate Failure**
   ```bash
   # Kill node 3 (SIGKILL for unclean shutdown)
   kill -9 <node3_pid>
   ```

3. **Watch for Detection**
   - Check logs on nodes 1 and 2
   - Wait for health check failures (3 missed pings)
   - Verify "Node detected as unhealthy" messages

4. **Watch for Replacement**
   - Check for "NODE_REPLACEMENT_NEEDED" broadcast
   - Node 4 should offer to replace
   - Verify "Starting as replacement node" on node 4
   - Verify node 4 joins Raft cluster

5. **Verify Data Integrity**
   - Query database from node 4
   - Verify all data present
   - Insert new data from node 4
   - Verify replication to nodes 1 and 2

**Expected Results:**
- Failure detected within 30 seconds
- Replacement completes automatically
- Data accessible from new node
- No data loss

---

### Test 5: Port Exhaustion

**Setup:** 1 node with small port range

**Configuration:**
```yaml
database:
  max_databases: 10
  port_range_http_start: 5001
  port_range_http_end: 5005  # Only 5 ports
  port_range_raft_start: 7001
  port_range_raft_end: 7005  # Only 5 ports
```

**Test Steps:**
1. **Create Databases**
   - Create database 1 (succeeds - uses 2 ports)
   - Create database 2 (succeeds - uses 2 ports)
   - Create database 3 (fails - only 1 port left)

2. **Verify Error**
   - Check logs for "Cannot allocate ports"
   - Verify error returned to client

3. **Free Ports**
   - Hibernate or delete database 1
   - Ports should be freed

4. **Retry**
   - Create database 3 again
   - Should succeed now

**Expected Results:**
- Graceful handling of port exhaustion
- Clear error messages
- Ports properly recycled

---

### Test 6: Orphaned Data Cleanup

**Setup:** 1 node stopped

**Test Steps:**
1. **Create Orphaned Data**
   ```bash
   # While node is stopped
   mkdir -p data/bootstrap/orphaned_db/rqlite
   echo "fake data" > data/bootstrap/orphaned_db/rqlite/db.sqlite
   ```

2. **Start Node**
   ```bash
   ./bin/node --data bootstrap --id bootstrap
   ```

3. **Check Reconciliation**
   - Watch logs for "Starting orphaned data reconciliation"
   - Verify "Found orphaned database directory"
   - Verify "Removed orphaned database directory"

4. **Verify Cleanup**
   ```bash
   ls data/bootstrap/
   # orphaned_db should be gone
   ```

**Expected Results:**
- Orphaned directories automatically detected
- Removed on startup
- Clean reconciliation logged

---

### Test 7: Stress Test - Many Databases

**Setup:** 5 nodes with high capacity

**Configuration:**
```yaml
database:
  max_databases: 50
  port_range_http_start: 5001
  port_range_http_end: 5150
  port_range_raft_start: 7001
  port_range_raft_end: 7150
```

**Test Steps:**
1. **Create Many Databases**
   ```
   Loop: Create databases db_1 through db_25
   ```

2. **Verify Distribution**
   - Check logs for node capacity announcements
   - Verify databases distributed across nodes
   - No single node overloaded

3. **Concurrent Operations**
   - Write to multiple databases simultaneously
   - Read from multiple databases
   - Verify no conflicts

4. **Hibernation Wave**
   - Stop all activity
   - Wait for hibernation
   - Verify all databases hibernate
   - Check resource usage drops

5. **Wake-Up Storm**
   - Query all 25 databases at once
   - Verify all wake up successfully
   - Check for thundering herd issues

**Expected Results:**
- All 25 databases created successfully
- Even distribution across nodes
- No port conflicts
- Successful mass hibernation/wake-up

---

### Test 8: Gateway API Access

**Setup:** Gateway running with 3 nodes

**Test Steps:**
1. **Authenticate**
   ```bash
   # Get JWT token
   TOKEN=$(curl -X POST http://localhost:8080/v1/auth/login \
     -H "Content-Type: application/json" \
     -d '{"wallet": "..."}' | jq -r .token)
   ```

2. **Create Table**
   ```bash
   curl -X POST http://localhost:8080/v1/database/create-table \
     -H "Authorization: Bearer $TOKEN" \
     -H "Content-Type: application/json" \
     -d '{
       "database": "testdb",
       "schema": "CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT, email TEXT)"
     }'
   ```

3. **Insert Data**
   ```bash
   curl -X POST http://localhost:8080/v1/database/exec \
     -H "Authorization: Bearer $TOKEN" \
     -H "Content-Type: application/json" \
     -d '{
       "database": "testdb",
       "sql": "INSERT INTO users (name, email) VALUES (?, ?)",
       "args": ["Alice", "alice@example.com"]
     }'
   ```

4. **Query Data**
   ```bash
   curl -X POST http://localhost:8080/v1/database/query \
     -H "Authorization: Bearer $TOKEN" \
     -H "Content-Type: application/json" \
     -d '{
       "database": "testdb",
       "sql": "SELECT * FROM users"
     }'
   ```

5. **Test Transaction**
   ```bash
   curl -X POST http://localhost:8080/v1/database/transaction \
     -H "Authorization: Bearer $TOKEN" \
     -H "Content-Type: application/json" \
     -d '{
       "database": "testdb",
       "queries": [
         "INSERT INTO users (name, email) VALUES (\"Bob\", \"bob@example.com\")",
         "INSERT INTO users (name, email) VALUES (\"Charlie\", \"charlie@example.com\")"
       ]
     }'
   ```

6. **Get Schema**
   ```bash
   curl -X GET "http://localhost:8080/v1/database/schema?database=testdb" \
     -H "Authorization: Bearer $TOKEN"
   ```

7. **Test Hibernation**
   - Wait for hibernation timeout
   - Query again and measure wake-up time
   - Should see delay on first query after hibernation

**Expected Results:**
- All API calls succeed
- Data persists across calls
- Transactions are atomic
- Schema reflects created tables
- Hibernation/wake-up transparent to API
- Response times reasonable (< 30s for queries)

---

## Test Checklist

### Unit Tests (To Implement)
- [ ] Metadata Store operations
- [ ] Metadata Store concurrency
- [ ] Vector Clock increment
- [ ] Vector Clock merge
- [ ] Vector Clock compare
- [ ] Coordinator election (single node)
- [ ] Coordinator election (multiple nodes)
- [ ] Coordinator election (deterministic)
- [ ] Port Manager allocation
- [ ] Port Manager release
- [ ] Port Manager exhaustion
- [ ] Port Manager specific ports
- [ ] RQLite Instance creation
- [ ] RQLite Instance IsIdle
- [ ] Message marshal/unmarshal (all types)
- [ ] Coordinator response collection
- [ ] Coordinator node selection
- [ ] Coordinator registry

### Integration Tests (To Implement)
- [ ] Single node database creation
- [ ] Three node database creation
- [ ] Multiple databases isolation
- [ ] Hibernation cycle
- [ ] Wake-up cycle
- [ ] Node failure detection
- [ ] Node replacement
- [ ] Orphaned data cleanup
- [ ] Concurrent database creation
- [ ] Concurrent hibernation

### Manual Tests (To Perform)
- [ ] Basic three node flow
- [ ] Hibernation and wake-up
- [ ] Multiple databases
- [ ] Node failure and recovery
- [ ] Port exhaustion handling
- [ ] Orphaned data cleanup
- [ ] Stress test with many databases

### Performance Validation
- [ ] Database creation < 10s
- [ ] Wake-up time < 8s
- [ ] Metadata sync < 5s
- [ ] Query overhead < 10ms additional

## Running Tests

### Unit Tests
```bash
# Run all tests
go test ./pkg/rqlite/... -v

# Run with race detector
go test ./pkg/rqlite/... -race

# Run specific test
go test ./pkg/rqlite/ -run TestMetadataStore_GetSetDatabase -v

# Run with coverage
go test ./pkg/rqlite/... -cover -coverprofile=coverage.out
go tool cover -html=coverage.out
```

### Integration Tests
```bash
# Run e2e tests
go test ./e2e/... -v -timeout 30m

# Run specific e2e test
go test ./e2e/ -run TestThreeNodeDatabaseCreation -v
```

### Manual Tests
Follow the scenarios above in dedicated terminals for each node.

## Success Criteria

### Correctness
✅ All unit tests pass  
✅ All integration tests pass  
✅ All manual scenarios complete successfully  
✅ No data loss in any scenario  
✅ No race conditions detected  

### Performance
✅ Database creation < 10 seconds  
✅ Wake-up < 8 seconds  
✅ Metadata sync < 5 seconds  
✅ Query overhead < 10ms  

### Reliability
✅ Survives node failures  
✅ Automatic recovery works  
✅ No orphaned data accumulates  
✅ Hibernation/wake-up cycles stable  
✅ Concurrent operations safe  

## Notes for Future Test Enhancements

When implementing advanced metrics and benchmarks:

1. **Prometheus Metrics Tests**
   - Verify metric export
   - Validate metric values
   - Test metric reset on restart

2. **Benchmark Suite**
   - Automated performance regression detection
   - Latency percentile tracking (p50, p95, p99)
   - Throughput measurements
   - Resource usage profiling

3. **Chaos Engineering**
   - Random node kills
   - Network partitions
   - Clock skew simulation
   - Disk full scenarios

4. **Long-Running Stability**
   - 24-hour soak test
   - Memory leak detection
   - Slow-growing resource usage

## Debugging Failed Tests

### Common Issues

**Port Conflicts**
```bash
# Check for processes using test ports
lsof -i :5001-5999
lsof -i :7001-7999

# Kill stale processes
pkill rqlited
```

**Stale Data**
```bash
# Clean test data directories
rm -rf data/test_*/
rm -rf /tmp/debros_test_*/
```

**Timing Issues**
- Increase timeouts in flaky tests
- Add retry logic with exponential backoff
- Use proper synchronization primitives

**Race Conditions**
```bash
# Always run with race detector during development
go test -race ./...
```


