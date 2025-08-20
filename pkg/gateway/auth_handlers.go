package gateway

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

    "git.debros.io/DeBros/network/pkg/storage"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
)

func (g *Gateway) whoamiHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// Determine namespace (may be overridden by auth layer)
	ns := g.cfg.ClientNamespace
	if v := ctx.Value(storage.CtxKeyNamespaceOverride); v != nil {
		if s, ok := v.(string); ok && s != "" {
			ns = s
		}
	}

	// Prefer JWT if present
	if v := ctx.Value(ctxKeyJWT); v != nil {
		if claims, ok := v.(*jwtClaims); ok && claims != nil {
			writeJSON(w, http.StatusOK, map[string]any{
				"authenticated": true,
				"method":        "jwt",
				"subject":       claims.Sub,
				"issuer":        claims.Iss,
				"audience":      claims.Aud,
				"issued_at":     claims.Iat,
				"not_before":    claims.Nbf,
				"expires_at":    claims.Exp,
				"namespace":     ns,
				"require_auth":  g.cfg != nil && g.cfg.RequireAuth,
			})
			return
		}
	}

	// Fallback: API key identity
	var key string
	if v := ctx.Value(ctxKeyAPIKey); v != nil {
		if s, ok := v.(string); ok {
			key = s
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"authenticated": key != "",
		"method":        "api_key",
		"api_key":       key,
		"namespace":     ns,
		"require_auth":  g.cfg != nil && g.cfg.RequireAuth,
	})
}

func (g *Gateway) challengeHandler(w http.ResponseWriter, r *http.Request) {
	if g.client == nil {
		writeError(w, http.StatusServiceUnavailable, "client not initialized")
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		Wallet   string `json:"wallet"`
		Purpose  string `json:"purpose"`
		Namespace string `json:"namespace"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if strings.TrimSpace(req.Wallet) == "" {
		writeError(w, http.StatusBadRequest, "wallet is required")
		return
	}
	ns := strings.TrimSpace(req.Namespace)
	if ns == "" {
		ns = strings.TrimSpace(g.cfg.ClientNamespace)
		if ns == "" { ns = "default" }
	}
	// Generate a URL-safe random nonce (32 bytes)
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate nonce")
		return
	}
	nonce := base64.RawURLEncoding.EncodeToString(buf)

	// Insert namespace if missing, fetch id
	ctx := r.Context()
	db := g.client.Database()
	if _, err := db.Query(ctx, "INSERT OR IGNORE INTO namespaces(name) VALUES (?)", ns); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	nres, err := db.Query(ctx, "SELECT id FROM namespaces WHERE name = ? LIMIT 1", ns)
	if err != nil || nres == nil || nres.Count == 0 || len(nres.Rows) == 0 || len(nres.Rows[0]) == 0 {
		writeError(w, http.StatusInternalServerError, "failed to resolve namespace")
		return
	}
	nsID := nres.Rows[0][0]

	// Store nonce with 5 minute expiry
	if _, err := db.Query(ctx,
		"INSERT INTO nonces(namespace_id, wallet, nonce, purpose, expires_at) VALUES (?, ?, ?, ?, datetime('now', '+5 minutes'))",
		nsID, req.Wallet, nonce, req.Purpose,
	); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"wallet":     req.Wallet,
		"namespace":  ns,
		"nonce":      nonce,
		"purpose":    req.Purpose,
		"expires_at": time.Now().Add(5 * time.Minute).UTC().Format(time.RFC3339Nano),
	})
}

func (g *Gateway) verifyHandler(w http.ResponseWriter, r *http.Request) {
	if g.client == nil {
		writeError(w, http.StatusServiceUnavailable, "client not initialized")
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		Wallet    string `json:"wallet"`
		Nonce     string `json:"nonce"`
		Signature string `json:"signature"`
		Namespace string `json:"namespace"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if strings.TrimSpace(req.Wallet) == "" || strings.TrimSpace(req.Nonce) == "" || strings.TrimSpace(req.Signature) == "" {
		writeError(w, http.StatusBadRequest, "wallet, nonce and signature are required")
		return
	}
	ns := strings.TrimSpace(req.Namespace)
	if ns == "" {
		ns = strings.TrimSpace(g.cfg.ClientNamespace)
		if ns == "" { ns = "default" }
	}
	ctx := r.Context()
	db := g.client.Database()
	nsID, err := g.resolveNamespaceID(ctx, ns)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	q := "SELECT id FROM nonces WHERE namespace_id = ? AND wallet = ? AND nonce = ? AND used_at IS NULL AND (expires_at IS NULL OR expires_at > datetime('now')) LIMIT 1"
	nres, err := db.Query(ctx, q, nsID, req.Wallet, req.Nonce)
	if err != nil || nres == nil || nres.Count == 0 {
		writeError(w, http.StatusBadRequest, "invalid or expired nonce")
		return
	}
	nonceID := nres.Rows[0][0]

	// EVM personal_sign verification of the nonce
	// Hash: keccak256("\x19Ethereum Signed Message:\n" + len(nonce) + nonce)
	msg := []byte(req.Nonce)
	prefix := []byte("\x19Ethereum Signed Message:\n" + strconv.Itoa(len(msg)))
	hash := ethcrypto.Keccak256(prefix, msg)

	// Decode signature (expects 65-byte r||s||v, hex with optional 0x)
	sigHex := strings.TrimSpace(req.Signature)
	if strings.HasPrefix(sigHex, "0x") || strings.HasPrefix(sigHex, "0X") {
		sigHex = sigHex[2:]
	}
	sig, err := hex.DecodeString(sigHex)
	if err != nil || len(sig) != 65 {
		writeError(w, http.StatusBadRequest, "invalid signature format")
		return
	}
	// Normalize V to 0/1 as expected by geth
	if sig[64] >= 27 {
		sig[64] -= 27
	}
	pub, err := ethcrypto.SigToPub(hash, sig)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "signature recovery failed")
		return
	}
	addr := ethcrypto.PubkeyToAddress(*pub).Hex()
	want := strings.ToLower(strings.TrimPrefix(strings.TrimPrefix(req.Wallet, "0x"), "0X"))
	got := strings.ToLower(strings.TrimPrefix(strings.TrimPrefix(addr, "0x"), "0X"))
	if got != want {
		writeError(w, http.StatusUnauthorized, "signature does not match wallet")
		return
	}

	// Mark nonce used now (after successful verification)
	if _, err := db.Query(ctx, "UPDATE nonces SET used_at = datetime('now') WHERE id = ?", nonceID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if g.signingKey == nil {
		writeError(w, http.StatusServiceUnavailable, "signing key unavailable")
		return
	}
    // Issue access token (15m) and a refresh token (30d)
    token, expUnix, err := g.generateJWT(ns, req.Wallet, 15*time.Minute)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// create refresh token
	rbuf := make([]byte, 32)
	if _, err := rand.Read(rbuf); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate refresh token")
		return
	}
	refresh := base64.RawURLEncoding.EncodeToString(rbuf)
	if _, err := db.Query(ctx, "INSERT INTO refresh_tokens(namespace_id, subject, token, audience, expires_at) VALUES (?, ?, ?, ?, datetime('now', '+30 days'))", nsID, req.Wallet, refresh, "gateway"); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token":       token,
		"token_type":         "Bearer",
		"expires_in":         int(expUnix - time.Now().Unix()),
		"refresh_token":      refresh,
		"subject":            req.Wallet,
		"namespace":          ns,
		"nonce":              req.Nonce,
		"signature_verified": true,
	})
}

// issueAPIKeyHandler creates or returns an API key for a verified wallet in a namespace.
// Requires: POST { wallet, nonce, signature, namespace }
// Behavior:
//  - Validates nonce and signature like verifyHandler
//  - Ensures namespace exists
//  - If an API key already exists for (namespace, wallet), returns it; else creates one
//  - Records namespace ownership mapping for the wallet and api_key
func (g *Gateway) issueAPIKeyHandler(w http.ResponseWriter, r *http.Request) {
    if g.client == nil {
        writeError(w, http.StatusServiceUnavailable, "client not initialized")
        return
    }
    if r.Method != http.MethodPost {
        writeError(w, http.StatusMethodNotAllowed, "method not allowed")
        return
    }
    var req struct {
        Wallet    string `json:"wallet"`
        Nonce     string `json:"nonce"`
        Signature string `json:"signature"`
        Namespace string `json:"namespace"`
        Plan      string `json:"plan"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeError(w, http.StatusBadRequest, "invalid json body")
        return
    }
    if strings.TrimSpace(req.Wallet) == "" || strings.TrimSpace(req.Nonce) == "" || strings.TrimSpace(req.Signature) == "" {
        writeError(w, http.StatusBadRequest, "wallet, nonce and signature are required")
        return
    }
    ns := strings.TrimSpace(req.Namespace)
    if ns == "" {
        ns = strings.TrimSpace(g.cfg.ClientNamespace)
        if ns == "" { ns = "default" }
    }
    ctx := r.Context()
    db := g.client.Database()
    // Resolve namespace id
    nsID, err := g.resolveNamespaceID(ctx, ns)
    if err != nil {
        writeError(w, http.StatusInternalServerError, err.Error())
        return
    }
    // Validate nonce exists and not used/expired
    q := "SELECT id FROM nonces WHERE namespace_id = ? AND wallet = ? AND nonce = ? AND used_at IS NULL AND (expires_at IS NULL OR expires_at > datetime('now')) LIMIT 1"
    nres, err := db.Query(ctx, q, nsID, req.Wallet, req.Nonce)
    if err != nil || nres == nil || nres.Count == 0 {
        writeError(w, http.StatusBadRequest, "invalid or expired nonce")
        return
    }
    nonceID := nres.Rows[0][0]
    // Verify signature like verifyHandler
    msg := []byte(req.Nonce)
    prefix := []byte("\x19Ethereum Signed Message:\n" + strconv.Itoa(len(msg)))
    hash := ethcrypto.Keccak256(prefix, msg)
    sigHex := strings.TrimSpace(req.Signature)
    if strings.HasPrefix(sigHex, "0x") || strings.HasPrefix(sigHex, "0X") { sigHex = sigHex[2:] }
    sig, err := hex.DecodeString(sigHex)
    if err != nil || len(sig) != 65 {
        writeError(w, http.StatusBadRequest, "invalid signature format")
        return
    }
    if sig[64] >= 27 { sig[64] -= 27 }
    pub, err := ethcrypto.SigToPub(hash, sig)
    if err != nil {
        writeError(w, http.StatusUnauthorized, "signature recovery failed")
        return
    }
    addr := ethcrypto.PubkeyToAddress(*pub).Hex()
    want := strings.ToLower(strings.TrimPrefix(strings.TrimPrefix(req.Wallet, "0x"), "0X"))
    got := strings.ToLower(strings.TrimPrefix(strings.TrimPrefix(addr, "0x"), "0X"))
    if got != want {
        writeError(w, http.StatusUnauthorized, "signature does not match wallet")
        return
    }
    // Mark nonce used
    if _, err := db.Query(ctx, "UPDATE nonces SET used_at = datetime('now') WHERE id = ?", nonceID); err != nil {
        writeError(w, http.StatusInternalServerError, err.Error())
        return
    }
    // Check if api key exists for (namespace, wallet) via linkage table
    var apiKey string
    r1, err := db.Query(ctx, "SELECT api_keys.key FROM wallet_api_keys JOIN api_keys ON wallet_api_keys.api_key_id = api_keys.id WHERE wallet_api_keys.namespace_id = ? AND LOWER(wallet_api_keys.wallet) = LOWER(?) LIMIT 1", nsID, req.Wallet)
    if err == nil && r1 != nil && r1.Count > 0 && len(r1.Rows) > 0 && len(r1.Rows[0]) > 0 {
        if s, ok := r1.Rows[0][0].(string); ok { apiKey = s } else { b, _ := json.Marshal(r1.Rows[0][0]); _ = json.Unmarshal(b, &apiKey) }
    }
    if strings.TrimSpace(apiKey) == "" {
        // Create new API key with format ak_<random>:<namespace>
        buf := make([]byte, 18)
        if _, err := rand.Read(buf); err != nil {
            writeError(w, http.StatusInternalServerError, "failed to generate api key")
            return
        }
        apiKey = "ak_" + base64.RawURLEncoding.EncodeToString(buf) + ":" + ns
        if _, err := db.Query(ctx, "INSERT INTO api_keys(key, name, namespace_id) VALUES (?, ?, ?)", apiKey, "", nsID); err != nil {
            writeError(w, http.StatusInternalServerError, err.Error())
            return
        }
        // Create linkage
        // Find api_key id
        rid, err := db.Query(ctx, "SELECT id FROM api_keys WHERE key = ? LIMIT 1", apiKey)
        if err == nil && rid != nil && rid.Count > 0 && len(rid.Rows) > 0 && len(rid.Rows[0]) > 0 {
            apiKeyID := rid.Rows[0][0]
            _, _ = db.Query(ctx, "INSERT OR IGNORE INTO wallet_api_keys(namespace_id, wallet, api_key_id) VALUES (?, ?, ?)", nsID, strings.ToLower(req.Wallet), apiKeyID)
        }
    }
    // Record ownerships (best-effort)
    _, _ = db.Query(ctx, "INSERT OR IGNORE INTO namespace_ownership(namespace_id, owner_type, owner_id) VALUES (?, 'api_key', ?)", nsID, apiKey)
    _, _ = db.Query(ctx, "INSERT OR IGNORE INTO namespace_ownership(namespace_id, owner_type, owner_id) VALUES (?, 'wallet', ?)", nsID, req.Wallet)

    writeJSON(w, http.StatusOK, map[string]any{
        "api_key":   apiKey,
        "namespace": ns,
        "plan":      func() string { if strings.TrimSpace(req.Plan) == "" { return "free" } else { return req.Plan } }(),
        "wallet":    strings.ToLower(strings.TrimPrefix(strings.TrimPrefix(req.Wallet, "0x"), "0X")),
    })
}

// apiKeyToJWTHandler issues a short-lived JWT for use with the gateway from a valid API key.
// Requires Authorization header with API key (Bearer or ApiKey or X-API-Key header).
// Returns a JWT bound to the namespace derived from the API key record.
func (g *Gateway) apiKeyToJWTHandler(w http.ResponseWriter, r *http.Request) {
    if g.client == nil {
        writeError(w, http.StatusServiceUnavailable, "client not initialized")
        return
    }
    if r.Method != http.MethodPost {
        writeError(w, http.StatusMethodNotAllowed, "method not allowed")
        return
    }
    key := extractAPIKey(r)
    if strings.TrimSpace(key) == "" {
        writeError(w, http.StatusUnauthorized, "missing API key")
        return
    }
    // Validate and get namespace
    db := g.client.Database()
    ctx := r.Context()
    q := "SELECT namespaces.name FROM api_keys JOIN namespaces ON api_keys.namespace_id = namespaces.id WHERE api_keys.key = ? LIMIT 1"
    res, err := db.Query(ctx, q, key)
    if err != nil || res == nil || res.Count == 0 || len(res.Rows) == 0 || len(res.Rows[0]) == 0 {
        writeError(w, http.StatusUnauthorized, "invalid API key")
        return
    }
    var ns string
    if s, ok := res.Rows[0][0].(string); ok { ns = s } else { b, _ := json.Marshal(res.Rows[0][0]); _ = json.Unmarshal(b, &ns) }
    ns = strings.TrimSpace(ns)
    if ns == "" {
        writeError(w, http.StatusUnauthorized, "invalid API key")
        return
    }
    if g.signingKey == nil {
        writeError(w, http.StatusServiceUnavailable, "signing key unavailable")
        return
    }
    // Subject is the API key string for now
    token, expUnix, err := g.generateJWT(ns, key, 15*time.Minute)
    if err != nil {
        writeError(w, http.StatusInternalServerError, err.Error())
        return
    }
    writeJSON(w, http.StatusOK, map[string]any{
        "access_token": token,
        "token_type":   "Bearer",
        "expires_in":   int(expUnix - time.Now().Unix()),
        "namespace":    ns,
    })
}

func (g *Gateway) registerHandler(w http.ResponseWriter, r *http.Request) {
	if g.client == nil {
		writeError(w, http.StatusServiceUnavailable, "client not initialized")
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		Wallet    string `json:"wallet"`
		Nonce     string `json:"nonce"`
		Signature string `json:"signature"`
		Namespace string `json:"namespace"`
		Name      string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if strings.TrimSpace(req.Wallet) == "" || strings.TrimSpace(req.Nonce) == "" || strings.TrimSpace(req.Signature) == "" {
		writeError(w, http.StatusBadRequest, "wallet, nonce and signature are required")
		return
	}
	ns := strings.TrimSpace(req.Namespace)
	if ns == "" {
		ns = strings.TrimSpace(g.cfg.ClientNamespace)
		if ns == "" { ns = "default" }
	}
	ctx := r.Context()
	db := g.client.Database()
	nsID, err := g.resolveNamespaceID(ctx, ns)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Validate nonce
	q := "SELECT id FROM nonces WHERE namespace_id = ? AND wallet = ? AND nonce = ? AND used_at IS NULL AND (expires_at IS NULL OR expires_at > datetime('now')) LIMIT 1"
	nres, err := db.Query(ctx, q, nsID, req.Wallet, req.Nonce)
	if err != nil || nres == nil || nres.Count == 0 || len(nres.Rows) == 0 || len(nres.Rows[0]) == 0 {
		writeError(w, http.StatusBadRequest, "invalid or expired nonce")
		return
	}
	nonceID := nres.Rows[0][0]

	// EVM personal_sign verification of the nonce
	msg := []byte(req.Nonce)
	prefix := []byte("\x19Ethereum Signed Message:\n" + strconv.Itoa(len(msg)))
	hash := ethcrypto.Keccak256(prefix, msg)

	// Decode signature (expects 65-byte r||s||v, hex with optional 0x)
	sigHex := strings.TrimSpace(req.Signature)
	if strings.HasPrefix(sigHex, "0x") || strings.HasPrefix(sigHex, "0X") {
		sigHex = sigHex[2:]
	}
	sig, err := hex.DecodeString(sigHex)
	if err != nil || len(sig) != 65 {
		writeError(w, http.StatusBadRequest, "invalid signature format")
		return
	}
	// Normalize V to 0/1 as expected by geth
	if sig[64] >= 27 {
		sig[64] -= 27
	}
	pub, err := ethcrypto.SigToPub(hash, sig)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "signature recovery failed")
		return
	}
	addr := ethcrypto.PubkeyToAddress(*pub).Hex()
	want := strings.ToLower(strings.TrimPrefix(strings.TrimPrefix(req.Wallet, "0x"), "0X"))
	got := strings.ToLower(strings.TrimPrefix(strings.TrimPrefix(addr, "0x"), "0X"))
	if got != want {
		writeError(w, http.StatusUnauthorized, "signature does not match wallet")
		return
	}

	// Mark nonce used now (after successful verification)
	if _, err := db.Query(ctx, "UPDATE nonces SET used_at = datetime('now') WHERE id = ?", nonceID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Derive public key (uncompressed) hex
	pubBytes := ethcrypto.FromECDSAPub(pub)
	pubHex := "0x" + hex.EncodeToString(pubBytes)

	// Generate client app_id
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate app id")
		return
	}
	appID := "app_" + base64.RawURLEncoding.EncodeToString(buf)

	// Persist app
	if _, err := db.Query(ctx, "INSERT INTO apps(namespace_id, app_id, name, public_key) VALUES (?, ?, ?, ?)", nsID, appID, req.Name, pubHex); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Record namespace ownership by wallet (best-effort)
	_, _ = db.Query(ctx, "INSERT OR IGNORE INTO namespace_ownership(namespace_id, owner_type, owner_id) VALUES (?, ?, ?)", nsID, "wallet", req.Wallet)

	writeJSON(w, http.StatusCreated, map[string]any{
		"client_id":          appID,
		"app": map[string]any{
			"app_id":     appID,
			"name":       req.Name,
			"public_key": pubHex,
			"namespace":  ns,
			"wallet":     strings.ToLower(strings.TrimPrefix(strings.TrimPrefix(req.Wallet, "0x"), "0X")),
		},
		"signature_verified": true,
	})
}

func (g *Gateway) refreshHandler(w http.ResponseWriter, r *http.Request) {
	if g.client == nil {
		writeError(w, http.StatusServiceUnavailable, "client not initialized")
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		RefreshToken string `json:"refresh_token"`
		Namespace    string `json:"namespace"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if strings.TrimSpace(req.RefreshToken) == "" {
		writeError(w, http.StatusBadRequest, "refresh_token is required")
		return
	}
	ns := strings.TrimSpace(req.Namespace)
	if ns == "" {
		ns = strings.TrimSpace(g.cfg.ClientNamespace)
		if ns == "" { ns = "default" }
	}
	ctx := r.Context()
	db := g.client.Database()
	nsID, err := g.resolveNamespaceID(ctx, ns)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	q := "SELECT subject FROM refresh_tokens WHERE namespace_id = ? AND token = ? AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at > datetime('now')) LIMIT 1"
	rres, err := db.Query(ctx, q, nsID, req.RefreshToken)
	if err != nil || rres == nil || rres.Count == 0 {
		writeError(w, http.StatusUnauthorized, "invalid or expired refresh token")
		return
	}
	subject := ""
	if len(rres.Rows) > 0 && len(rres.Rows[0]) > 0 {
		if s, ok := rres.Rows[0][0].(string); ok {
			subject = s
		} else {
			// fallback: format via json
			b, _ := json.Marshal(rres.Rows[0][0])
			_ = json.Unmarshal(b, &subject)
		}
	}
	if g.signingKey == nil {
		writeError(w, http.StatusServiceUnavailable, "signing key unavailable")
		return
	}
	token, expUnix, err := g.generateJWT(ns, subject, 15*time.Minute)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token":  token,
		"token_type":    "Bearer",
		"expires_in":    int(expUnix - time.Now().Unix()),
		"refresh_token": req.RefreshToken,
		"subject":       subject,
		"namespace":     ns,
	})
}

// logoutHandler revokes refresh tokens. If a refresh_token is provided, it will
// be revoked. If all=true is provided (and the request is authenticated via JWT),
// all tokens for the JWT subject within the namespace are revoked.
func (g *Gateway) logoutHandler(w http.ResponseWriter, r *http.Request) {
	if g.client == nil {
		writeError(w, http.StatusServiceUnavailable, "client not initialized")
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		RefreshToken string `json:"refresh_token"`
		Namespace    string `json:"namespace"`
		All          bool   `json:"all"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	ns := strings.TrimSpace(req.Namespace)
	if ns == "" {
		ns = strings.TrimSpace(g.cfg.ClientNamespace)
		if ns == "" { ns = "default" }
	}
	ctx := r.Context()
	db := g.client.Database()
	nsID, err := g.resolveNamespaceID(ctx, ns)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if strings.TrimSpace(req.RefreshToken) != "" {
		// Revoke specific token
		if _, err := db.Query(ctx, "UPDATE refresh_tokens SET revoked_at = datetime('now') WHERE namespace_id = ? AND token = ? AND revoked_at IS NULL", nsID, req.RefreshToken); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "revoked": 1})
		return
	}

	if req.All {
		// Require JWT to identify subject
		var subject string
		if v := ctx.Value(ctxKeyJWT); v != nil {
			if claims, ok := v.(*jwtClaims); ok && claims != nil {
				subject = strings.TrimSpace(claims.Sub)
			}
		}
		if subject == "" {
			writeError(w, http.StatusUnauthorized, "jwt required for all=true")
			return
		}
		if _, err := db.Query(ctx, "UPDATE refresh_tokens SET revoked_at = datetime('now') WHERE namespace_id = ? AND subject = ? AND revoked_at IS NULL", nsID, subject); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "revoked": "all"})
		return
	}

	writeError(w, http.StatusBadRequest, "nothing to revoke: provide refresh_token or all=true")
}
