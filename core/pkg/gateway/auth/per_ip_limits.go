package auth

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Orama-Network/orama/core/pkg/client"
)

// Per-network ceilings a namespace turns on (bugboard #749).
//
// With open sign-in a wallet costs nothing: one host can mint a new identity
// as often as the credential limiter lets it, multiply the per-wallet invoke
// limit by the identities it holds, and fill the application with end users
// nobody asked for. The namespace's owner may bound what one client network
// does, whatever identities it presents: how many wallets new to the namespace
// it may sign in per hour, and how many authenticated invokes it may make per
// minute. A client network is the address the gateway attributes a request to,
// an IPv6 one folded to its /64 (clientkey.BucketKey).
//
// Off by default. A ceiling every namespace had would refuse an office or a
// carrier NAT that puts many people behind one address, so it is the owner's
// call, made for a namespace whose sign-in is open.

// Defaults a namespace has when it turns the ceilings on without naming them.
const (
	// DefaultNewWalletsPerIPHour is how many wallets that never held a session
	// in the namespace one client network may sign in per hour.
	DefaultNewWalletsPerIPHour = 20
	// DefaultInvokesPerIPMinute is how many authenticated invokes one client
	// network may make per minute on one gateway: twice one wallet's limit
	// (serverless.DefaultLimiterConfig), so a household with two users is not
	// squeezed and a host holding a hundred wallets gets two wallets' worth.
	DefaultInvokesPerIPMinute = 1200
)

// Bounds on the configurable values.
const (
	// MaxNewWalletsPerIPHour bounds the sign-in ceiling. Past it the ceiling
	// stops meaning anything a namespace would want; turning it off is the
	// honest way to say "no ceiling".
	MaxNewWalletsPerIPHour = 10_000
	// MaxInvokesPerIPMinute bounds the invoke ceiling at the per-namespace
	// default, which refuses everything above it anyway.
	MaxInvokesPerIPMinute = 60_000
)

// NewWalletWindow is the window new sign-ins are counted in.
const NewWalletWindow = time.Hour

// PerIPLimits is a namespace's per-network ceilings.
type PerIPLimits struct {
	// Enabled turns both ceilings on. The values below are kept while it is
	// off, so turning it back on restores what the owner chose.
	Enabled bool
	// NewWalletsPerHour is the sign-in ceiling. It bites only where sign-in
	// is open: a wallet holding a grant was invited and is never counted.
	NewWalletsPerHour int
	// InvokesPerMinute is the authenticated-invoke ceiling, per gateway.
	InvokesPerMinute int
}

// defaultPerIPLimits is what a namespace with no row, or the lobby, has.
func defaultPerIPLimits() PerIPLimits {
	return PerIPLimits{NewWalletsPerHour: DefaultNewWalletsPerIPHour, InvokesPerMinute: DefaultInvokesPerIPMinute}
}

// PerIPLimitsChange is a change to a namespace's ceilings. A nil field keeps
// what it was.
type PerIPLimitsChange struct {
	Enabled           *bool
	NewWalletsPerHour *int
	InvokesPerMinute  *int
}

// Empty reports whether the change names nothing.
func (c PerIPLimitsChange) Empty() bool {
	return c.Enabled == nil && c.NewWalletsPerHour == nil && c.InvokesPerMinute == nil
}

// Validate checks the values a change names.
func (c PerIPLimitsChange) Validate() error {
	if v := c.NewWalletsPerHour; v != nil && (*v < 1 || *v > MaxNewWalletsPerIPHour) {
		return fmt.Errorf("new_wallets_per_ip_hour is 1 to %d (got %d); turn per_ip_limits off for no ceiling", MaxNewWalletsPerIPHour, *v)
	}
	if v := c.InvokesPerMinute; v != nil && (*v < 1 || *v > MaxInvokesPerIPMinute) {
		return fmt.Errorf("invokes_per_ip_minute is 1 to %d (got %d); turn per_ip_limits off for no ceiling", MaxInvokesPerIPMinute, *v)
	}
	return nil
}

func (c PerIPLimitsChange) applyTo(l PerIPLimits) PerIPLimits {
	if c.Enabled != nil {
		l.Enabled = *c.Enabled
	}
	if c.NewWalletsPerHour != nil {
		l.NewWalletsPerHour = *c.NewWalletsPerHour
	}
	if c.InvokesPerMinute != nil {
		l.InvokesPerMinute = *c.InvokesPerMinute
	}
	return l
}

// ParsePerIPLimitsSwitch reads "on" or "off".
func ParsePerIPLimitsSwitch(raw string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case perIPLimitsOn:
		return true, nil
	case perIPLimitsOff:
		return false, nil
	default:
		return false, fmt.Errorf("per_ip_limits is on or off (got %q)", raw)
	}
}

// The stored values of per_ip_limits.
const (
	perIPLimitsOn  = "on"
	perIPLimitsOff = "off"
)

// PerIPLimitsSwitch names the switch the way it is stored and set.
func PerIPLimitsSwitch(enabled bool) string {
	if enabled {
		return perIPLimitsOn
	}
	return perIPLimitsOff
}

// PerIPLimitsOf returns a namespace's ceilings, as recorded now.
func (s *Service) PerIPLimitsOf(ctx context.Context, namespace string) (PerIPLimits, error) {
	namespace = sessionNamespace(namespace)
	if IsLobbyNamespace(namespace) {
		return defaultPerIPLimits(), nil
	}
	db, nsID, err := s.deviceRegistry(ctx, namespace)
	if err != nil {
		return PerIPLimits{}, err
	}
	res, err := db.Query(client.WithInternalAuth(ctx),
		`SELECT per_ip_limits, new_wallets_per_ip_hour, invokes_per_ip_minute
		   FROM namespace_session_policy WHERE namespace_id = ? LIMIT 1`, nsID)
	if err != nil {
		return PerIPLimits{}, fmt.Errorf("read the per-IP limits of %q: %w", namespace, err)
	}
	limits := defaultPerIPLimits()
	if res != nil && len(res.Rows) > 0 && len(res.Rows[0]) >= 3 {
		row := res.Rows[0]
		if limits.Enabled, err = ParsePerIPLimitsSwitch(getStringVal(row[0])); err != nil {
			return PerIPLimits{}, err
		}
		if limits.NewWalletsPerHour, err = parseIntCell(row[1]); err != nil {
			return PerIPLimits{}, fmt.Errorf("the new-wallet ceiling of %q: %w", namespace, err)
		}
		if limits.InvokesPerMinute, err = parseIntCell(row[2]); err != nil {
			return PerIPLimits{}, fmt.Errorf("the invoke ceiling of %q: %w", namespace, err)
		}
	}
	s.perIPLimits.put(namespace, limits, time.Now())
	return limits, nil
}

// CachedPerIPLimitsOf is PerIPLimitsOf, answered from what this gateway read
// within devicePolicyStaleness: the invoke ceiling is asked on every
// authenticated invoke, so a change reaches every gateway within that bound
// instead of costing a registry read per call.
func (s *Service) CachedPerIPLimitsOf(ctx context.Context, namespace string) (PerIPLimits, error) {
	namespace = sessionNamespace(namespace)
	if l, ok := s.perIPLimits.get(namespace, time.Now()); ok {
		return l, nil
	}
	return s.PerIPLimitsOf(ctx, namespace)
}

// AuthenticatedInvokesPerIP is the invoke ceiling the serverless limiter asks
// for (serverless.NamespaceLimits): requests per minute per client network,
// 0 when the namespace has none.
func (s *Service) AuthenticatedInvokesPerIP(ctx context.Context, namespace string) (int, error) {
	l, err := s.CachedPerIPLimitsOf(ctx, namespace)
	if err != nil || !l.Enabled {
		return 0, err
	}
	return l.InvokesPerMinute, nil
}

// SetPerIPLimits applies a change to a namespace's ceilings and returns them
// as recorded. It writes only those columns: a row it creates starts with the
// device and sign-in policies a namespace with no row has.
func (s *Service) SetPerIPLimits(ctx context.Context, namespace string, change PerIPLimitsChange, actor string) (PerIPLimits, error) {
	if err := change.Validate(); err != nil {
		return PerIPLimits{}, err
	}
	if s.db == nil {
		return PerIPLimits{}, ErrRotationNotConfigured
	}
	namespace = sessionNamespace(namespace)
	if IsLobbyNamespace(namespace) {
		return PerIPLimits{}, ErrLobbyHasNoSignInPolicy
	}
	current, err := s.PerIPLimitsOf(ctx, namespace)
	if err != nil {
		return PerIPLimits{}, err
	}
	_, nsID, err := s.deviceRegistry(ctx, namespace)
	if err != nil {
		return PerIPLimits{}, err
	}
	next := change.applyTo(current)
	if _, err := s.db.Exec(client.WithInternalAuth(ctx),
		`INSERT INTO namespace_session_policy(namespace_id, device_policy, sign_in,
		   per_ip_limits, new_wallets_per_ip_hour, invokes_per_ip_minute, updated_by, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, datetime('now'))
		 ON CONFLICT(namespace_id) DO UPDATE SET
		   per_ip_limits = excluded.per_ip_limits,
		   new_wallets_per_ip_hour = excluded.new_wallets_per_ip_hour,
		   invokes_per_ip_minute = excluded.invokes_per_ip_minute,
		   updated_by = excluded.updated_by,
		   updated_at = excluded.updated_at`,
		nsID, string(DevicePolicyOptional), string(SignInMembers),
		PerIPLimitsSwitch(next.Enabled), next.NewWalletsPerHour, next.InvokesPerMinute, actor); err != nil {
		return PerIPLimits{}, fmt.Errorf("record the per-IP limits of %q: %w", namespace, err)
	}
	s.perIPLimits.put(namespace, next, time.Now())
	return next, nil
}
