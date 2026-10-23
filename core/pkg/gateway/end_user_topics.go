package gateway

import (
	"context"
	"net/http"
	"strings"

	"github.com/Orama-Network/orama/core/pkg/gateway/auth"
	"github.com/Orama-Network/orama/core/pkg/gateway/routepolicy"
	"github.com/Orama-Network/orama/core/pkg/logging"
	"go.uber.org/zap"
)

// The pub/sub topics an end user reads are the namespace's to name (bugboard
// #733; auth/end_user_topics.go). They are read here, once per request, before
// the scope gate computes the caller's permissions, because reading them can
// fail and the permission computation cannot answer a request.

// endUserTopicSource reads the topics a namespace lists for its end users,
// through the short cache auth.Service keeps.
type endUserTopicSource interface {
	CachedEndUserTopicsOf(ctx context.Context, namespace string) ([]string, error)
}

// endUserTopicSource is where this gateway reads end-user topics from: the
// auth service, unless something else was set.
func (g *Gateway) endUserTopicSource() endUserTopicSource {
	if g.endUserTopics != nil {
		return g.endUserTopics
	}
	if g.authService != nil {
		return g.authService
	}
	return nil
}

// endUserTopicsKey carries the topics the namespace lists for its end users on
// a request made by one.
type endUserTopicsKey struct{}

// isEndUserSession reports whether the request is a signed-in wallet's that
// holds no grant in the namespace it names: an application's end user, who
// holds auth.NoGrantPermissions.
func isEndUserSession(r *http.Request) bool {
	ctx := r.Context()
	if grant, _ := ctx.Value(ctxKeyGrant).(*auth.Grant); grant != nil {
		return false
	}
	claims, _ := ctx.Value(ctxKeyJWT).(*auth.JWTClaims)
	if claims == nil || strings.TrimSpace(claims.Sub) == "" || isAPIKeySubject(claims.Sub) {
		return false
	}
	ns, _ := ctx.Value(CtxKeyNamespaceOverride).(string)
	return !auth.IsLobbyNamespace(ns) && strings.TrimSpace(ns) != ""
}

// endUserTopicsOf is what withEndUserTopics put on the request: none when it
// was not asked, so an end user reads no topic on a route that did not load
// them.
func endUserTopicsOf(ctx context.Context) []string {
	topics, _ := ctx.Value(endUserTopicsKey{}).([]string)
	return topics
}

// withEndUserTopics loads the namespace's end-user topics onto a pub/sub read
// made by an end user. Only a pub/sub read asks: no other route reads them,
// and an end user publishes to no topic whatever the list says. It reports
// whether the request may continue; on false it has been answered, 503,
// because a list that could not be read is not a list that is empty.
func (g *Gateway) withEndUserTopics(w http.ResponseWriter, r *http.Request, policy routepolicy.Policy) (*http.Request, bool) {
	if policy.Domain != string(auth.DomainPubsub) || policy.Action != string(auth.ActionRead) || !isEndUserSession(r) {
		return r, true
	}
	source := g.endUserTopicSource()
	if source == nil {
		writeError(w, http.StatusServiceUnavailable, "the namespace's end-user topics cannot be read: auth service not initialized")
		return nil, false
	}
	ns, _ := r.Context().Value(CtxKeyNamespaceOverride).(string)
	topics, err := source.CachedEndUserTopicsOf(r.Context(), ns)
	if err != nil {
		g.logger.ComponentError(logging.ComponentGeneral, "could not read the namespace's end-user topics; refusing the request",
			zap.String("namespace", ns), zap.Error(err))
		writeError(w, http.StatusServiceUnavailable, "the namespace's end-user topics could not be read right now; retry shortly")
		return nil, false
	}
	return r.WithContext(context.WithValue(r.Context(), endUserTopicsKey{}, topics)), true
}
