package gateway

import "net/http"

// Routes returns the http.Handler with all routes and middleware configured
func (g *Gateway) Routes() http.Handler {
	mux := http.NewServeMux()

	// root and v1 health/status
	mux.HandleFunc("/health", g.healthHandler)
	mux.HandleFunc("/status", g.statusHandler)
	mux.HandleFunc("/v1/health", g.healthHandler)
	mux.HandleFunc("/v1/status", g.statusHandler)

	// auth endpoints
	mux.HandleFunc("/v1/auth/jwks", g.jwksHandler)
	mux.HandleFunc("/v1/auth/challenge", g.challengeHandler)
	mux.HandleFunc("/v1/auth/verify", g.verifyHandler)
	mux.HandleFunc("/v1/auth/register", g.registerHandler)
	mux.HandleFunc("/v1/auth/refresh", g.refreshHandler)
	mux.HandleFunc("/v1/auth/logout", g.logoutHandler)
	mux.HandleFunc("/v1/auth/whoami", g.whoamiHandler)

	// apps CRUD
	mux.HandleFunc("/v1/apps", g.appsHandler)
	mux.HandleFunc("/v1/apps/", g.appsHandler)

	// storage and network
	mux.HandleFunc("/v1/storage", g.storageHandler)
	mux.HandleFunc("/v1/network/status", g.networkStatusHandler)
	mux.HandleFunc("/v1/network/peers", g.networkPeersHandler)

	return g.withMiddleware(mux)
}
