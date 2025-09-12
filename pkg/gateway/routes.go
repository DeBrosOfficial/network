package gateway

import "net/http"

// Routes returns the http.Handler with all routes and middleware configured
func (g *Gateway) Routes() http.Handler {
	mux := http.NewServeMux()

	// root and v1 health/status
	mux.HandleFunc("/health", g.healthHandler)
	mux.HandleFunc("/status", g.statusHandler)
	mux.HandleFunc("/v1/health", g.healthHandler)
	mux.HandleFunc("/v1/version", g.versionHandler)
	mux.HandleFunc("/v1/status", g.statusHandler)

	// auth endpoints
	mux.HandleFunc("/v1/auth/jwks", g.jwksHandler)
	mux.HandleFunc("/.well-known/jwks.json", g.jwksHandler)
	mux.HandleFunc("/v1/auth/login", g.loginPageHandler)
	mux.HandleFunc("/v1/auth/challenge", g.challengeHandler)
	mux.HandleFunc("/v1/auth/verify", g.verifyHandler)
	// New: issue JWT from API key; new: create or return API key for a wallet after verification
	mux.HandleFunc("/v1/auth/token", g.apiKeyToJWTHandler)
	mux.HandleFunc("/v1/auth/api-key", g.issueAPIKeyHandler)
	mux.HandleFunc("/v1/auth/register", g.registerHandler)
	mux.HandleFunc("/v1/auth/refresh", g.refreshHandler)
	mux.HandleFunc("/v1/auth/logout", g.logoutHandler)
	mux.HandleFunc("/v1/auth/whoami", g.whoamiHandler)

	// apps CRUD
	mux.HandleFunc("/v1/apps", g.appsHandler)
	mux.HandleFunc("/v1/apps/", g.appsHandler)

	// database
	mux.HandleFunc("/v1/db/query", g.dbQueryHandler)
	mux.HandleFunc("/v1/db/transaction", g.dbTransactionHandler)
	mux.HandleFunc("/v1/db/schema", g.dbSchemaHandler)
	mux.HandleFunc("/v1/db/create-table", g.dbCreateTableHandler)
	mux.HandleFunc("/v1/db/drop-table", g.dbDropTableHandler)

	// network
	mux.HandleFunc("/v1/network/status", g.networkStatusHandler)
	mux.HandleFunc("/v1/network/peers", g.networkPeersHandler)
	mux.HandleFunc("/v1/network/connect", g.networkConnectHandler)
	mux.HandleFunc("/v1/network/disconnect", g.networkDisconnectHandler)

	// pubsub
	mux.HandleFunc("/v1/pubsub/ws", g.pubsubWebsocketHandler)
	mux.HandleFunc("/v1/pubsub/publish", g.pubsubPublishHandler)
	mux.HandleFunc("/v1/pubsub/topics", g.pubsubTopicsHandler)

	return g.withMiddleware(mux)
}
