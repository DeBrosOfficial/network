package attest

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	verdict "github.com/Orama-Network/orama/core/pkg/attest/refusal"
	authsvc "github.com/Orama-Network/orama/core/pkg/gateway/auth"
	"github.com/Orama-Network/orama/core/pkg/gateway/ctxkeys"
)

// fakeService records what the handlers asked and answers what the test set.
type fakeService struct {
	attestErr error
	policy    authsvc.AttestationPolicy
	setErr    error

	gotChallenge string
	gotChain     [][]byte
	gotDevice    string
	setPolicy    *authsvc.AttestationPolicy
}

func (f *fakeService) IssueAttestationChallenge(_ context.Context, _, device string) (string, time.Time, error) {
	f.gotDevice = device
	return "the-challenge", time.Now().Add(authsvc.AttestationChallengeTTL), nil
}

func (f *fakeService) AttestIOS(_ context.Context, _, device, challenge string, _, _ []byte) (*authsvc.AttestationClaim, error) {
	f.gotDevice, f.gotChallenge = device, challenge
	if f.attestErr != nil {
		return nil, f.attestErr
	}
	return &authsvc.AttestationClaim{Platform: "ios", AppID: "TEAM.chat.anchat", AttestedAt: time.Now().Unix()}, nil
}

func (f *fakeService) AttestAndroid(_ context.Context, _, device, challenge string, chain [][]byte) (*authsvc.AttestationClaim, error) {
	f.gotDevice, f.gotChallenge, f.gotChain = device, challenge, chain
	if f.attestErr != nil {
		return nil, f.attestErr
	}
	return &authsvc.AttestationClaim{Platform: "android", AppID: "chat.anchat", AppVersion: "412", AttestedAt: time.Now().Unix()}, nil
}

func (f *fakeService) ReissueAttested(*authsvc.JWTClaims, *authsvc.AttestationClaim) (string, int64, error) {
	return "attested.token.here", time.Now().Add(15 * time.Minute).Unix(), nil
}

func (f *fakeService) AttestationPolicyOf(context.Context, string) (authsvc.AttestationPolicy, error) {
	if f.setPolicy != nil {
		return *f.setPolicy, nil
	}
	return f.policy, nil
}

func (f *fakeService) SetAttestationPolicy(_ context.Context, _ string, p authsvc.AttestationPolicy, _ string) error {
	if f.setErr != nil {
		return f.setErr
	}
	f.setPolicy = &p
	return nil
}

type recordedAudit struct{ events []authsvc.AuditEvent }

func (a *recordedAudit) RecordFromRequest(_ context.Context, _ *http.Request, e authsvc.AuditEvent) {
	a.events = append(a.events, e)
}

const device = "kPrK_qmxVWaYVA9wwBF6Iuo3vVzz7TxHCTwXBygrS4k"

func call(h http.HandlerFunc, method, path string, claims *authsvc.JWTClaims, body any) (*httptest.ResponseRecorder, map[string]any) {
	raw, _ := json.Marshal(body)
	r := httptest.NewRequest(method, path, bytes.NewReader(raw))
	ctx := context.WithValue(r.Context(), ctxkeys.NamespaceOverride, "anchat")
	if claims != nil {
		ctx = context.WithValue(ctx, ctxkeys.JWT, claims)
	}
	rec := httptest.NewRecorder()
	h(rec, r.WithContext(ctx))
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec, out
}

func deviceSession() *authsvc.JWTClaims {
	return &authsvc.JWTClaims{Sub: "0x00000000000000000000000000000000000000aa", Namespace: "anchat", Did: device}
}

// A challenge is for the session's device; a key or a session with no device
// gets none.
func TestChallengeHandler(t *testing.T) {
	svc := &fakeService{}
	h := NewHandlers(svc, nil, nil)
	rec, body := call(h.ChallengeHandler, http.MethodPost, "/v1/attest/challenge", deviceSession(), nil)
	if rec.Code != http.StatusOK || body["challenge"] != "the-challenge" || body["device_id"] != device || svc.gotDevice != device {
		t.Errorf("%d %v", rec.Code, body)
	}
	if rec, body := call(h.ChallengeHandler, http.MethodPost, "/v1/attest/challenge",
		&authsvc.JWTClaims{Sub: "0x00000000000000000000000000000000000000aa", Namespace: "anchat"}, nil); rec.Code != http.StatusForbidden || body["code"] != CodeAttestationNeedsDevice {
		t.Errorf("a session with no device: %d %v", rec.Code, body)
	}
	if rec, _ := call(h.ChallengeHandler, http.MethodPost, "/v1/attest/challenge", nil, nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("no session: %d", rec.Code)
	}
	if rec, _ := call(h.ChallengeHandler, http.MethodGet, "/v1/attest/challenge", deviceSession(), nil); rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET: %d", rec.Code)
	}
}

// An accepted proof answers a token carrying it; a refused one answers its
// name and nothing about which check refused it.
func TestAndroidHandler(t *testing.T) {
	svc := &fakeService{}
	h := NewHandlers(svc, nil, nil)
	chain := []string{base64.StdEncoding.EncodeToString([]byte("leaf")), base64.StdEncoding.EncodeToString([]byte("root"))}
	rec, body := call(h.AndroidHandler, http.MethodPost, "/v1/attest/android", deviceSession(),
		AndroidRequest{Challenge: "c", Chain: chain})
	if rec.Code != http.StatusOK || body["access_token"] != "attested.token.here" {
		t.Fatalf("%d %v", rec.Code, body)
	}
	if att, _ := body["attestation"].(map[string]any); att["platform"] != "android" || att["app_version"] != "412" {
		t.Errorf("attestation %v", body["attestation"])
	}
	if len(svc.gotChain) != 2 || string(svc.gotChain[0]) != "leaf" || svc.gotChallenge != "c" || svc.gotDevice != device {
		t.Errorf("the service was asked %q %q %q", svc.gotChain, svc.gotChallenge, svc.gotDevice)
	}

	svc.attestErr = verdict.Invalid(verdict.ReasonSigningCert, "the APK was re-signed with key X")
	rec, body = call(h.AndroidHandler, http.MethodPost, "/v1/attest/android", deviceSession(), AndroidRequest{Challenge: "c", Chain: chain})
	if rec.Code != http.StatusForbidden || body["code"] != CodeAttestationInvalid {
		t.Errorf("a re-signed APK: %d %v", rec.Code, body)
	}
	if msg, _ := body["error"].(string); bytes.Contains([]byte(msg), []byte("key X")) {
		t.Errorf("the refusal tells a forger which check failed: %q", msg)
	}

	svc.attestErr = verdict.UpdateRequired("versionCode 3 < 400")
	if rec, body := call(h.AndroidHandler, http.MethodPost, "/v1/attest/android", deviceSession(), AndroidRequest{Challenge: "c", Chain: chain}); body["code"] != CodeAppUpdateRequired {
		t.Errorf("an old build: %d %v", rec.Code, body)
	}
	svc.attestErr = authsvc.ErrNonceInvalid
	if rec, body := call(h.AndroidHandler, http.MethodPost, "/v1/attest/android", deviceSession(), AndroidRequest{Challenge: "c", Chain: chain}); rec.Code != http.StatusUnauthorized || body["code"] != CodeAttestationChallengeInvalid {
		t.Errorf("a spent challenge: %d %v", rec.Code, body)
	}
	svc.attestErr = errors.New("registry down")
	if rec, _ := call(h.AndroidHandler, http.MethodPost, "/v1/attest/android", deviceSession(), AndroidRequest{Challenge: "c", Chain: chain}); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("the server's own failure: %d", rec.Code)
	}
	for name, req := range map[string]AndroidRequest{
		"no chain":     {Challenge: "c"},
		"not base64":   {Challenge: "c", Chain: []string{"%%%"}},
		"too long":     {Challenge: "c", Chain: make([]string, maxChainLength+1)},
		"no challenge": {Chain: chain},
	} {
		if rec, body := call(h.AndroidHandler, http.MethodPost, "/v1/attest/android", deviceSession(), req); rec.Code != http.StatusBadRequest || body["code"] != CodeAttestationMalformed {
			t.Errorf("%s: %d %v", name, rec.Code, body)
		}
	}
}

func TestIOSHandler(t *testing.T) {
	svc := &fakeService{}
	h := NewHandlers(svc, nil, nil)
	b64 := base64.StdEncoding.EncodeToString
	rec, body := call(h.IOSHandler, http.MethodPost, "/v1/attest/ios", deviceSession(),
		IOSRequest{Challenge: "c", KeyID: b64([]byte("key")), Attestation: b64([]byte("object"))})
	if rec.Code != http.StatusOK || body["access_token"] == nil {
		t.Errorf("%d %v", rec.Code, body)
	}
	if rec, _ := call(h.IOSHandler, http.MethodPost, "/v1/attest/ios", deviceSession(), IOSRequest{Challenge: "c"}); rec.Code != http.StatusBadRequest {
		t.Errorf("no key or attestation: %d", rec.Code)
	}
}

// A PUT changes only what it names, is validated whole, refuses unknown
// fields and is audited; the lobby has no policy.
func TestPolicyHandler(t *testing.T) {
	svc := &fakeService{policy: authsvc.AttestationPolicy{Mode: authsvc.AttestationMonitor,
		Android:          authsvc.AndroidAttestation{Package: "chat.anchat", SigningCertDigests: []string{"ab"}},
		ReattestInterval: authsvc.DefaultReattestInterval}}
	audit := &recordedAudit{}
	h := NewHandlers(svc, audit, nil)

	rec, body := call(h.PolicyHandler, http.MethodGet, "/v1/namespace/attestation-policy", nil, nil)
	if rec.Code != http.StatusOK || body["mode"] != "monitor" || body["reattest_interval"] != "168h0m0s" || body["namespace"] != "anchat" {
		t.Fatalf("GET: %d %v", rec.Code, body)
	}

	rec, body = call(h.PolicyHandler, http.MethodPut, "/v1/namespace/attestation-policy", nil,
		map[string]any{"mode": "enforce", "android": map[string]any{"allow_grapheneos": true}, "reattest_interval": "72h"})
	if rec.Code != http.StatusOK || svc.setPolicy == nil {
		t.Fatalf("PUT: %d %v", rec.Code, body)
	}
	got := *svc.setPolicy
	if got.Mode != authsvc.AttestationEnforce || got.Android.Package != "chat.anchat" || !got.Android.AllowGrapheneOS || got.ReattestInterval != 72*time.Hour {
		t.Errorf("stored %+v", got)
	}
	if len(audit.events) != 1 || audit.events[0].Action != authsvc.AuditAttestationPolicySet || audit.events[0].Resource != "enforce" {
		t.Errorf("audit %+v", audit.events)
	}

	if rec, _ := call(h.PolicyHandler, http.MethodPut, "/v1/namespace/attestation-policy", nil, map[string]any{"mood": "enforce"}); rec.Code != http.StatusBadRequest {
		t.Errorf("an unknown field: %d", rec.Code)
	}
	if rec, _ := call(h.PolicyHandler, http.MethodPut, "/v1/namespace/attestation-policy", nil, map[string]any{"reattest_interval": "a week"}); rec.Code != http.StatusBadRequest {
		t.Errorf("a bad interval: %d", rec.Code)
	}
	svc.setErr = authsvc.ErrInvalidAttestationPolicy
	if rec, body := call(h.PolicyHandler, http.MethodPut, "/v1/namespace/attestation-policy", nil, map[string]any{"mode": "enforce"}); rec.Code != http.StatusBadRequest || body["code"] != CodeAttestationPolicyInvalid {
		t.Errorf("an invalid policy: %d %v", rec.Code, body)
	}

	r := httptest.NewRequest(http.MethodGet, "/v1/namespace/attestation-policy", nil)
	r = r.WithContext(context.WithValue(r.Context(), ctxkeys.NamespaceOverride, authsvc.LobbyNamespace))
	lobby := httptest.NewRecorder()
	h.PolicyHandler(lobby, r)
	if lobby.Code != http.StatusForbidden {
		t.Errorf("the lobby: %d", lobby.Code)
	}
}
