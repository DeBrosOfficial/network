# DeBros Network — Gateway, Auth & Staking TASKS

This document captures the plan, endpoints, auth/staking model, data layout, security hardening, and an implementation roadmap for the new `gateway` service. It is a single-source checklist to turn the ideas discussed into working code and infra.

Goals
- Provide a standalone `cmd/gateway` binary that exposes HTTP (default) and optional gRPC to allow non-Go clients (JS/Swift/etc.) to access Database, Storage, PubSub and Network features.
- Authenticate and authorize apps via wallet-based verification and on-chain staking / NFT attestation.
- Issue short-lived access tokens (JWT) + rotating refresh tokens + optional API keys for server apps.
- Use staking or NFT ownership to grant higher rate limits and scopes.
- Keep node processes and gateway process separable for scaling, security and reliability.
- Store gateway/core metadata in a dedicated `core` RQLite database to avoid mixing runtime app data with cluster DB.

High-level architecture
- `cmd/gateway` (new binary)
  - HTTP server (REST + WebSocket)
  - Optional gRPC server
  - Bridge layer that calls `pkg/client` (NetworkClient) to interact with the network
  - Auth & staking modules, token manager, rate-limiter, background chain watcher
- `pkg/gateway` packages
  - `bridge` — adapters to call `client` methods
  - `http` — REST handlers & middleware
  - `ws` — WebSocket pubsub broker
  - `auth` — challenge, register, JWT + refresh token handling
  - `payments` — payment adapters, payment verification, subscription state
  - `rate` — Redis-backed token-bucket / quota manager
  - `db` — gateway schema migrations / helper for `core` DB
- Persistence
  - `core` RQLite database (separate from application DBs) stores apps, stakes, tokens and nonces
  - Redis for rate-limiting and ephemeral session state (optional fallback to in-memory for dev)
- Payment adapters
  - Ethereum (EVM) JSON-RPC adapter (support for mainnet + testnets such as Goerli)
  - Abstract interface allows adding other chains later if desired

Endpoints (HTTP + WebSocket) — MVP (no admin endpoints)
- General
  - GET /v1/health
  - GET /v1/version
- Network
  - GET /v1/network/peers
  - GET /v1/network/status
  - POST /v1/network/connect { multiaddr }
  - POST /v1/network/disconnect { peer_id }
- Database
  - POST /v1/db/query { sql, params?, timeout? }  (enforce scopes)
  - POST /v1/db/transaction { queries: [] }
  - GET  /v1/db/schema
  - POST /v1/db/create-table { sql }  (admin / gated)
- Storage
  - GET  /v1/storage/get?key=&namespace=
  - POST /v1/storage/put (binary body or JSON base64) ?key=&namespace=
  - DELETE /v1/storage/delete { key, namespace }
  - GET /v1/storage/list?prefix=&limit=&namespace=
  - GET /v1/storage/exists?key=&namespace=
- Pub/Sub
  - POST /v1/pubsub/publish { topic, data(base64|raw), namespace?, ttl? }
  - GET /v1/pubsub/topics?namespace=
  - WS /v1/pubsub/ws  (subscribe/unsubscribe/publish over WS frames)
  - SSE /v1/pubsub/sse?topic=... (optional read-only)
- Auth & App onboarding
  - POST /v1/auth/challenge { wallet, wallet_type, app_name, metadata? }
    - Response: { challenge, expires_in }
  - POST /v1/auth/register { wallet, wallet_type, challenge, signature, app_name, metadata? }
    - On success: create provisional App and return client_id (status: pending or active per flow)
  - POST /v1/auth/refresh { client_id, refresh_token } -> new access_token
  - GET  /v1/auth/whoami (protected)
- Staking (on-chain)
  - POST /v1/stake/info { client_id }  -> returns required stake, contract address, memo format
  - POST /v1/stake/commit { client_id, chain, tx_signature } -> verify on-chain, update stake
  - GET /v1/stake/status?client_id=...
  - POST /v1/stake/unstake { client_id } -> returns steps and marks pending_unstake

Authentication & authorization model (recommended MVP)
- App registration:
  1. Client obtains ephemeral `challenge` for a wallet.
  2. Client signs `challenge` with wallet and calls `/v1/auth/register`.
  3. Gateway verifies signature and creates `app` record (status pending or active).
- App activation:
  - NFT path: if the wallet holds qualifying NFT(s), gateway verifies and activates the app.
  - Staking path: gateway asks you to stake tokens to a staking contract with a memo bound to `client_id`. After verifying the on-chain tx, gateway activates the app and assigns a tier.
- Tokens:
  - Issue short-lived JWT access tokens (e.g., 15m) signed by gateway private key (RS256 or ES256). Publish JWKS at `/.well-known/jwks.json`.
  - Issue rotating refresh tokens (keep hashed in `core` DB).
  - Optionally issue API keys (hashed) for server-to-server use (longer TTL, revokable).
- JWT claims:
  - iss, sub (client_id), aud, exp, iat, jti
  - namespace, wallet, wallet_type, scopes, stake_amount, stake_chain, tier
- Scopes:
  - `storage:read`, `storage:write`, `pubsub:publish`, `pubsub:subscribe`, `db:read`, `db:write`
  - Enforce scopes in middleware for each endpoint.

Payment / Subscription model
- Pricing & plans (Ethereum-based monthly payments)
  - The gateway requires paid subscriptions to use the network. Example starter plans:
    - Basic: 0.1 ETH / month -> default quota (e.g., 1,000 RPM)
    - Pro:   0.2 ETH / month -> higher quota (e.g., 5,000 RPM)
    - Elite: 0.3 ETH / month -> top quota (e.g., 50,000 RPM)
  - Plans are configurable and billed per subscription period (monthly by default).
- Payment verification mode:
  - Transaction-proof commit (MVP): the user pays the gateway's billing address or staking/payment contract on Ethereum. The payment transaction must include a memo/metadata field or be directed to a payment endpoint structured to identify the `client_id`. The gateway verifies the transaction on-chain (via JSON-RPC) and marks the subscription active for the paid period.
  - Event-driven contract listening (optional): deploy a simple subscription contract that emits `PaymentMade(client_id, wallet, plan, amount, tx)` events — gateway listens and reconciles subscriptions automatically.
- Testnet support:
  - Support Ethereum testnets (e.g., Goerli) for testing flows without spending real ETH. Gateway config must allow testnet mode and separate testnet payment address/contract.
- Billing cycle & renewal:
  - When a payment is verified, set subscription validity for the plan period (e.g., 30 days). The gateway should notify (via webhook or SDK callback) before expiration and support manual or automated renewal (client submits another payment).
  - If payment is missed or subscription expires, downgrade quotas to the free/default plan or suspend access depending on policy.
- Confirmation requirements:
  - Require configurable confirmation counts before marking a payment as final (example: 12 confirmations for Ethereum mainnet; lower for testnet in dev).
- Refunds & dispute handling:
  - Gateway should define a refund/dispute policy (manual or automated) — out of scope for MVP but planned.

Database & storage plan
- Use a separate RQLite logical DB called `core` to store gateway metadata. Rationale:
  - Avoid mixing application data with gateway operational metadata.
  - Easier backups / migrations for gateway-only state.
- `core` schema (sketch)
  - `apps`:
    - id UUID, client_id TEXT (unique), namespace TEXT, wallet_pubkey TEXT, wallet_type TEXT, scopes JSON, status TEXT, metadata JSON, created_at, updated_at
  - `nonces`:
    - nonce TEXT PK, wallet_pubkey TEXT, created_at, expires_at, used BOOL, ip_addr TEXT
  - `subscriptions`:
    - id UUID, app_id FK, chain TEXT, plan TEXT, amount_paid NUMERIC, tx_signature TEXT, confirmed BOOL, confirmations INT, period_start TIMESTAMP, period_end TIMESTAMP, auto_renew BOOL, testnet BOOL, created_at, updated_at
  - `refresh_tokens`:
    - jti TEXT PK, client_id FK, hashed_token TEXT, expires_at TIMESTAMP, revoked BOOL, created_at
  - `api_keys`:
    - id UUID, app_id, hashed_key TEXT, description, created_at, revoked BOOL
  - `audit_events`:
    - id UUID, app_id nullable, event_type TEXT, details JSON, created_at
- Access patterns:
  - Gateway writes to `core`; background watcher verifies payments/subscriptions and writes audit events.
  - When validating JWTs, check `jti` blacklist and optionally verify `apps.status` and `subscriptions` validity.

Rate-limiting architecture
- Use Redis token-bucket per `client_id` for production. Fallback to in-memory limiter for dev.
- Keyed by `client_id`; token bucket parameters derived from subscription plan or default (free) plan.
- Quota update flow:
  - When subscription status changes (payment commit / expiration / renewal), background worker recalculates plan quotas and sets new token-bucket capacity in Redis.
  - Middleware consumes tokens on request; return 429 when exhausted.
- Consider endpoint-specific quota (DB-write quotas smaller than read/storage).

Background workers & payment watcher
- Background service tasks:
  - Payment watcher: listen to payment contract events or poll transactions; verify payments, mark confirmations, and update `subscriptions`.
  - Token revocation worker: expire revoked tokens from cache; enforce `jti` blacklist.
  - Quota reconciler: push new quotas to Redis after subscription changes (payment/expiration/renewal).
  - Audit logger: persist critical events to `core.audit_events`.

Security hardening (key points)
- Transport: require TLS for all gateways. Support mTLS for internal connections if desired.
- Signature verification:
  - Support Solana (ed25519) and Ethereum (secp256k1 SIWE/EIP-191) signature formats.
  - Challenges are single-use and time-limited.
- JWT & keys:
  - Use RS256/ES256 and store private keys in KMS/HSM. Publish JWKS for clients.
  - Rotate keys and keep old keys in JWKS until no tokens signed by them remain.
- Token lifecycle:
  - Short access TTL, rotating refresh tokens with hashed storage, jti blacklisting for revocation.
- Input validation:
  - Enforce strict validation on SQL queries (parameterized), topic names, key names, sizes.
- Rate-limits + quotas:
  - Enforce per-client quotas. Additional IP-based rate-limiting as fallback.
- Anti-sybil for registration:
  - Rate-limit challenge requests per IP and per wallet address.
  - Require payments/NFT for elevated capabilities (paid plans or verified NFT holders).
  - Apply CAPTCHAs or step-up verification if suspicious activity is detected (many signups from same IP/wallet).
  - Monitor behavioral signals (usage spikes, repeated failures) and automatically throttle or block abusive actors.

- Multi-tenant isolation & namespace enforcement:
  - Principle: every App must be strictly namespaced. All cross-service operations (DB, Storage, Pub/Sub, Network actions) must be checked and scoped to the App's namespace as declared in the App record and embedded in JWTs.
  - JWT binding: issue tokens that include a `namespace` claim. All gateway handlers must verify that any request-scoped `namespace` parameter or implied namespace (for example when creating resources) matches the token's `namespace`.
  - Storage keys: internally prefix all storage operations with the namespace (e.g., `ns::<namespace>::<key>`). The gateway API must require either an explicit `namespace` parameter or infer it from the token; never accept a raw key without namespacing.
  - Pub/Sub topics: require namespaced topic names (e.g., `<namespace>.<topic>`). Reject topic operations that omit or try to impersonate a different namespace. When forwarding subscriptions to the internal pubsub layer, map to namespaced topics only.
  - Database isolation and table naming:
    - Prefer one of these techniques (ordered by recommended deployment complexity):
      1. Logical per-app RQLite DB (best isolation): create/assign a dedicated logical DB or database file for each app so tables live in the app's DB and cannot collide.
      2. Table name prefixing (practical): prefix table names with namespace (e.g., `ns__<namespace>__users`) and enforce gateway-only creation to avoid collisions.
      3. SQL sandboxing + query rewriting: rewrite queries to inject namespace-qualified table names or run them in a namespaced schema; disallow raw `ATTACH`/`DETACH` and other DDL that can escape namespace.
    - For MVP, implement table name prefixing and forbid arbitrary DDL by non-admin apps. If you later need stronger isolation, migrate to per-app logical DBs.
  - Resource creation rules:
    - Only allow apps to create resources (tables, topics, storage keys) within their namespace.
    - When a create request arrives, the gateway must validate:
      - The token `namespace` matches the requested namespace.
      - The resource name, after applying namespace prefix, does not already exist under another namespace.
      - Enforce strict name validation (regex), disallowing `..`, slashes and control chars.
  - Prevent namespace collisions and impersonation:
    - Reserve a namespace namespace-ownership table in `core` that records `owner_app_id`, `namespace`, `created_at`, and optional `domain`/`metadata`.
    - Reject any create attempt for a namespace that is already registered to another app.
    - Provide an admin-only transfer mechanism for namespace ownership (manual transfer requires validation).
  - Middleware & enforcement:
    - Implement a centralized namespace enforcement middleware that runs before handler logic:
      - Extract `namespace` from JWT.
      - Compare to requested namespace (query/body/path). If mismatch, return 403.
      - For internal DB calls, automatically apply namespace prefix mapping.
    - Log and audit any 403 cross-namespace attempts for monitoring and alerts.
  - SQL & command safety:
    - Disallow dangerous SQL statements from non-admin tokens: `ATTACH`, `DETACH`, `PRAGMA` (sensitive), `ALTER` (unless permitted), and `DROP TABLE` without proper scope checks.
    - Enforce parameterized queries only. Optionally maintain a whitelist of allowed DDL/DDL-like statements for higher-tier apps that request explicit permissions.
  - Testing & validation:
    - Add unit and integration tests that attempt cross-namespace access (DB reads/writes, storage key reads, pubsub subscriptions) and assert they are rejected.
    - Add fuzz tests for topic and key naming to ensure sanitization and prefixing is robust.
  - Migration & operator notes:
    - Document the namespace naming convention and migration path if you need to move an app from prefixing to per-app DB later.
    - Provide tooling to inspect `core.namespace_ownership` and reconcile accidental collisions or orphaned namespaces.
- Secrets management:
  - Store API keys and refresh tokens hashed.
  - Protect DB credentials and RQLite nodes with network controls.
- Audit & monitoring:
  - Structured logs with redaction rules.
  - Prometheus metrics for key events: stake commits, registration attempts, JWT issuance, quota breaches.

Operational & deployment considerations
- Separate gateway process to scale independently from nodes.
- For local/demo: provide `--embedded` or `node --enable-gateway` dev-only mode that runs gateway in-process on localhost.
- Expose Prometheus `/metrics`.
- Provide configuration flags:
  - `--http-listen`, `--grpc-listen`, `--tls-cert`, `--tls-key`, `--jwks-url`, `--stake-contracts`, `--chain-rpc` endpoints, `--redis-url`, `--core-db-endpoint`
- Run gateway behind an API gateway / LB in production for TLS termination or WAF if needed.
- Use KMS for signing key management and automated cert issuance (cert-manager or cloud provider).

Implementation roadmap (milestones & tasks)
Phase 0 — Design & infra
- Finalize JWKS/token signing approach (RS256 vs ES256).
- Define staking contract interface for Solana (and EVM if planned).
- Create `core` RQLite schema SQL migrations.

Phase 1 — Minimal Gateway MVP
- Scaffold `cmd/gateway` and `pkg/gateway/bridge`.
- Implement:
  - `/v1/auth/challenge` and `/v1/auth/register` (wallet signature verification)
  - Token issuance (JWT + refresh token)
  - Simple `apps` CRUD in `core` DB
  - Basic endpoints: `/v1/health`, `/v1/network/peers`, `/v1/pubsub/publish`, `/v1/storage/get|put`
  - Basic WebSocket pubsub `/v1/pubsub/ws`
  - Local in-memory rate limiter with default quotas
- Create TypeScript example client demonstrating challenge → sign → register → use token → WS pubsub.

Phase 2 — Payments & quotas
- Add payment endpoints: `/v1/payments/info`, `/v1/payments/commit`, `/v1/payments/status`
- Implement Ethereum (EVM) adapter to verify payment txs via JSON-RPC (support mainnet + testnets such as Goerli)
- Add Redis-backed rate limiter and plan mapping based on subscription plan (Basic/Pro/Elite)
- Implement background payment watcher to verify transactions, confirm payments, set subscription periods, and push quota updates
- Provide testnet configuration and flows so integrations can be tested without spending real ETH

Phase 3 — Production hardening & SDKs
- Integrate persistent Redis + RQLite `core` DB in prod config
- Replace in-memory limiter with Redis; add quota recalculation on stake changes
- Add JWKS endpoints, key rotation, KMS integration
- Add API key issuance (hashed)
- Add OpenAPI spec and generate JS/Swift SDKs
- Add metrics, logging, alerting and documentation

Phase 4 — Optional: On-chain contracts & advanced flows
- Deploy staking contract (Solana/EVM) with event emission
- Add NFT attestation flow
- (Optional) Implement direct libp2p-js path for browser-native P2P

Developer tasks — immediate actionable items
1. Create RQLite `core` DB migration SQL and add to repo migrations (include `apps`, `nonces`, `subscriptions`, `refresh_tokens`, `api_keys`, `audit_events`, and `namespace_ownership` tables).
2. Scaffold `cmd/gateway/main.go` with flags `--http-listen`, `--grpc`, `--tls-*`, `--redis`, `--core-db`.
3. Implement `pkg/gateway/auth` with challenge/register handlers and Ethereum signature verification helper (for EOA flows / SIWE).
4. Implement `pkg/gateway/bridge` to call `client.NewClient(DefaultClientConfig(namespace))` and wire basic endpoints (pubsub publish, storage get/put).
5. Add WebSocket pubsub forwarding using `client.PubSub().Subscribe` and map to WS sessions.
6. Add Redis-based token-bucket `pkg/gateway/rate` and middleware for HTTP endpoints.
7. Implement `/v1/payments/commit` Ethereum adapter skeleton (verify payment tx via JSON-RPC and support testnets like Goerli).
8. Produce OpenAPI (YAML) for the endpoints to allow SDK generation.
9. Build example TypeScript client that performs challenge -> sign -> register -> use payments on testnet -> publish/subscribe.
10. Implement namespace enforcement middleware:
    - Validate token `namespace` claim and ensure it matches any requested `namespace` parameter or infers namespace for the operation.
    - Map and apply namespace prefixes to storage keys, pubsub topics, and DB table names.
    - Reject attempts to access or create resources outside the token's namespace (return 403).
11. Add `core.namespace_ownership` table and enforcement logic to prevent two apps from owning the same namespace; disallow create requests for reserved/owned namespaces.
12. Implement create-resource guards:
    - Ensure table/topic/key creation requests include the namespace and that the gateway applies/validates namespace prefixes before creating resources.
    - Disallow non-admin DDL that can escape namespace boundaries (`ATTACH`, `DETACH`, raw file access).
13. Add unit and integration tests for multi-tenant isolation:
    - Tests that verify reads/writes across namespaces are rejected.
    - Tests that verify topic and storage key isolation enforcement.
14. Add audit hooks to log any cross-namespace access attempts and integrate alerts for repeated violations.
15. Update API documentation and SDKs to document the namespace requirement and show examples of correctly namespaced calls.

Notes & guidelines
- Use separate `core` logical DB name when creating rqlite connections: `http://<host>:<port>/core` or a connection that uses a dedicated DB directory for the gateway.
- Keep gateway stateless where possible: store short-lived state in Redis. Persistent state goes to `core` RQLite.
- Prefer parameterized SQL calls in gateway code when writing to `core`.
- For wallet signature verification use battle-tested crypto libs (Solana ed25519 from x/crypto, Ethereum ecrecover libs) and accept explicit `wallet_type`.
- Keep WebSocket messages compact (use base64 for binary payloads) and add per-connection subscription limits.

Open questions to finalize before coding
- Which chains to support in v1? (Solana recommended as first)
- Exact stake thresholds and confirmation counts for each chain
- JWKS key storage policy (local PEM for dev; KMS in prod)
- Redis availability & cluster sizing for rate-limiter
- Should `core` RQLite be colocated on node or run as separate RQLite node cluster? (Separate RQLite logical DB is recommended)

If you want, I can now:
- Generate the `network/TASK.md` (this file) as well as scaffolded Go handler stubs for `/v1/auth/challenge`, `/v1/auth/register`, `/v1/payments/commit` and example SQL migrations for `core` DB (including `subscriptions` table).
- Or produce an OpenAPI spec for the MVP endpoints so you can generate SDKs. I can also produce example testnet payment flows (Goerli) and a TypeScript test client that demonstrates paying on testnet and activating a subscription.

Tell me which code artifact you want next and I will produce it.