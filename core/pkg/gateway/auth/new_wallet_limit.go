package auth

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/Orama-Network/orama/core/pkg/client"
)

// The ceiling on wallets new to a namespace that one client network signs in
// per hour, under open sign-in (bugboard #749).
//
// A wallet is new to a namespace when it never held a session there: no
// refresh_tokens row for (namespace, subject), a table nothing deletes from.
// A returning end user is therefore never counted or refused, and neither is a
// wallet holding a grant: it was invited. What is counted is exactly what is
// free under open sign-in, a fresh identity.
//
// The count is shared by every gateway that signs in to the namespace through
// the same cache: it lives in Olric, not in the registry, because an abuse
// ceiling may be approximate and one Raft write per sign-in would not scale.
// Fixed one-hour windows, keyed by the window's start, so a counter is never
// reset early and needs no cleanup beyond its expiry.
//
// What is counted is distinct wallets, not attempts: a wallet keeps the place
// it was first given in the window. A sign-in that fails after the count (a
// bad device signature, a device-flow poll collected later) and is tried
// again costs the network nothing more, and a wallet refused once stays
// refused until the window ends, however often it retries.

// WindowCounter counts distinct members per key within a fixed window.
// Position returns member's place among the members counted under key: the
// place it was given if it was already counted, else the next one, which it
// keeps until the entry expires window after it was first counted.
type WindowCounter interface {
	Position(ctx context.Context, key, member string, window time.Duration) (int, error)
}

// SetNewWalletCounter gives the service the counter new sign-ins are charged
// to. A service with none refuses every sign-in the ceiling applies to
// (ErrNewWalletCountUnavailable): a ceiling the owner turned on is not skipped.
func (s *Service) SetNewWalletCounter(c WindowCounter) {
	s.newWallets = c
}

// ErrNewWalletCountUnavailable is a sign-in the ceiling applies to that could
// not be counted: the cache is unreachable. Retryable.
var ErrNewWalletCountUnavailable = errors.New("the new-wallet sign-ins of this network cannot be counted right now")

// ErrNewWalletsPerIPExceeded is a sign-in refused because the client network
// signed in as many new wallets this hour as the namespace allows.
type ErrNewWalletsPerIPExceeded struct {
	Limit      int
	RetryAfter time.Duration
}

func (e *ErrNewWalletsPerIPExceeded) Error() string {
	return fmt.Sprintf("this network has signed in %d new wallets to this namespace this hour, as many as it allows", e.Limit)
}

// AdmitSignIn is RequireSignInAllowed for a sign-in about to be issued a
// session, from client (the address the request is attributed to, already
// folded to its bucket key). A wallet let in as a grantless end user that
// never held a session in the namespace is charged to client's count when the
// namespace has its per-IP limits on, and refused past the ceiling.
func (s *Service) AdmitSignIn(ctx context.Context, wallet, namespace, client string) error {
	endUser, err := s.signInAdmission(ctx, wallet, namespace)
	if err != nil || !endUser {
		return err
	}
	limits, err := s.PerIPLimitsOf(ctx, namespace)
	if err != nil {
		return err
	}
	if !limits.Enabled {
		return nil
	}
	returning, err := s.heldSessionIn(ctx, namespace, wallet)
	if err != nil || returning {
		return err
	}
	return s.chargeNewWallet(ctx, namespace, client, wallet, limits.NewWalletsPerHour, time.Now())
}

// heldSessionIn reports whether a wallet was ever issued a session in a
// namespace.
func (s *Service) heldSessionIn(ctx context.Context, namespace, wallet string) (bool, error) {
	db := s.registryDatabase()
	if db == nil {
		return false, fmt.Errorf("client not initialized")
	}
	nsID, err := s.ResolveNamespaceID(ctx, namespace)
	if err != nil {
		return false, fmt.Errorf("resolve the namespace %q: %w", namespace, err)
	}
	res, err := db.Query(client.WithInternalAuth(ctx),
		`SELECT 1 FROM refresh_tokens WHERE subject = ? AND namespace_id = ? LIMIT 1`,
		NormalizeWallet(wallet), nsID)
	if err != nil {
		return false, fmt.Errorf("read the sessions of %s in %q: %w", RedactSubject(wallet), namespace, err)
	}
	return res != nil && len(res.Rows) > 0, nil
}

// chargeNewWallet counts a new wallet against client in the window holding
// now, and refuses it past limit.
func (s *Service) chargeNewWallet(ctx context.Context, namespace, client, wallet string, limit int, now time.Time) error {
	if s.newWallets == nil {
		return ErrNewWalletCountUnavailable
	}
	start := now.Truncate(NewWalletWindow)
	n, err := s.newWallets.Position(ctx, newWalletKey(namespace, client, start), NormalizeWallet(wallet), NewWalletWindow)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrNewWalletCountUnavailable, err)
	}
	if n > limit {
		return &ErrNewWalletsPerIPExceeded{Limit: limit, RetryAfter: start.Add(NewWalletWindow).Sub(now)}
	}
	return nil
}

// newWalletKey names a client network's count in a namespace for the window
// starting at start.
func newWalletKey(namespace, client string, start time.Time) string {
	return namespace + "|" + client + "|" + strconv.FormatInt(start.Unix(), 10)
}
