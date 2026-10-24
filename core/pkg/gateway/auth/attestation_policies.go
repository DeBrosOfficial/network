package auth

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Orama-Network/orama/core/pkg/client"
	"github.com/Orama-Network/orama/core/pkg/logging"
	"go.uber.org/zap"
)

// AttestationPolicies is this gateway's copy of every namespace's attestation
// policy: what each request to a namespace is judged by.
//
// Judging runs on every data-plane request, so it never reads the registry:
// the copy is held in memory and reloaded whole, one query for the cluster,
// with the bounds the revocation list keeps — reloaded once it is
// RevocationRefreshInterval old, and refused once it is RevocationStaleness
// old and cannot be reloaded. Only namespaces whose mode is not off are in it,
// so the copy is the size of the set of namespaces that attest, however many
// namespaces the cluster has: a thousand of them is well under a megabyte.
//
// A changed policy therefore takes effect on every gateway within
// RevocationStaleness, and at once on the gateway that recorded it.
type AttestationPolicies struct {
	registry func() client.DatabaseClient
	logger   *logging.ColoredLogger

	mu          sync.RWMutex
	byNamespace map[string]AttestationPolicy
	// loadedAt is when the last successful read began; a failed one never
	// moves it, so the copy ages towards RevocationStaleness.
	loadedAt time.Time
	loaded   bool
	// lastAttempt rate-limits reloads while the registry does not answer.
	lastAttempt time.Time
	// flight is closed when the reload in progress ends; nil when none is.
	// At most one read runs, however many requests find the copy stale, and a
	// read the registry never answers holds it rather than piling up others.
	flight chan struct{}
	// local holds the policies this gateway recorded itself, so a reload whose
	// read began before one committed does not undo it.
	local map[string]localPolicy

	now           func() time.Time
	reloadTimeout time.Duration
}

type localPolicy struct {
	policy AttestationPolicy
	at     time.Time
}

// ErrAttestationPoliciesUnavailable is a copy older than RevocationStaleness
// that cannot be reloaded. Which namespaces attest is then unknown, and
// unknown is not "off": the request is refused with a retryable 503.
var ErrAttestationPoliciesUnavailable = errors.New("the namespaces' attestation policies are temporarily unavailable")

// NewAttestationPolicies builds the copy a Service judges requests by.
func NewAttestationPolicies(registry func() client.DatabaseClient, logger *logging.ColoredLogger) *AttestationPolicies {
	return &AttestationPolicies{
		registry:      registry,
		logger:        logger,
		byNamespace:   map[string]AttestationPolicy{},
		local:         map[string]localPolicy{},
		now:           time.Now,
		reloadTimeout: revocationReloadTimeout,
	}
}

// For returns namespace's policy from the copy, reloading it first when it is
// stale. A request waits for the reload only when the copy is past
// RevocationStaleness; otherwise it is answered from the copy while the
// reload runs.
func (p *AttestationPolicies) For(namespace string) (AttestationPolicy, error) {
	if p == nil {
		return AttestationPolicy{}, fmt.Errorf("%w: this gateway keeps no copy", ErrAttestationPoliciesUnavailable)
	}
	p.mu.Lock()
	now := p.now()
	fresh := p.loaded && now.Sub(p.loadedAt) < RevocationRefreshInterval
	flight := p.flight
	if !fresh && flight == nil && now.Sub(p.lastAttempt) >= revocationRetryInterval {
		flight = make(chan struct{})
		p.flight = flight
		p.lastAttempt = now
		go p.reload(flight, now)
	}
	mustWait := flight != nil && (!p.loaded || now.Sub(p.loadedAt) >= RevocationStaleness)
	p.mu.Unlock()

	if mustWait {
		select {
		case <-flight:
		case <-time.After(p.reloadTimeout):
		}
	}

	p.mu.RLock()
	defer p.mu.RUnlock()
	if !p.loaded || p.now().Sub(p.loadedAt) >= RevocationStaleness {
		return AttestationPolicy{}, fmt.Errorf("%w: the copy is older than %s", ErrAttestationPoliciesUnavailable, RevocationStaleness)
	}
	if policy, ok := p.byNamespace[namespace]; ok {
		return policy, nil
	}
	return offPolicy(), nil
}

// Refresh reloads the copy now. The socket sweeper calls it before each pass.
func (p *AttestationPolicies) Refresh(ctx context.Context) error {
	if p == nil {
		return nil
	}
	started := p.now()
	read, err := p.read(ctx)
	if err != nil {
		return p.failed(err)
	}
	p.apply(read, started)
	return nil
}

// reload runs one flight and releases whoever waits on it.
func (p *AttestationPolicies) reload(flight chan struct{}, started time.Time) {
	defer func() {
		p.mu.Lock()
		p.flight = nil
		p.mu.Unlock()
		close(flight)
	}()
	ctx, cancel := context.WithTimeout(context.Background(), p.reloadTimeout)
	defer cancel()
	read, err := p.read(ctx)
	if err != nil {
		_ = p.failed(err)
		return
	}
	p.apply(read, started)
}

// read loads every namespace whose mode is not off.
func (p *AttestationPolicies) read(ctx context.Context) (map[string]AttestationPolicy, error) {
	db := p.registry()
	if db == nil {
		return nil, errors.New("no registry database is configured on this gateway")
	}
	res, err := db.Query(client.WithInternalAuth(ctx),
		`SELECT n.name, `+attestationPolicyColumns+`
		   FROM namespace_attestation_policy AS p JOIN namespaces AS n ON n.id = p.namespace_id
		  WHERE p.mode != 'off'`)
	if err != nil {
		return nil, fmt.Errorf("read the attestation policies: %w", err)
	}
	out := map[string]AttestationPolicy{}
	if res == nil {
		return out, nil
	}
	for _, row := range res.Rows {
		if len(row) < 2 {
			continue
		}
		policy, err := scanAttestationPolicy(row[1:])
		if err != nil {
			return nil, fmt.Errorf("the attestation policy of %q: %w", getStringVal(row[0]), err)
		}
		out[getStringVal(row[0])] = policy
	}
	return out, nil
}

// apply replaces the copy with a read that began at started, keeping every
// policy this gateway recorded at or after then.
func (p *AttestationPolicies) apply(read map[string]AttestationPolicy, started time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.loaded && started.Before(p.loadedAt) {
		return // a newer read is already the copy
	}
	for ns, l := range p.local {
		if l.at.Before(started) {
			delete(p.local, ns)
			continue
		}
		setLocal(read, ns, l.policy)
	}
	p.byNamespace = read
	p.loadedAt = started
	p.loaded = true
}

// recordLocal applies a policy this gateway just recorded.
func (p *AttestationPolicies) recordLocal(namespace string, policy AttestationPolicy) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.local[namespace] = localPolicy{policy: policy, at: p.now()}
	setLocal(p.byNamespace, namespace, policy)
}

func setLocal(m map[string]AttestationPolicy, namespace string, policy AttestationPolicy) {
	if policy.Mode == AttestationOff {
		delete(m, namespace)
		return
	}
	m[namespace] = policy
}

// failed logs why a reload produced nothing. The copy is left as it was, and
// ages.
func (p *AttestationPolicies) failed(err error) error {
	if p.logger != nil {
		p.logger.ComponentWarn(logging.ComponentGeneral,
			"could not reload the attestation policies; requests to attesting namespaces are refused once the copy is too old",
			zap.Duration("bound", RevocationStaleness), zap.Error(err))
	}
	return err
}
