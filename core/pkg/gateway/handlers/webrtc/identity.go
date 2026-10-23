package webrtc

import (
	"net/http"
	"strings"

	"github.com/Orama-Network/orama/core/pkg/gateway/auth"
	"github.com/Orama-Network/orama/core/pkg/gateway/ctxkeys"
)

// caller is who the gateway authenticated for a request.
type caller struct {
	// UserID is who the caller is to the namespace (auth.JWTClaims.AccountIdentity):
	// the account its claims provider signed into the token, else the token's
	// subject — a wallet, or a deployed app's workload. It is the participant's
	// identity everywhere: on the SFU's wire, in admissions, kicks, mutes and
	// membership events. A session the namespace named an account for never
	// puts its wallet in front of another participant.
	UserID string
	// DeviceID is the device the session is bound to, "" when bound to none.
	DeviceID string
}

// callerOf reads the identity the auth middleware validated. It never reads a
// request header or the join frame: a client controls both. ok is false when no
// token was validated, which the route's policy should have made impossible.
func callerOf(r *http.Request) (caller, bool) {
	claims, ok := r.Context().Value(ctxkeys.JWT).(*auth.JWTClaims)
	if !ok || claims == nil {
		return caller{}, false
	}
	user := strings.TrimSpace(claims.AccountIdentity())
	if user == "" {
		return caller{}, false
	}
	return caller{UserID: user, DeviceID: strings.TrimSpace(claims.Did)}, true
}
