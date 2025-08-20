# Task: Enforce API Key/JWT and Namespace in Go Client (Auto-Resolve Namespace) and Guard All Operations

Owner: To be assigned
Status: Ready to implement

## Objective
Implement strict client-side access enforcement in the Go client (`pkg/client`) so that:
- An API key or JWT is required by default to use the client.
- The client auto-resolves the namespace from the provided API key or JWT without requiring callers to pass the namespace per call.
- Per-call namespace overrides via context are still allowed for compatibility, but must match the resolved namespace; otherwise, deny the call.
- All operations (Storage, PubSub, Database/RQLite, and NetworkInfo) are guarded and return access errors when unauthenticated or namespace-mismatched.
- No backward compatibility guarantees required.

Note: This is client-side enforcement for now. Protocol-level auth/ACL for libp2p can be added later.

## High-level behavior
- `ClientConfig.RequireAPIKey` defaults to true. If true and neither `APIKey` nor `JWT` is present, `Connect()` fails.
- Namespace is automatically derived:
  - From JWT: parse claims and read `Namespace` claim (no network roundtrip). Verification of signature is not required for this task; parsing is enough to derive namespace. Optionally, add a TODO hook for future verification against JWKS if provided.
  - From API key: the namespace must be embedded in the key using a documented format (below). The client parses it locally and derives the namespace without any remote calls.
- All calls check that any provided per-call namespace override matches the derived namespace, else return an “access denied: namespace mismatch” error.
- All modules are guarded: Database (RQLite), Storage, PubSub, and NetworkInfo.

## API key and JWT formats
- JWT: RS256 token with claim `Namespace` (string). We will parse claims (unverified) to obtain `Namespace`.
- API key: change to an encoded format that includes the namespace so the client can parse locally. Options (pick one and implement consistently):
  - Option A (dotted): `ak_<random>.<namespace>`
  - Option B (colon): `ak_<random>:<namespace>`
  - Option C (base64 JSON): base64url of `{ "kid": "...", "ns": "<namespace>" }` prefixed by `ak_`

For simplicity and readability, choose Option B: `ak_<random>:<namespace>`.
- Parsing rules:
  - If `APIKey` contains a single colon, split and use the right side as `namespace` (trim spaces). If empty -> error.
  - If more than one colon or invalid format -> error.

## Changes to implement

### 1) Client configuration and types
- File: `pkg/client/interface.go`
  - Extend `ClientConfig`:
    - `Namespace string` // optional; if empty, auto-derived from API key or JWT; if still empty, fallback to `AppName`.
    - `RequireAPIKey bool` // default true; when true, require either `APIKey` or `JWT`.
    - `JWT string` // optional bearer token; used for namespace derivation and future protocol auth.
  - Update `DefaultClientConfig(appName string)` to set:
    - `RequireAPIKey: true`
    - `Namespace: ""` (meaning auto)

### 2) Namespace resolution and access gating
- File: `pkg/client/client.go`
  - At construction or `Connect()` time:
    - Implement `deriveNamespace()`:
      - If `config.Namespace != ""`, use it.
      - Else if `config.JWT != ""`, parse JWT claims (unverified) and read `Namespace` claim.
      - Else if `config.APIKey != ""`, parse `ak_<random>:<namespace>` and extract namespace.
      - Else use `config.AppName`.
      - Store the resolved namespace back into `config.Namespace`.
    - Enforce presence of credentials:
      - If `config.RequireAPIKey` is true AND both `config.APIKey` and `config.JWT` are empty -> return error `access denied: API key or JWT required`.
  - Add `func (c *Client) requireAccess(ctx context.Context) error` that:
    - If `RequireAPIKey` and both `APIKey` and `JWT` are empty -> error `access denied: credentials required`.
    - Resolve per-call namespace override from context (via storage/pubsub helpers below). If present and `override != c.config.Namespace` -> error `access denied: namespace mismatch`.

### 3) Guard all operations
- File: `pkg/client/implementations.go`
  - At the start of each public method, call `client.requireAccess(ctx)` and return the error if any.
    - DatabaseClientImpl: `Query`, `Transaction`, `CreateTable`, `DropTable`, `GetSchema`.
    - StorageClientImpl: `Get`, `Put`, `Delete`, `List`, `Exists`.
    - NetworkInfoImpl: `GetPeers`, `GetStatus`, `ConnectToPeer`, `DisconnectFromPeer`.
  - For Storage operations, ensure we propagate the effective namespace:
    - If override present and equals `config.Namespace`, pass that context through; else use `storage.WithNamespace(ctx, config.Namespace)`.

### 4) PubSub context-based namespace override (parity with Storage)
- Files: `pkg/pubsub/*`
  - Add:
    - `type ctxKey string`
    - `const CtxKeyNamespaceOverride ctxKey = "pubsub_ns_override"`
    - `func WithNamespace(ctx context.Context, ns string) context.Context`
  - Update topic naming in `manager.go` and `subscriptions.go`/`publish.go`:
    - Before computing `namespacedTopic`, check for ctx override; if present and non-empty, use it; else fall back to `m.namespace`.

### 5) Client context helper
- New file: `pkg/client/context.go`
  - Add `func WithNamespace(ctx context.Context, ns string) context.Context` that applies both storage and pubsub overrides by chaining:
    - `ctx = storage.WithNamespace(ctx, ns)`
    - `ctx = pubsub.WithNamespace(ctx, ns)`
    - return `ctx`

### 6) Documentation updates
- Files: `README.md`, `AI_CONTEXT.md`
  - Document the new client auth behavior:
    - An API key or JWT is required by default (`RequireAPIKey=true`).
    - Namespace auto-derived from token:
      - JWT claim `Namespace`.
      - API key format `ak_<random>:<namespace>`.
    - Per-call override via `client.WithNamespace(ctx, ns)` allowed but must match derived namespace.
    - All modules (Storage, PubSub, Database, NetworkInfo) are guarded.
  - Provide usage examples for constructing `ClientConfig` with API key or JWT and making calls.

## Helper details
- JWT parsing: implement a minimal helper to split the token and base64url-decode the payload; read `Namespace` field from JSON. Do not verify signature for this task. If parsing fails, return a clear error.
- API key parsing: simple split on `:`; trim spaces; validate non-empty.

## Error messages (standardize)
- Missing credentials: `access denied: API key or JWT required`
- Namespace mismatch: `access denied: namespace mismatch`
- Client not connected: keep existing `client not connected` error.

## Acceptance criteria
- Without credentials and `RequireAPIKey=true`, `Connect()` returns error and no operations are allowed.
- With API key `ak_abc123:myapp`, the client auto-resolves namespace `myapp`; operations succeed.
- With JWT containing `{ "Namespace": "myapp" }`, the client auto-resolves `myapp`; operations succeed.
- If a caller sets `client.WithNamespace(ctx, "otherNS")` while resolved namespace is `myapp`, any operation returns `access denied: namespace mismatch`.
- PubSub topic names use the override when present (and allowed) else the resolved namespace.
- NetworkInfo methods are also guarded and require credentials.

## Out of scope (for this task)
- Protocol-level auth or verification of JWT signatures against JWKS.
- ETH payments/subscriptions and tier enforcement. (Separate design/implementation.)

## Files to modify/add
- Modify:
  - `pkg/client/interface.go`
  - `pkg/client/client.go`
  - `pkg/client/implementations.go`
  - `pkg/pubsub/manager.go`
  - `pkg/pubsub/subscriptions.go`
  - `pkg/pubsub/publish.go` (if exists; add override resolution there too)
  - `README.md`, `AI_CONTEXT.md`
- Add:
  - `pkg/pubsub/context.go` (if not present)
  - `pkg/client/context.go`

## Notes
- Keep logs concise and avoid leaking tokens in logs. You may log the resolved namespace at `INFO` level on connect.
- Ensure thread-safety when accessing `Client.config` fields (use existing locks if needed).
