package operator

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Orama-Network/orama/core/pkg/gateway/auth"
	"github.com/Orama-Network/orama/core/pkg/gateway/ctxkeys"
)

func TestWalletFromRequest_withClaims(t *testing.T) {
	h := NewHandler(nil, nil)
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	claims := &auth.JWTClaims{Sub: "0xabc123"}
	ctx := context.WithValue(r.Context(), ctxkeys.JWT, claims)
	r = r.WithContext(ctx)

	wallet := h.walletFromRequest(r)
	if wallet != "0xabc123" {
		t.Errorf("wallet = %q, want %q", wallet, "0xabc123")
	}
}

func TestWalletFromRequest_noClaims(t *testing.T) {
	h := NewHandler(nil, nil)
	r := httptest.NewRequest(http.MethodGet, "/", nil)

	wallet := h.walletFromRequest(r)
	if wallet != "" {
		t.Errorf("wallet = %q, want empty", wallet)
	}
}

func TestWalletFromRequest_nilClaims(t *testing.T) {
	h := NewHandler(nil, nil)
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := context.WithValue(r.Context(), ctxkeys.JWT, (*auth.JWTClaims)(nil))
	r = r.WithContext(ctx)

	wallet := h.walletFromRequest(r)
	if wallet != "" {
		t.Errorf("wallet = %q, want empty", wallet)
	}
}

// A key is a namespace's credential, never an operator's, whoever owns the
// namespace (bugboard bug-55). Presented bare, or exchanged for a token that
// names its namespace, it resolves to no wallet at all, so it can never match
// the operator list; nor does a deployed app's workload token.
func TestWalletFromRequest_aKeyOrAnAppIsNeverAnOperatorWallet(t *testing.T) {
	bare := httptest.NewRequest(http.MethodGet, "/", nil)
	bare = bare.WithContext(context.WithValue(context.WithValue(bare.Context(),
		ctxkeys.APIKey, "orama_sk_abc_def"), ctxkeys.NamespaceOverride, "operatorsns"))
	if got := WalletFromRequest(bare); got != "" {
		t.Errorf("a bare API key resolved to wallet %q", got)
	}
	for _, sub := range []string{"orama_sk_abc_def", "app:operatorsns/web"} {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r = r.WithContext(context.WithValue(r.Context(), ctxkeys.JWT,
			&auth.JWTClaims{Sub: sub, Namespace: "operatorsns"}))
		if got := WalletFromRequest(r); got != "" {
			t.Errorf("a token with subject %q resolved to wallet %q", sub, got)
		}
	}
}

func TestDecodeJSON_valid(t *testing.T) {
	body := strings.NewReader(`{"node_id":"test-node","environment":"devnet"}`)
	r := httptest.NewRequest(http.MethodPost, "/", body)

	var req RegisterRequest
	if err := decodeJSON(r, &req); err != nil {
		t.Fatalf("decodeJSON: %v", err)
	}
	if req.NodeID != "test-node" {
		t.Errorf("NodeID = %q, want %q", req.NodeID, "test-node")
	}
	if req.Environment != "devnet" {
		t.Errorf("Environment = %q, want %q", req.Environment, "devnet")
	}
}

func TestDecodeJSON_invalid(t *testing.T) {
	body := strings.NewReader(`not-json`)
	r := httptest.NewRequest(http.MethodPost, "/", body)

	var req RegisterRequest
	if err := decodeJSON(r, &req); err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestHandleInvite_noAuth(t *testing.T) {
	h := NewHandler(nil, nil)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/v1/operator/invite", nil)

	h.HandleInvite(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestHandleInvite_wrongMethod(t *testing.T) {
	h := NewHandler(nil, nil)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/v1/operator/invite", nil)

	h.HandleInvite(w, r)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandleListNodes_noAuth(t *testing.T) {
	h := NewHandler(nil, nil)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/v1/operator/nodes", nil)

	h.HandleListNodes(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestHandleListNodes_wrongMethod(t *testing.T) {
	h := NewHandler(nil, nil)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/v1/operator/nodes", nil)

	h.HandleListNodes(w, r)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandleRegister_noAuth(t *testing.T) {
	h := NewHandler(nil, nil)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/v1/operator/node/register", strings.NewReader(`{"node_id":"test"}`))

	h.HandleRegister(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestHandleRegister_missingFields(t *testing.T) {
	// Authorization runs first now, so the caller has to be an operator before
	// the request body is looked at: an unauthorized caller should not learn
	// which inputs are valid.
	h, _ := operatorHandler(t, "0xabc")
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/v1/operator/node/register", strings.NewReader(`{}`))
	claims := &auth.JWTClaims{Sub: "0xabc"}
	r = r.WithContext(context.WithValue(r.Context(), ctxkeys.JWT, claims))

	h.HandleRegister(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleRegister_invalidEnvironment(t *testing.T) {
	h, _ := operatorHandler(t, "0xabc")
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/v1/operator/node/register",
		strings.NewReader(`{"node_id":"test","environment":"<script>alert(1)</script>"}`))
	claims := &auth.JWTClaims{Sub: "0xabc"}
	r = r.WithContext(context.WithValue(r.Context(), ctxkeys.JWT, claims))

	h.HandleRegister(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleRegister_invalidRole(t *testing.T) {
	h, _ := operatorHandler(t, "0xabc")
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/v1/operator/node/register",
		strings.NewReader(`{"node_id":"test","role":"admin"}`))
	claims := &auth.JWTClaims{Sub: "0xabc"}
	r = r.WithContext(context.WithValue(r.Context(), ctxkeys.JWT, claims))

	h.HandleRegister(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestAllowedEnvironments(t *testing.T) {
	valid := []string{"devnet", "testnet", "production", "local"}
	invalid := []string{"-x", "<script>", "", "has space", "a.b"}

	for _, env := range valid {
		if !validEnvironment(env) {
			t.Errorf("expected %q to be allowed", env)
		}
	}
	for _, env := range invalid {
		if validEnvironment(env) {
			t.Errorf("expected %q to be disallowed", env)
		}
	}
}

func TestAllowedRoles(t *testing.T) {
	valid := []string{"node", "nameserver", "nameserver-ns1", "nameserver-ns4"}
	invalid := []string{"admin", "root", "", "nameserver-ns0", "nameserver-ns01", "-x"}

	for _, role := range valid {
		if !validRole(role) {
			t.Errorf("expected %q to be allowed", role)
		}
	}
	for _, role := range invalid {
		if validRole(role) {
			t.Errorf("expected %q to be disallowed", role)
		}
	}
}
