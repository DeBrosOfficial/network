package webrtc

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

const (
	testWallet  = "0xwallet-of-alice"
	testAccount = "acct-alice"
)

// signalAsAccount sends a ?room= join for a session whose token names account.
func signalAsAccount(h *WebRTCHandlers, room, sub, account, device string) (*http.Request, *httptest.ResponseRecorder) {
	req := requestWithNamespace("GET", "/v1/webrtc/signal?"+url.Values{"room": {room}}.Encode(), "ns")
	req = asAccountCaller(req, sub, account, device)
	w := httptest.NewRecorder()
	h.SignalHandler(w, req)
	return req, w
}

func TestSignalHandler_ticketNamesTheNamespacesAccountNotTheWallet(t *testing.T) {
	_, dir := threeSFUs(t)
	var target string
	h := gatewayFor(t, dir, &target)

	req, w := signalAsAccount(h, "r1", testWallet, testAccount, "phone")

	if w.Code != http.StatusOK {
		t.Fatalf("status %d %s", w.Code, w.Body)
	}
	tk := openTicketOf(t, req)
	if tk.UserID != testAccount || tk.DeviceID != "phone" {
		t.Fatalf("ticket names user %q device %q, want %q on %q: the SFU shows the ticket's user to every peer",
			tk.UserID, tk.DeviceID, testAccount, "phone")
	}
}

func TestSignalHandler_admissionIsKeyedOnTheAccount(t *testing.T) {
	_, dir := threeSFUs(t)
	var target string
	h := gatewayFor(t, dir, &target)
	ctx := httptest.NewRequest("GET", "/", nil).Context()
	if err := h.admissions.SetRequireAdmission(ctx, "ns", true); err != nil {
		t.Fatal(err)
	}
	if _, err := h.admissions.Admit(ctx, "ns", "account-room", testAccount, "", time.Hour); err != nil {
		t.Fatal(err)
	}
	if _, err := h.admissions.Admit(ctx, "ns", "wallet-room", testWallet, "", time.Hour); err != nil {
		t.Fatal(err)
	}

	if _, w := signalAsAccount(h, "account-room", testWallet, testAccount, ""); w.Code != http.StatusOK {
		t.Fatalf("admitted by account: status %d %s", w.Code, w.Body)
	}
	_, w := signalAsAccount(h, "wallet-room", testWallet, testAccount, "")
	if w.Code != http.StatusForbidden || rpcCode(t, w) != codeAdmissionRequired {
		t.Fatalf("admitted only by wallet: status %d %s, want %s: an account-named session is not its wallet",
			w.Code, w.Body, codeAdmissionRequired)
	}
}

func TestSignalHandler_sessionWithoutAnAccountKeepsItsSubject(t *testing.T) {
	_, dir := threeSFUs(t)
	var target string
	h := gatewayFor(t, dir, &target)

	req, w := signalAsAccount(h, "r1", testWallet, "", "")

	if w.Code != http.StatusOK {
		t.Fatalf("status %d %s", w.Code, w.Body)
	}
	if tk := openTicketOf(t, req); tk.UserID != testWallet {
		t.Fatalf("ticket user = %q, want the subject %q", tk.UserID, testWallet)
	}
}
