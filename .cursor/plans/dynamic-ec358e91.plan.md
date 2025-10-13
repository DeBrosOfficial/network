<!-- ec358e91-8e19-4fc8-a81e-cb388a4b2fc9 4c357d4a-bae7-4fe2-943d-84e5d3d3714c -->
# Dynamic Database Clustering — Implementation Plan

### Scope

Implement the feature described in `DYNAMIC_DATABASE_CLUSTERING.md`: decentralized metadata via libp2p pubsub, dynamic per-database rqlite clusters (3-node default), idle hibernation/wake-up, node failure replacement, and client UX that exposes `cli.Database(name)` with app namespacing.

### Guiding Principles

- Reuse existing `pkg/pubsub` and `pkg/rqlite` where practical; avoid singletons.
- Backward-compatible config migration with deprecations, feature-flag controlled rollout.
- Strong eventual consistency (vector clocks + periodic gossip) over centralized control planes.
- Tests and observability at each phase.

### Phase 0: Prep & Scaffolding

- Add feature flag `dynamic_db_clustering` (env/config) → default off.
- Introduce config shape for new `database` fields while supporting legacy fields (soft deprecated).
- Create empty packages and interfaces to enable incremental compilation:
  - `pkg/metadata/{types.go,manager.go,pubsub.go,consensus.go,vector_clock.go}`
  - `pkg/dbcluster/{manager.go,lifecycle.go,subprocess.go,ports.go,health.go,metrics.go}`
- Ensure rqlite subprocess availability (binary path detection, `scripts/install-debros-network.sh` update if needed).
- Establish CI jobs for new unit/integration suites and longer-running e2e.

### Phase 1: Metadata Layer (No hibernation yet)

- Implement metadata types and store (RW locks, versioning) inside `pkg/rqlite/metadata.go`:
  - `DatabaseMetadata`, `NodeCapacity`, `PortRange`, `MetadataStore`.
- Pubsub schema and handlers inside `pkg/rqlite/pubsub.go` using existing `pkg/pubsub` bridge:
  - Topic `/debros/metadata/v1`; messages for create request/response/confirm, status, node capacity, health.
- Consensus helpers inside `pkg/rqlite/consensus.go` and `pkg/rqlite/vector_clock.go`:
  - Deterministic coordinator (lowest peer ID), vector clocks, merge rules, periodic full-state gossip (checksums + fetch diffs).
- Reuse existing node connectivity/backoff; no new ping service required.
- Skip unit tests for now; validate by wiring e2e flows later.

### Phase 2: Database Creation & Client API

- Port management:
  - `PortManager` with bind-probing, random allocation within configured ranges; local bookkeeping.
- Subprocess control:
  - `RQLiteInstance` lifecycle (start, wait ready via /status and simple query, stop, status).
- Cluster manager:
  - `ClusterManager` keeps `activeClusters`, listens to metadata events, executes creation protocol, readiness fan-in, failure surfaces.
- Client API:
  - Update `pkg/client/interface.go` to include `Database(name string)`.
  - Implement app namespacing in `pkg/client/client.go` (sanitize app name + db name).
  - Backoff polling for readiness during creation.
- Data isolation:
  - Data dir per db: `./data/<app>_<db>/rqlite` (respect node `data_dir` base).
- Integration tests: create single db across 3 nodes; multiple databases coexisting; cross-node read/write.

### Phase 3: Hibernation & Wake-Up

- Idle detection and coordination:
  - Track `LastQuery` per instance; periodic scan; all-nodes-idle quorum → coordinated shutdown schedule.
- Hibernation protocol:
  - Broadcast idle notices, coordinator schedules `DATABASE_SHUTDOWN_COORDINATED`, graceful SIGTERM, ports freed, status → `hibernating`.
- Wake-up protocol:
  - Client detects `hibernating`, performs CAS to `waking`, triggers wake request; port reuse if available else re-negotiate; start instances; status → `active`.
- Client retry UX:
  - Transparent retries with exponential backoff; treat `waking` as wait-only state.
- Tests: hibernation under load; thundering herd; resource verification and persistence across cycles.

### Phase 4: Resilience (Failure & Replacement)

- Continuous health checks with timeouts → mark node unhealthy.
- Replacement orchestration:
  - Coordinator initiates `NODE_REPLACEMENT_NEEDED`, eligible nodes respond, confirm selection, new node joins raft via `-join` then syncs.
- Startup reconciliation:
  - Detect and cleanup orphaned or non-member local data directories.
- Rate limiting replacements to prevent cascades; prioritize by usage metrics.
- Tests: forced crashes, partitions, replacement within target SLO; reconciliation sanity.

### Phase 5: Production Hardening & Optimization

- Metrics/logging:
  - Structured logs with trace IDs; counters for queries/min, hibernations, wake-ups, replacements; health and capacity gauges.
- Config validation, replication factor settings (1,3,5), and debugging APIs (read-only metadata dump, node status).
- Client metadata caching and query routing improvements (simple round-robin → latency-aware later).
- Performance benchmarks and operator-facing docs.

### File Changes (Essentials)

- `pkg/config/config.go`
  - Remove (deprecate, then delete): `Database.DataDir`, `RQLitePort`, `RQLiteRaftPort`, `RQLiteJoinAddress`.
  - Add: `ReplicationFactor int`, `HibernationTimeout time.Duration`, `MaxDatabases int`, `PortRange {HTTPStart, HTTPEnd, RaftStart, RaftEnd int}`, `Discovery.HealthCheckInterval`.
- `pkg/client/interface.go`/`pkg/client/client.go`
  - Add `Database(name string)` and app namespace requirement (`DefaultClientConfig(appName)`); backoff polling.
- `pkg/node/node.go`
  - Wire `metadata.Manager` and `dbcluster.ClusterManager`; remove direct rqlite singleton usage.
- `pkg/rqlite/*`
  - Refactor to instance-oriented helpers from singleton.
- New packages under `pkg/metadata` and `pkg/dbcluster` as listed above.
- `configs/node.yaml` and validation paths to reflect new `database` block.

### Config Example (target end-state)

```yaml
node:
  data_dir: "./data"

database:
  replication_factor: 3
  hibernation_timeout: 60
  max_databases: 100
  port_range:
    http_start: 5001
    http_end: 5999
    raft_start: 7001
    raft_end: 7999

discovery:
  health_check_interval: 10s
```

### Rollout Strategy

- Keep feature flag off by default; support legacy single-cluster path.
- Ship Phase 1 behind flag; enable in dev/e2e only.
- Incrementally enable creation (Phase 2), then hibernation (Phase 3) per environment.
- Remove legacy config after deprecation window.

### Testing & Quality Gates

- Unit tests: metadata ops, consensus, ports, subprocess, manager state machine.
- Integration tests under `e2e/` for creation, isolation, hibernation, failure handling, partitions.
- Benchmarks for creation (<10s), wake-up (<8s), metadata sync (<5s), query overhead (<10ms).
- Chaos suite for randomized failures and partitions.

### Risks & Mitigations (operationalized)

- Metadata divergence → vector clocks + periodic checksums + majority read checks in client.
- Raft churn → adaptive timeouts; allow `always_on` flag per-db (future).
- Cascading replacements → global rate limiter and prioritization.
- Debuggability → verbose structured logging and metadata dump endpoints.

### Timeline (indicative)

- Weeks 1-2: Phases 0-1
- Weeks 3-4: Phase 2
- Weeks 5-6: Phase 3
- Weeks 7-8: Phase 4
- Weeks 9-10+: Phase 5

### To-dos

- [ ] Add feature flag, scaffold packages, CI jobs, rqlite binary checks
- [ ] Extend `pkg/config/config.go` and YAML schemas; deprecate legacy fields
- [ ] Implement metadata types and thread-safe store with versioning
- [ ] Implement pubsub messages and handlers using existing pubsub manager
- [ ] Implement coordinator election, vector clocks, gossip reconciliation
- [ ] Implement `PortManager` with bind-probing and allocation
- [ ] Implement rqlite subprocess control and readiness checks
- [ ] Implement `ClusterManager` and creation lifecycle orchestration
- [ ] Add `Database(name)` and app namespacing to client; backoff polling
- [ ] Adopt per-database data dirs under node `data_dir`
- [ ] Integration tests for creation and isolation across nodes
- [ ] Idle detection, coordinated shutdown, status updates
- [ ] Wake-up CAS to `waking`, port reuse/renegotiation, restart
- [ ] Client transparent retry/backoff for hibernation and waking
- [ ] Health checks, replacement orchestration, rate limiting
- [ ] Implement orphaned data reconciliation on startup
- [ ] Add metrics and structured logging across managers
- [ ] Benchmarks for creation, wake-up, sync, query overhead
- [ ] Operator and developer docs; config and migration guides