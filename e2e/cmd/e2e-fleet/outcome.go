package main

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"time"

	"github.com/Orama-Network/orama/core/pkg/fleetattest"
	"github.com/Orama-Network/orama/e2e/harness/config"
	"github.com/Orama-Network/orama/e2e/harness/notify"
	"github.com/Orama-Network/orama/e2e/harness/report"
)

// notifyBudget bounds the completion push.
const notifyBudget = time.Minute

// outcome is what a run does with its report beyond writing it: sign it with
// the runner key, so a release can stand on it, and push the verdict. Either
// is off when its secret is not set; a set secret that cannot be used refuses
// the run before anything is provisioned.
type outcome struct {
	signer   ed25519.PrivateKey
	notifier sender
}

// sender is *notify.Ntfy; tests replace it.
type sender interface {
	Send(ctx context.Context, m notify.Message) error
}

// loadOutcome reads the runner key and the ntfy topic through lookup
// (secrets.LookupEnv).
func loadOutcome(lookup func(string) (string, bool)) (outcome, error) {
	var o outcome
	var errs []error
	if pemKey, ok := lookup(config.EnvRunnerSigningKey); ok && pemKey != "" {
		key, err := fleetattest.ParsePrivateKey([]byte(pemKey))
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", config.EnvRunnerSigningKey, err))
		} else {
			o.signer = key
		}
	}
	if topic, ok := lookup(notify.EnvURL); ok && topic != "" {
		n, err := notify.New(topic)
		if err != nil {
			errs = append(errs, err)
		} else {
			o.notifier = n
		}
	}
	return o, errors.Join(errs...)
}

// attest signs the report just written in dir. Without a runner key the
// report is left unsigned, and no release can stand on it.
func (o outcome) attest(ctx context.Context, dir string, rep report.Report, now time.Time) error {
	if o.signer == nil {
		fmt.Printf("report not signed (%s is not set): no release can stand on this run\n", config.EnvRunnerSigningKey)
		return nil
	}
	s := fleetattest.Statement{RunID: rep.RunID, Commit: rep.Commit, Verdict: rep.Verdict, SignedAt: now.UTC()}
	if err := fleetattest.Sign(ctx, dir, o.signer, s); err != nil {
		return fmt.Errorf("sign the report: %w", err)
	}
	fmt.Println("report signed:", fleetattest.FileName)
	return nil
}

// push sends the verdict of a finished run: its report's notification line
// when it wrote one, or the run's failure when it could not.
func (o outcome) push(ctx context.Context, rep *report.Report, runID, commit string, code int) error {
	if o.notifier == nil {
		fmt.Printf("no push sent (%s is not set)\n", notify.EnvURL)
		return nil
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), notifyBudget)
	defer cancel()
	return o.notifier.Send(ctx, message(rep, runID, commit, code))
}

// message is the push for a run that ended with exit code code.
func message(rep *report.Report, runID, commit string, code int) notify.Message {
	verdict := verdictOf(code)
	body := fmt.Sprintf("Orama e2e %s @ %s: %s — the run wrote no report; see the runner's output", runID, commit, verdict)
	if rep != nil {
		body = rep.Notification
	}
	m := notify.Message{Title: "Orama e2e " + verdict, Body: body, Priority: notify.PriorityDefault, Tags: []string{"white_check_mark"}}
	switch verdict {
	case report.VerdictFail:
		m.Priority, m.Tags = notify.PriorityHigh, []string{"x"}
	case report.VerdictIncomplete:
		m.Tags = []string{"warning"}
	}
	return m
}
