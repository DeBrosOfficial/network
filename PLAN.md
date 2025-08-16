# DeBros Network Gateway Implementation Plan

## Overview
This document outlines the phased implementation plan for the DeBros Network Gateway system, which provides HTTP/gRPC interfaces for non-Go clients to access network features like pub-sub, RQLite database, and storage through Ethereum wallet-based authentication and subscription models.

## Architecture Summary
- Separate `cmd/gateway` binary (not embedded in node)
- HTTP endpoints by default, optional gRPC support  
- WebSocket support for pub-sub subscriptions
- Core RQLite database for gateway operational data
- Ethereum wallet-based authentication
- Multi-tenant namespace isolation
- Subscription-based payment model

---

## Phase 1: Basic Gateway Foundation (Week 1)

### Objective
Create the core gateway structure without authentication - a working HTTP proxy to the network.

### Step 1.1: Gateway Skeleton
**Files to create:**
```
cmd/gateway/main.go
pkg/gateway/config/config.go
pkg/gateway/server/server.go
```

**Implementation:**
- Basic HTTP server setup with graceful shutdown
- Configuration loading (port, network client settings)
- Health check endpoint (`/health`)
- Signal handling for SIGTERM/SIGINT
- Structured logging integration

### Step 1.2: Network Client Integration
**Files to create:**
```
pkg/gateway/client/network.go
pkg/gateway/client/pool.go
```

**Implementation:**
- Initialize network client connection using existing `pkg/client`
- Connection management and pooling
- Basic error handling and retries
- Health monitoring of network connections

### Step 1.3: Basic HTTP Handlers (No Auth)
**Files to create:**
```
pkg/gateway/handlers/health.go
pkg/gateway/handlers/storage.go
pkg/gateway/handlers/network.go
pkg/gateway/middleware/cors.go
pkg/gateway/middleware/logging.go
```

**Implementation:**
- Health check endpoint
- Basic storage GET/PUT (pass-through to network)
- Network status endpoint
- CORS middleware for web clients
- Request/response logging middleware

### Deliverables Phase 1
- [ ] Working gateway that can proxy basic requests to the network
- [ ] `/health` and `/status` endpoints functional
- [ ] Basic storage operations working without auth
- [ ] Proper error handling and logging

---

## Phase 2: Core Database & Models (Week 1-2)

### Objective
Set up the foundation database schema and models for authentication and multi-tenancy.

### Step 2.1: Database Setup
**Files to create:**
```
migrations/001_initial.sql
migrations/002_indexes.sql
pkg/gateway/db/migrations.go
```

**Implementation:**
```sql
-- Core tables
CREATE TABLE apps (id, namespace, wallet_address, created_at, updated_at);
CREATE TABLE namespaces (id, name, owner_wallet, created_at);
CREATE TABLE api_keys (id, app_id, key_hash, created_at, last_used);
CREATE TABLE audit_events (id, namespace, action, resource, timestamp);
CREATE TABLE nonces (wallet_address, nonce, expires_at);
CREATE TABLE refresh_tokens (id, app_id, token_hash, expires_at);
```

### Step 2.2: Database Access Layer
**Files to create:**
```
pkg/gateway/db/connection.go
pkg/gateway/db/models.go
pkg/gateway/db/queries.go
pkg/gateway/db/migrate.go
```

**Implementation:**
- Database connection management
- Model structs for all tables
- CRUD operations for each model
- Migration runner and version tracking

### Step 2.3: Namespace Management
**Files to create:**
```
pkg/gateway/namespace/manager.go
pkg/gateway/namespace/validator.go
pkg/gateway/namespace/errors.go
```

**Implementation:**
- Namespace CRUD operations
- Validation rules (naming, uniqueness)
- Ownership verification
- Namespace reservation system

### Deliverables Phase 2
- [ ] Database schema deployed and versioned
- [ ] Basic CRUD operations for apps and namespaces
- [ ] Migration system working
- [ ] Namespace management API

---

## Phase 3: Ethereum Wallet Authentication (Week 2)

### Objective
Implement the core Ethereum wallet-based authentication system.

### Step 3.1: Wallet Signature Verification
**Files to create:**
```
pkg/gateway/auth/ethereum.go
pkg/gateway/auth/nonce.go
pkg/gateway/auth/signature.go
```

**Implementation:**
- Message signing/verification using secp256k1
- Nonce generation and management (prevent replay attacks)
- Address recovery from signatures
- EIP-191 message formatting

### Step 3.2: JWT Token System
**Files to create:**
```
pkg/gateway/auth/jwt.go
pkg/gateway/auth/claims.go
pkg/gateway/auth/refresh.go
```

**Implementation:**
- JWT token generation with namespace claims
- Token validation middleware
- Refresh token implementation
- Token blacklisting for logout

### Step 3.3: Authentication Endpoints
**Files to create:**
```
pkg/gateway/handlers/auth.go
pkg/gateway/middleware/auth.go
```

**Endpoints:**
- `POST /v1/auth/nonce` - Get signing nonce
- `POST /v1/auth/verify` - Verify signature and get JWT
- `POST /v1/auth/refresh` - Refresh JWT token
- `POST /v1/auth/logout` - Invalidate tokens

### Deliverables Phase 3
- [ ] Working Ethereum wallet authentication
- [ ] JWT tokens with namespace claims
- [ ] Session management with refresh tokens
- [ ] Secure logout functionality

---

## Phase 4: Namespace Isolation & Security (Week 2-3)

### Objective
Implement strict multi-tenant security with complete namespace isolation.

### Step 4.1: Namespace Enforcement Middleware
**Files to create:**
```
pkg/gateway/middleware/namespace.go
pkg/gateway/middleware/ownership.go
pkg/gateway/security/validator.go
```

**Implementation:**
- Extract namespace from JWT claims
- Validate namespace ownership against database
- Inject namespace into request context
- Block cross-namespace access attempts

### Step 4.2: Resource Prefixing
**Files to create:**
```
pkg/gateway/isolation/storage.go
pkg/gateway/isolation/pubsub.go
pkg/gateway/isolation/database.go
pkg/gateway/isolation/keys.go
```

**Implementation:**
- Storage keys: `ns::<namespace>::<key>`
- PubSub topics: `<namespace>.<topic>`
- Database tables: `ns__<namespace>__tablename`
- Consistent prefixing across all resources

### Step 4.3: Secure Handlers Update
**Files to update:**
```
pkg/gateway/handlers/storage.go
pkg/gateway/handlers/pubsub.go
pkg/gateway/handlers/database.go
```

**Implementation:**
- All handlers updated to use namespace isolation
- Resource access validation
- Audit logging for all operations

### Deliverables Phase 4
- [ ] All operations namespace-isolated
- [ ] Cross-namespace access prevented and logged
- [ ] Security tests passing
- [ ] Audit trail for all resource access

---

## Phase 5: Complete API Implementation (Week 3)

### Objective
Implement all remaining REST and WebSocket endpoints.

### Step 5.1: Storage API
**Files to create/update:**
```
pkg/gateway/handlers/storage.go
pkg/gateway/api/storage.go
```

**Endpoints:**
- `GET /v1/storage/:key` - Get value
- `PUT /v1/storage/:key` - Set value
- `DELETE /v1/storage/:key` - Delete key
- `GET /v1/storage` - List keys with prefix filter

### Step 5.2: PubSub API with WebSockets
**Files to create:**
```
pkg/gateway/handlers/pubsub.go
pkg/gateway/websocket/manager.go
pkg/gateway/websocket/subscriber.go
```

**Endpoints:**
- `POST /v1/pubsub/publish` - Publish message
- `WebSocket /v1/pubsub/subscribe` - Real-time subscriptions
- `GET /v1/pubsub/topics` - List topics
- `GET /v1/pubsub/subscriptions` - List active subscriptions

### Step 5.3: Database API
**Files to create:**
```
pkg/gateway/handlers/database.go
pkg/gateway/api/database.go
```

**Endpoints:**
- `POST /v1/db/query` - Execute SELECT queries
- `POST /v1/db/execute` - Execute INSERT/UPDATE/DELETE
- `POST /v1/db/batch` - Execute multiple statements
- `GET /v1/db/tables` - List namespace tables
- `POST /v1/db/migrate` - Run schema migrations

### Deliverables Phase 5
- [ ] All CRUD operations working with namespace isolation
- [ ] WebSocket subscriptions functional and secure
- [ ] Database operations isolated per namespace
- [ ] API documentation generated

---

## Phase 6: Rate Limiting & Quotas (Week 4)

### Objective
Add usage controls, monitoring, and tier-based quotas.

### Step 6.1: Rate Limiter Implementation
**Files to create:**
```
pkg/gateway/ratelimit/limiter.go
pkg/gateway/ratelimit/middleware.go
pkg/gateway/ratelimit/storage.go
pkg/gateway/ratelimit/config.go
```

**Implementation:**
- Token bucket algorithm implementation
- Per-namespace rate limiting
- Redis backend for distributed limiting
- Configurable limits per endpoint

### Step 6.2: Usage Tracking
**Files to create:**
```
pkg/gateway/usage/tracker.go
pkg/gateway/usage/quotas.go
pkg/gateway/usage/metrics.go
pkg/gateway/usage/reporter.go
```

**Implementation:**
- Track API calls per namespace
- Monitor resource usage (storage, database queries)
- Export Prometheus metrics
- Daily/monthly usage reports

### Step 6.3: Tier Enforcement
**Files to create:**
```
pkg/gateway/middleware/tier.go
pkg/gateway/subscription/tiers.go
pkg/gateway/subscription/limits.go
```

**Tier Limits:**
- **Free**: 250 RPM, 10k requests/day
- **Basic**: 1000 RPM, 100k requests/day, 100MB storage
- **Pro**: 5000 RPM, 1M requests/day, 1GB storage
- **Elite**: Unlimited RPM, 10M requests/day, 10GB storage

### Deliverables Phase 6
- [ ] Rate limiting active and configurable
- [ ] Usage tracking in database
- [ ] Tier-based quotas enforced
- [ ] Metrics exported for monitoring

---

## Phase 7: Payment & Subscription System (Week 4-5)

### Objective
Implement the Ethereum-based payment and subscription system.

### Step 7.1: Smart Contract Integration
**Files to create:**
```
pkg/gateway/blockchain/client.go
pkg/gateway/blockchain/contracts.go
pkg/gateway/blockchain/verifier.go
pkg/gateway/blockchain/events.go
```

**Implementation:**
- Ethereum client setup (mainnet + testnet)
- Payment verification smart contracts
- Event listening for payments
- Transaction verification

### Step 7.2: Subscription Management
**Files to create:**
```
pkg/gateway/subscription/manager.go
pkg/gateway/subscription/validator.go
pkg/gateway/subscription/renewal.go
pkg/gateway/subscription/pricing.go
```

**Pricing:**
- **Basic**: 0.1 ETH/month
- **Pro**: 0.2 ETH/month  
- **Elite**: 0.3 ETH/month
- **Testnet**: Free for testing

### Step 7.3: Payment Endpoints
**Files to create:**
```
pkg/gateway/handlers/payments.go
pkg/gateway/handlers/subscriptions.go
```

**Endpoints:**
- `POST /v1/payments/subscribe` - Initiate subscription
- `GET /v1/payments/status` - Check payment status
- `POST /v1/payments/verify` - Verify blockchain payment
- `GET /v1/subscriptions/current` - Get subscription details
- `POST /v1/subscriptions/cancel` - Cancel subscription

### Deliverables Phase 7
- [ ] Payment verification working on mainnet/testnet
- [ ] Subscription status tracking
- [ ] Automatic tier application based on payments
- [ ] Payment event monitoring

---

## Phase 8: Testing & Hardening (Week 5)

### Objective
Comprehensive testing, security auditing, and performance optimization.

### Step 8.1: Integration Tests
**Files to create:**
```
tests/integration/auth_test.go
tests/integration/namespace_test.go
tests/integration/api_test.go
tests/integration/payments_test.go
tests/security/isolation_test.go
```

**Test Coverage:**
- Full API test suite
- Cross-namespace security tests
- Rate limit and quota tests
- Payment flow tests
- WebSocket connection tests

### Step 8.2: Load Testing
**Files to create:**
```
tests/load/k6_scripts/
tests/load/websocket_stress.js
tests/load/api_concurrent.js
```

**Testing:**
- Concurrent user simulations
- WebSocket stress testing
- Database connection pooling tests
- Rate limiter performance tests

### Step 8.3: Security Audit
**Files to create:**
```
docs/security_audit.md
tests/security/penetration_tests.go
```

**Security Checks:**
- Input validation on all endpoints
- SQL injection prevention
- Rate limit bypass attempts
- JWT security verification
- Cross-namespace isolation verification

### Deliverables Phase 8
- [ ] 80%+ test coverage across all components
- [ ] Load test results and performance benchmarks
- [ ] Security audit report with findings
- [ ] Performance optimization recommendations

---

## Quick Start Implementation Order

For immediate progress, implement in this exact order:

### Day 1-2: Minimal Gateway
1. Create `cmd/gateway/main.go` with basic HTTP server
2. Add health check endpoint (`/health`)
3. Connect to network using existing `pkg/client`
4. Test basic connectivity

### Day 3-4: Database Foundation  
1. Create migration files with core tables
2. Setup database connection and models
3. Add app registration endpoint (no auth yet)
4. Test database operations

### Day 5-7: Basic Authentication
1. Implement nonce generation and storage
2. Add Ethereum wallet signature verification
3. Create JWT token system
4. Add authentication middleware

### Week 2: Core Security Features
1. Add namespace isolation middleware
2. Implement resource prefixing
3. Update handlers for namespace isolation
4. Add basic rate limiting

### Week 3: Complete API
1. Implement all storage endpoints
2. Add WebSocket support for pub-sub
3. Complete database API
4. Add comprehensive error handling

### Week 4: Production Ready
1. Add usage tracking and quotas
2. Implement tier-based limiting
3. Add payment verification
4. Complete subscription management

### Week 5: Testing & Launch
1. Write integration tests
2. Perform security testing
3. Load testing and optimization
4. Documentation and deployment

---

## File Structure Overview

```
cmd/gateway/
├── main.go                          # Entry point
└── config.yaml                      # Configuration

pkg/gateway/
├── server/
│   ├── server.go                    # HTTP server setup
│   └── routes.go                    # Route definitions
├── config/
│   └── config.go                    # Configuration management
├── db/
│   ├── connection.go                # Database connection
│   ├── models.go                    # Data models
│   ├── queries.go                   # SQL queries
│   └── migrations.go                # Migration runner
├── auth/
│   ├── ethereum.go                  # Wallet authentication
│   ├── jwt.go                       # JWT handling
│   └── nonce.go                     # Nonce management
├── handlers/
│   ├── auth.go                      # Auth endpoints
│   ├── storage.go                   # Storage API
│   ├── pubsub.go                    # PubSub API
│   ├── database.go                  # Database API
│   └── payments.go                  # Payment API
├── middleware/
│   ├── auth.go                      # Authentication
│   ├── namespace.go                 # Namespace isolation
│   ├── ratelimit.go                 # Rate limiting
│   └── cors.go                      # CORS handling
├── isolation/
│   ├── storage.go                   # Storage isolation
│   ├── pubsub.go                    # PubSub isolation
│   └── database.go                  # Database isolation
├── subscription/
│   ├── manager.go                   # Subscription management
│   └── tiers.go                     # Tier definitions
├── blockchain/
│   ├── client.go                    # Ethereum client
│   └── verifier.go                  # Payment verification
└── websocket/
    ├── manager.go                   # WebSocket management
    └── subscriber.go                # PubSub subscriptions

migrations/
├── 001_initial.sql                  # Initial schema
├── 002_indexes.sql                  # Performance indexes
└── 003_payments.sql                 # Payment tables

tests/
├── integration/                     # Integration tests
├── security/                        # Security tests
└── load/                           # Load tests

docs/
├── api.md                          # API documentation
├── security.md                     # Security guidelines
└── deployment.md                   # Deployment guide
```

---

## Success Metrics

### Phase 1 Success Criteria
- [ ] Gateway starts and connects to network
- [ ] Health endpoint returns 200 OK
- [ ] Basic storage operations work

### Phase 2 Success Criteria  
- [ ] Database migrations run successfully
- [ ] CRUD operations work for all models
- [ ] Namespace management functional

### Phase 3 Success Criteria
- [ ] Ethereum wallet authentication works
- [ ] JWT tokens generated and validated
- [ ] Session management operational

### Phase 4 Success Criteria
- [ ] Cross-namespace access blocked
- [ ] All resources properly isolated
- [ ] Security tests pass

### Phase 5 Success Criteria
- [ ] All API endpoints functional
- [ ] WebSocket subscriptions work
- [ ] Complete feature parity with direct client

### Phase 6 Success Criteria
- [ ] Rate limiting enforced
- [ ] Usage tracking accurate
- [ ] Tier limits respected

### Phase 7 Success Criteria
- [ ] Payment verification works
- [ ] Subscription management complete
- [ ] Automatic tier upgrades/downgrades

### Phase 8 Success Criteria
- [ ] 80%+ test coverage
- [ ] Security audit passed
- [ ] Load testing completed
- [ ] Production deployment ready

---

This implementation plan provides a clear roadmap from basic gateway functionality to a production-ready, secure, multi-tenant system with Ethereum-based payments and comprehensive API coverage.