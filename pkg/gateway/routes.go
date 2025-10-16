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

	// network
	mux.HandleFunc("/v1/network/status", g.networkStatusHandler)
	mux.HandleFunc("/v1/network/peers", g.networkPeersHandler)
	mux.HandleFunc("/v1/network/connect", g.networkConnectHandler)
	mux.HandleFunc("/v1/network/disconnect", g.networkDisconnectHandler)

	// pubsub
	mux.HandleFunc("/v1/pubsub/ws", g.pubsubWebsocketHandler)
	mux.HandleFunc("/v1/pubsub/publish", g.pubsubPublishHandler)
	mux.HandleFunc("/v1/pubsub/topics", g.pubsubTopicsHandler)

	// database operations (dynamic clustering)
	mux.HandleFunc("/v1/database/exec", g.databaseExecHandler)
	mux.HandleFunc("/v1/database/query", g.databaseQueryHandler)
	mux.HandleFunc("/v1/database/transaction", g.databaseTransactionHandler)
	mux.HandleFunc("/v1/database/schema", g.databaseSchemaHandler)
	mux.HandleFunc("/v1/database/create-table", g.databaseCreateTableHandler)
	mux.HandleFunc("/v1/database/drop-table", g.databaseDropTableHandler)

	// admin endpoints
	mux.HandleFunc("/v1/admin/databases/create", g.databaseCreateHandler)

	return g.withMiddleware(mux)
}
