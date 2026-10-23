package serverless

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Orama-Network/orama/core/pkg/gateway/auth"
	"github.com/Orama-Network/orama/core/pkg/gateway/ctxkeys"
	"github.com/Orama-Network/orama/core/pkg/httputil"
	"github.com/Orama-Network/orama/core/pkg/serverless"
	"go.uber.org/zap"
)

// An API key narrowed to five functions, as the gateway's scope middleware
// hands it to these handlers: invoke, narrowed by the key's resources.
var narrowedKeyFunctions = []string{
	"subscription-read",
	"subscription-intent-create",
	"subscription-record-payment",
	"user-find-by-username",
	"user-find-by-wallet",
}

func narrowedKeyContext() map[any]any {
	resources := auth.KeyResources{}
	for _, fn := range narrowedKeyFunctions {
		resources = append(resources, "fn:"+fn)
	}
	return map[any]any{
		ctxkeys.Scopes:       auth.ScopeSet{auth.ScopeInvoke: {}},
		ctxkeys.KeyResources: resources,
		ctxkeys.Permissions:  resources.Narrow(auth.PermissionsFromScopes(auth.ScopeInvoke)),
	}
}

// invokeAs invokes a function the registry does not hold: a call the key's
// resources admit gets as far as looking it up (404), and one they refuse
// never does.
func invokeAs(t *testing.T, fn string, ctxValues map[any]any) *httptest.ResponseRecorder {
	t.Helper()
	reg := newMockRegistry()
	h := newTestHandlers(reg)
	h.invoker = serverless.NewInvoker(nil, reg, nil, "acme", zap.NewNop())
	r := asCredentialOf(httptest.NewRequest(http.MethodPost, "/v1/invoke/acme/"+fn, nil), "acme")
	ctx := r.Context()
	for k, v := range ctxValues {
		ctx = context.WithValue(ctx, k, v)
	}
	rec := httptest.NewRecorder()
	h.HandleInvoke(rec, r.WithContext(ctx))
	return rec
}

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("not an RPC error envelope: %s", rec.Body.String())
	}
	return body.Error.Code
}

// Each of the five runs as for any invoke key; any other function is refused
// with the refusal a wallet narrowed to one function gets: 403 FORBIDDEN,
// naming the function.
func TestInvoke_aKeyNarrowedToFiveFunctions(t *testing.T) {
	for _, fn := range narrowedKeyFunctions {
		if rec := invokeAs(t, fn, narrowedKeyContext()); rec.Code != http.StatusNotFound {
			t.Errorf("%s: %d %s, want it admitted as far as the lookup (404)", fn, rec.Code, rec.Body.String())
		}
	}

	walletNarrowed := map[any]any{ctxkeys.Permissions: auth.PermissionsFor(auth.RoleRuntime, "fn:name=checkout")}
	for _, fn := range []string{"user-delete", "subscription-read-all", "admin-reset"} {
		rec := invokeAs(t, fn, narrowedKeyContext())
		if rec.Code != http.StatusForbidden || errorCode(t, rec) != string(httputil.ErrCodeForbidden) {
			t.Errorf("%s: %d %s, want 403 FORBIDDEN", fn, rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "does not cover fn:") || !strings.Contains(rec.Body.String(), ":"+fn) {
			t.Errorf("%s: the refusal does not name the function it refused: %s", fn, rec.Body.String())
		}
		wallet := invokeAs(t, fn, walletNarrowed)
		if wallet.Code != rec.Code || errorCode(t, wallet) != errorCode(t, rec) {
			t.Errorf("%s: a narrowed key is refused %d %s and a narrowed wallet %d %s; they are one decision",
				fn, rec.Code, errorCode(t, rec), wallet.Code, errorCode(t, wallet))
		}
	}
}

// The function WebSocket is an invocation too, refused before the upgrade.
func TestHandleWebSocket_aKeyNarrowedToFiveFunctions(t *testing.T) {
	for _, fn := range narrowedKeyFunctions {
		h := handlersWith(persistentFn(fn, false, false))
		rec := httptest.NewRecorder()
		h.HandleWebSocket(rec, upgradeRequest(narrowedKeyContext()), fn, 0)
		if rec.Code == http.StatusForbidden {
			t.Errorf("%s: refused (%s)", fn, strings.TrimSpace(rec.Body.String()))
		}
	}
	h := handlersWith(persistentFn("user-delete", false, false))
	rec := httptest.NewRecorder()
	h.HandleWebSocket(rec, upgradeRequest(narrowedKeyContext()), "user-delete", 0)
	if rec.Code != http.StatusForbidden || errorCode(t, rec) != string(httputil.ErrCodeForbidden) {
		t.Errorf("user-delete: %d %s, want 403 FORBIDDEN", rec.Code, rec.Body.String())
	}
}
