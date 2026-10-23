package serverless

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Orama-Network/orama/core/pkg/gateway/auth"
	"github.com/Orama-Network/orama/core/pkg/gateway/ctxkeys"
)

// bug-934: a caller with no wallet is named to the invoke rate limiter by its
// credential, not by the namespace getWalletFromRequest answers for it.
func TestGetCallerKeyFromRequest(t *testing.T) {
	h := newTestHandlers(nil)
	const wallet = "0xbBbBBBBbbBBBbbbBbbBbbbbBBbBbbbbBbBbbBBbB"
	cases := []struct {
		name   string
		values map[ctxkeys.ContextKey]any
		want   string
	}{
		{"a bare API key, by the fingerprint its lookup set", map[ctxkeys.ContextKey]any{
			ctxkeys.APIKey: "orama_live_secret", ctxkeys.KeyFingerprint: "key_0123456789ab", ctxkeys.NamespaceOverride: "anchat",
		}, "key_0123456789ab"},
		{"an exchanged key's token, by its subject", map[ctxkeys.ContextKey]any{
			ctxkeys.JWT: &auth.JWTClaims{Sub: "ak_stored_form"}, ctxkeys.NamespaceOverride: "anchat",
		}, auth.KeyFingerprint("ak_stored_form")},
		{"a workload's token, by its subject", map[ctxkeys.ContextKey]any{
			ctxkeys.JWT: &auth.JWTClaims{Sub: "app:anchat/web"}, ctxkeys.NamespaceOverride: "anchat",
		}, auth.KeyFingerprint("app:anchat/web")},
		{"a wallet, which has a bucket of its own", map[ctxkeys.ContextKey]any{
			ctxkeys.JWT: &auth.JWTClaims{Sub: wallet}, ctxkeys.KeyFingerprint: "key_0123456789ab",
		}, ""},
		{"no credential, bucketed by address", nil, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			for k, v := range tc.values {
				ctx = context.WithValue(ctx, k, v)
			}
			r := httptest.NewRequest(http.MethodPost, "/v1/invoke/anchat/rpc", nil).WithContext(ctx)
			if got := h.getCallerKeyFromRequest(r); got != tc.want {
				t.Errorf("caller key = %q, want %q", got, tc.want)
			}
		})
	}
}

// A bare key and the same key exchanged for a token are one caller: the
// lookup fingerprints the stored form, which is the exchanged token's subject.
func TestGetCallerKeyFromRequest_bareAndExchangedKeyAgree(t *testing.T) {
	h := newTestHandlers(nil)
	const stored = "5f2b0c9e0f1d4c3a8b7e6d5c4b3a29181716151413121110f0e0d0c0b0a09080"
	bare := httptest.NewRequest(http.MethodPost, "/", nil).WithContext(
		context.WithValue(context.Background(), ctxkeys.KeyFingerprint, auth.KeyFingerprint(stored)))
	exchanged := httptest.NewRequest(http.MethodPost, "/", nil).WithContext(
		context.WithValue(context.Background(), ctxkeys.JWT, &auth.JWTClaims{Sub: stored}))
	if a, b := h.getCallerKeyFromRequest(bare), h.getCallerKeyFromRequest(exchanged); a != b || a == "" {
		t.Errorf("bare %q and exchanged %q name different callers", a, b)
	}
}
