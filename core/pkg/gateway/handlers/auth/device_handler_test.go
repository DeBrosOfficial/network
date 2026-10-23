package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	authsvc "github.com/Orama-Network/orama/core/pkg/gateway/auth"
)

// The polling client switches on the `error` member and nothing else, so the
// mapping from outcome to wire name is the contract. RFC 8628 §3.5 names all
// four, and a client that sees an unrecognised one has to guess.
func TestWriteDeviceError_saysWhatTheClientSwitchesOn(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
		code   string
	}{
		{authsvc.ErrDeviceAuthorizationPending, http.StatusBadRequest, "authorization_pending"},
		{authsvc.ErrDeviceSlowDown, http.StatusBadRequest, "slow_down"},
		{authsvc.ErrDeviceCodeExpired, http.StatusBadRequest, "expired_token"},
		{authsvc.ErrDeviceAccessDenied, http.StatusBadRequest, "access_denied"},
		{authsvc.ErrDeviceCodeUnknown, http.StatusBadRequest, "invalid_grant"},
		{authsvc.ErrDeviceAlreadyApproved, http.StatusConflict, "already_approved"},
	} {
		t.Run(tc.code, func(t *testing.T) {
			rec := httptest.NewRecorder()
			whoamiHandlers(t).writeDeviceError(rec, "collect the login", tc.err)

			if rec.Code != tc.status {
				t.Errorf("status = %d, want %d", rec.Code, tc.status)
			}
			var body map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode: %v (%s)", err, rec.Body.String())
			}
			if got, _ := body["error"].(string); got != tc.code {
				t.Errorf("error = %q, want %q", got, tc.code)
			}
			// One word is not enough for somebody reading a failed poll with
			// curl, and the description is the only place the next step goes.
			if desc, _ := body["error_description"].(string); strings.TrimSpace(desc) == "" {
				t.Error("no error_description")
			}
		})
	}
}

// A refusal the approver can act on says which, with its code; anything else is
// the gateway failing, answered 503 without the failure's own words. The claim
// path is open to whoever holds a device code, and a registry error names
// tables and namespaces.
func TestWriteDeviceError_classifiesRefusalsAndHidesFailures(t *testing.T) {
	_, malformed := authsvc.NormalizeUserCode("ABC")
	for _, tc := range []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"malformed user code", malformed, http.StatusBadRequest, ErrCodeDeviceUserCodeMalformed},
		{"wrong namespace", fmt.Errorf("%w: this login asked for namespace %q", authsvc.ErrDeviceLoginMismatch, "a"),
			http.StatusBadRequest, ErrCodeDeviceLoginMismatch},
		{"device link approved by a wallet", authsvc.ErrDeviceLinkNeedsDevice, http.StatusBadRequest, ErrCodeDeviceLoginMismatch},
		{"registry failure", errors.New(`failed to read the grants of namespace "x": rqlite: connection refused`),
			http.StatusServiceUnavailable, ""},
		{"no registry", errors.New("client not initialized"), http.StatusServiceUnavailable, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			whoamiHandlers(t).writeDeviceError(rec, "collect the login", tc.err)

			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d (%s)", rec.Code, tc.status, rec.Body.String())
			}
			var body map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode: %v (%s)", err, rec.Body.String())
			}
			if got, _ := body["code"].(string); got != tc.code {
				t.Errorf("code = %q, want %q", got, tc.code)
			}
			if tc.status == http.StatusServiceUnavailable && strings.Contains(rec.Body.String(), tc.err.Error()) {
				t.Errorf("the response carries the failure's own words: %s", rec.Body.String())
			}
		})
	}
}

// Every one of these takes a POST. A GET that fell through to the body decode
// would report "invalid json body" for what is actually the wrong method.
func TestDeviceHandlers_refuseAnythingButPost(t *testing.T) {
	h := whoamiHandlers(t)
	for name, handler := range map[string]http.HandlerFunc{
		"/v1/auth/device":         h.DeviceAuthorizationHandler,
		"/v1/auth/device/approve": h.DeviceApprovalHandler,
		"/v1/auth/device/token":   h.DeviceTokenHandler,
	} {
		t.Run(name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handler(rec, httptest.NewRequest(http.MethodGet, name, nil))
			if rec.Code != http.StatusMethodNotAllowed {
				t.Errorf("GET %s = %d, want %d", name, rec.Code, http.StatusMethodNotAllowed)
			}
		})
	}
}

// A code is the whole of what the approver is acting on, and approving an empty
// one would mean approving whichever pending login the query happened to find.
func TestDeviceApprovalHandler_requiresAUserCode(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/device/approve",
		strings.NewReader(`{"message":"m","signature":"s"}`))

	whoamiHandlers(t).DeviceApprovalHandler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rec.Body.String(), "user_code") {
		t.Errorf("the refusal does not name what is missing: %s", rec.Body.String())
	}
}

// Sessions belong to a wallet. An API key is a credential in its own right and
// names no wallet, so it must not reach the sessions of whoever minted it.
func TestSessionsHandler_refusesAnAPIKeyCredential(t *testing.T) {
	h := whoamiHandlers(t)

	for name, call := range map[string]func(http.ResponseWriter, *http.Request){
		"list": h.SessionsHandler,
		"end":  h.SessionByIDHandler,
	} {
		t.Run(name, func(t *testing.T) {
			method := http.MethodGet
			path := "/v1/auth/sessions"
			if name == "end" {
				method, path = http.MethodDelete, "/v1/auth/sessions/1"
			}
			req := httptest.NewRequest(method, path, nil)
			ctx := context.WithValue(req.Context(), CtxKeyAPIKey, "orama_sk_whatever_x")
			rec := httptest.NewRecorder()

			call(rec, req.WithContext(ctx))

			if rec.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
			}
			if !strings.Contains(rec.Body.String(), "orama auth login") {
				t.Errorf("the refusal does not say what to do instead: %s", rec.Body.String())
			}
		})
	}
}

func TestSessionByIDHandler_refusesAPathWithNoUsableID(t *testing.T) {
	h := whoamiHandlers(t)
	claims := &authsvc.JWTClaims{Sub: "0xowner", Namespace: "anchat"}

	for _, path := range []string{"/v1/auth/sessions/", "/v1/auth/sessions/all", "/v1/auth/sessions/0", "/v1/auth/sessions/-3"} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodDelete, path, nil)
			ctx := context.WithValue(req.Context(), CtxKeyJWT, claims)
			rec := httptest.NewRecorder()

			h.SessionByIDHandler(rec, req.WithContext(ctx))

			if rec.Code != http.StatusBadRequest {
				t.Errorf("DELETE %s = %d, want %d", path, rec.Code, http.StatusBadRequest)
			}
		})
	}
}
