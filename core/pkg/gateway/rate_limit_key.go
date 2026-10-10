package gateway

import (
	"net/http"
	"strings"

	"github.com/DeBrosOfficial/network/pkg/gateway/clientkey"
)

// rateLimitClient returns the address to hold responsible for a request, and whether it is internal
// traffic exempt from limits. See package clientkey.
func rateLimitClient(r *http.Request) (client string, exempt bool) { return clientkey.Resolve(r) }

// bucketKey is the key a client address is limited under (its IPv6 /64). See clientkey.BucketKey.
func bucketKey(client string) string { return clientkey.BucketKey(client) }

// authRateLimitPaths are the endpoints that mint or exchange credentials.
//
// They are cheap to call and expensive to serve — a challenge writes a nonce
// row and can create a namespace, a verify runs signature recovery, a token
// exchange mints a JWT — and they are the ones worth grinding. They get their
// own, much tighter bucket than the general one.
const (
	// chainQueriesPerMinute and chainQueryBurst are the per-address bucket of the public
	// /v1/chain/query/ route: each request runs a query on the node's chain process, which is
	// far dearer than the static routes the general limit was sized for. The explorer does not
	// use this route, so a person browsing is nowhere near it.
	chainQueriesPerMinute = 120
	chainQueryBurst       = 30

	chainQueryPathPrefix = "/v1/chain/query/"

	// chainHeadStreamPath is the chain's head as server-sent events (handlers/chainread/head.go).
	// A stream lasts a hundred seconds and a browser connects again when it ends, so a page open
	// all day opens about a thousand a day; the bucket lets a household of tabs do that and not
	// a client that opens streams to hold the gateway's slots. How many streams are open at once
	// is bounded by the handler.
	chainHeadStreamPath       = "/v1/chain/head/stream"
	chainHeadStreamsPerMinute = 12
	chainHeadStreamBurst      = 6
)

// isChainQueryPath reports whether path is the public Orama module-query route.
func isChainQueryPath(path string) bool {
	return strings.HasPrefix(path, chainQueryPathPrefix)
}

func isAuthRateLimitPath(path string) bool {
	switch path {
	case "/v1/auth/challenge", "/v1/auth/verify", "/v1/auth/api-key",
		"/v1/auth/token", "/v1/auth/refresh",
		"/v1/auth/device", "/v1/auth/device/approve", "/v1/auth/device/token",
		"/v1/auth/devices/approve":
		return true
	}
	return false
}
