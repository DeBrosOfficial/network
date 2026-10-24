package auth

import (
	"context"
	"fmt"
	"strconv"
)

// A socket opened with a bare API key holds no token: nothing it carries can be
// looked up on the revocation list. The gateway that holds it knows the key's
// registry id — the index gateway from its own lookup, a namespace gateway
// from the hop (X-Internal-Auth-Key-ID) — so revoking a key also revokes that
// id, and the socket sweeper asks by it (bug-702). The prefix is one no wallet,
// key hash, device or session revocation ever carries.
const keyIDRevocationPrefix = "api-key-id:"

// keyIDRevocationSubject is the subject a key's id is revoked under.
func keyIDRevocationSubject(id int64) string {
	return keyIDRevocationPrefix + strconv.FormatInt(id, 10)
}

// revokeKeyID puts a revoked key's id on the revocation list. The entry needs
// to outlive only the sweeps that close the key's open sockets — every gateway
// sweeps within seconds — and a revoked key opens no new one; it is kept as
// long as the key's exchanged tokens are.
func (s *Service) revokeKeyID(ctx context.Context, id int64, reason string) error {
	return s.revocations.RevokeSubject(ctx, keyIDRevocationSubject(id), reason, maxExchangedTokenLifetime)
}

// RevokedKey reports whether the API key with registry id id was revoked. The
// socket sweeper asks it for every socket a bare key opened. The error is
// ErrRevocationsUnavailable when the list is too old to say.
func (s *Service) RevokedKey(id int64) (bool, error) {
	if id <= 0 {
		return false, fmt.Errorf("no API key id to check")
	}
	return s.revocations.DeniesSubject(keyIDRevocationSubject(id))
}
