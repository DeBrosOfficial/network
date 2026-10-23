package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Orama-Network/orama/core/pkg/fleetattest"
	"github.com/Orama-Network/orama/e2e/harness/config"
	"github.com/Orama-Network/orama/e2e/harness/fleet"
	"github.com/Orama-Network/orama/e2e/harness/notify"
	"github.com/Orama-Network/orama/e2e/harness/provision"
	"github.com/Orama-Network/orama/e2e/harness/report"
)

// fakeSender records the pushes.
type fakeSender struct {
	sent []notify.Message
	err  error
}

func (f *fakeSender) Send(_ context.Context, m notify.Message) error {
	f.sent = append(f.sent, m)
	return f.err
}

func runnerPEM(t *testing.T) (string, ed25519.PublicKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})), pub
}

func lookupOf(env map[string]string) func(string) (string, bool) {
	return func(k string) (string, bool) { v, ok := env[k]; return v, ok }
}

func TestLoadOutcome(t *testing.T) {
	key, _ := runnerPEM(t)
	o, err := loadOutcome(lookupOf(nil))
	if err != nil || o.signer != nil || o.notifier != nil {
		t.Fatalf("unset: %+v, %v", o, err)
	}
	o, err = loadOutcome(lookupOf(map[string]string{config.EnvRunnerSigningKey: key, notify.EnvURL: "https://push.example.org/e2e"}))
	if err != nil || o.signer == nil || o.notifier == nil {
		t.Fatalf("set: %+v, %v", o, err)
	}
	_, err = loadOutcome(lookupOf(map[string]string{config.EnvRunnerSigningKey: "not a key", notify.EnvURL: "http://push.example.org/e2e"}))
	if err == nil || !strings.Contains(err.Error(), config.EnvRunnerSigningKey) || !strings.Contains(err.Error(), notify.EnvURL) {
		t.Fatalf("unusable secrets: %v", err)
	}
}

func TestAttest_signsAReportTheReleaseGateAccepts(t *testing.T) {
	key, pub := runnerPEM(t)
	o, err := loadOutcome(lookupOf(map[string]string{config.EnvRunnerSigningKey: key}))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	rep := report.Report{RunID: "ab12", Commit: strings.Repeat("c", 40), Verdict: report.VerdictPass}
	if err := report.WriteAll(dir, rep, false); err != nil {
		t.Fatal(err)
	}
	if err := o.attest(t.Context(), dir, rep, time.Now()); err != nil {
		t.Fatal(err)
	}
	s, err := fleetattest.Verify(t.Context(), dir, []ed25519.PublicKey{pub})
	if err != nil || s.RunID != "ab12" || s.Commit != rep.Commit || s.Verdict != report.VerdictPass {
		t.Fatalf("%+v, %v", s, err)
	}
}

func TestAttest_withoutAKeyLeavesTheReportUnsigned(t *testing.T) {
	dir := t.TempDir()
	if err := (outcome{}).attest(t.Context(), dir, report.Report{}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, fleetattest.FileName)); !os.IsNotExist(err) {
		t.Fatalf("stat: %v", err)
	}
}

func TestMessage_carriesTheVerdict(t *testing.T) {
	rep := &report.Report{Notification: "Orama e2e ab12 @ c: FAIL — 1 failed"}
	for _, tc := range []struct {
		code     int
		rep      *report.Report
		title    string
		priority int
		tag      string
		body     string
	}{
		{exitOK, rep, "Orama e2e PASS", notify.PriorityDefault, "white_check_mark", rep.Notification},
		{exitIncomplete, rep, "Orama e2e INCOMPLETE", notify.PriorityDefault, "warning", rep.Notification},
		{exitFail, rep, "Orama e2e FAIL", notify.PriorityHigh, "x", rep.Notification},
		{exitFail, nil, "Orama e2e FAIL", notify.PriorityHigh, "x", "the run wrote no report"},
	} {
		m := message(tc.rep, "ab12", "c0ffee", tc.code)
		if m.Title != tc.title || m.Priority != tc.priority || m.Tags[0] != tc.tag || !strings.Contains(m.Body, tc.body) {
			t.Errorf("code %d: %+v", tc.code, m)
		}
	}
}

// TestRun_pushesOnceAfterTheTeardown: the push carries the final verdict,
// teardown failure included, and is sent once.
func TestRun_pushesOnceAfterTheTeardown(t *testing.T) {
	fakeProvisioner(t, func() (*fleet.State, error) {
		return nil, &provision.UpError{Err: errors.New("hetzner quota"), Owned: true}
	}, errors.New("2 servers left"))
	rs := testRunState(t)
	push := &fakeSender{}
	rs.out.notifier = push
	code, _ := rs.run(context.Background(), context.Background(), false)
	if code != exitFail || len(push.sent) != 1 {
		t.Fatalf("code %d, pushes %+v", code, push.sent)
	}
	if m := push.sent[0]; m.Title != "Orama e2e FAIL" || !strings.Contains(m.Body, "FAIL") {
		t.Fatalf("push %+v", m)
	}
}

// TestRun_aFailedPushFailsTheRun: the owner must learn the push did not arrive.
func TestRun_aFailedPushFailsTheRun(t *testing.T) {
	fakeProvisioner(t, func() (*fleet.State, error) {
		return nil, &provision.UpError{Err: errors.New("hetzner quota"), Owned: false}
	}, nil)
	rs := testRunState(t)
	rs.out.notifier = &fakeSender{err: errors.New("ntfy refused the push: HTTP 429")}
	code, err := rs.run(context.Background(), context.Background(), false)
	if code != exitFail || err == nil || !strings.Contains(err.Error(), "notification failed") {
		t.Fatalf("code %d err %v", code, err)
	}
}
