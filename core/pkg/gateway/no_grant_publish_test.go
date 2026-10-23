package gateway

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Orama-Network/orama/core/pkg/gateway/auth"
)

// bugboard #733: a signed-in wallet with no grant in a tenant namespace is an
// application's end user, and it could publish to any topic of the namespace.
// It keeps the rest of the data plane and may subscribe, but publishing is for
// a grant that says so, or for the application's functions.

func TestNoGrantWallet_forwardedPublishIsRefusedWithARemedy(t *testing.T) {
	for _, path := range []string{"/v1/pubsub/publish", "/v1/pubsub/publish-batch"} {
		t.Run(path, func(t *testing.T) {
			g, _ := namespaceGatewayForHops(t, "")

			rec, reached := serveHop(g, hop(t, g, http.MethodPost, path, hopNamespace, hopWallet))

			body := rec.Body.String()
			if reached || rec.Code != http.StatusForbidden || !strings.Contains(body, CodeScopeMissing) {
				t.Fatalf("reached %v, status %d, body %s; want 403 %s", reached, rec.Code, body, CodeScopeMissing)
			}
			for _, want := range []string{"pubsub write", "publish from a function", "pubsub:write:*"} {
				if !strings.Contains(body, want) {
					t.Errorf("body %s does not name %q", body, want)
				}
			}
		})
	}
}

func TestNoGrantWallet_directPublishIsRefused(t *testing.T) {
	g, _ := controlPlaneGateway(t, "")
	chain, reached := controlChain(g)

	w := httptest.NewRecorder()
	chain.ServeHTTP(w, grantWalletRequest(http.MethodPost, "/v1/pubsub/publish", "0xstranger", "anchat"))

	// Reached directly, publish asks for ownership, which a wallet with no
	// grant lacks; either refusal keeps it out.
	if *reached || w.Code != http.StatusForbidden {
		t.Fatalf("reached %v, status %d, body %s; want a 403", *reached, w.Code, w.Body.String())
	}
}

// fixedEndUserTopics is a namespace's end-user topic list, or the error
// reading it gave.
type fixedEndUserTopics struct {
	topics []string
	err    error
}

func (f fixedEndUserTopics) CachedEndUserTopicsOf(context.Context, string) ([]string, error) {
	return f.topics, f.err
}

// pubsubReads are the routes that read pub/sub.
var pubsubReads = []string{"/v1/pubsub/ws", "/v1/pubsub/topics", "/v1/pubsub/presence"}

func TestNoGrantWallet_keepsSubscribeAndTheRestOfTheDataPlane(t *testing.T) {
	for _, route := range []struct{ method, path string }{
		{http.MethodGet, "/v1/pubsub/ws"},
		{http.MethodGet, "/v1/pubsub/topics"},
		{http.MethodGet, "/v1/pubsub/presence"},
		{http.MethodPost, "/v1/cache/put"},
		{http.MethodPost, "/v1/storage/upload"},
	} {
		t.Run(route.path, func(t *testing.T) {
			g, _ := namespaceGatewayForHops(t, "")
			g.endUserTopics = fixedEndUserTopics{topics: []string{"presence/*"}}

			rec, reached := serveHop(g, hop(t, g, route.method, route.path, hopNamespace, hopWallet))

			if !reached {
				t.Errorf("a wallet with no grant was refused %s: %d %s", route.path, rec.Code, rec.Body.String())
			}
		})
	}
}

// bugboard #733: a namespace that lists no end-user topic gives its end users
// no pub/sub read at all, the scope gate refusing each read route; one whose
// list cannot be read refuses them 503, not as empty and not as everything;
// and a member reads whatever the list says.
func TestNoGrantWallet_readsPubsubOnlyWhenTheNamespaceListsTopics(t *testing.T) {
	for _, path := range pubsubReads {
		t.Run(path, func(t *testing.T) {
			g, _ := namespaceGatewayForHops(t, "")
			g.endUserTopics = fixedEndUserTopics{topics: []string{}}
			rec, reached := serveHop(g, hop(t, g, http.MethodGet, path, hopNamespace, hopWallet))
			if reached || rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), CodeScopeMissing) {
				t.Errorf("nothing listed: reached %v, %d %s; want 403 %s", reached, rec.Code, rec.Body.String(), CodeScopeMissing)
			}

			g.endUserTopics = fixedEndUserTopics{err: errors.New("registry unreachable")}
			if rec, reached := serveHop(g, hop(t, g, http.MethodGet, path, hopNamespace, hopWallet)); reached || rec.Code != http.StatusServiceUnavailable {
				t.Errorf("an unreadable list: reached %v, %d; want 503", reached, rec.Code)
			}

			member, _ := namespaceGatewayForHops(t, "runtime")
			member.endUserTopics = fixedEndUserTopics{topics: []string{}}
			if rec, reached := serveHop(member, hop(t, member, http.MethodGet, path, hopNamespace, hopWallet)); !reached {
				t.Errorf("a runtime member was held to the end-user list: %d %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestNoGrantWallet_withARuntimeGrantPublishes(t *testing.T) {
	g, _ := namespaceGatewayForHops(t, "runtime")

	rec, reached := serveHop(g, hop(t, g, http.MethodPost, "/v1/pubsub/publish", hopNamespace, hopWallet))

	if !reached {
		t.Errorf("a runtime grant was refused publishing: %d %s", rec.Code, rec.Body.String())
	}
}

func TestNoGrantWallet_ownerAndAdminAndKeysStillPublish(t *testing.T) {
	for _, role := range []string{"developer", "admin", "owner"} {
		g, _ := namespaceGatewayForHops(t, role)
		if rec, reached := serveHop(g, hop(t, g, http.MethodPost, "/v1/pubsub/publish-batch", hopNamespace, hopWallet)); !reached {
			t.Errorf("a %s was refused publishing: %d %s", role, rec.Code, rec.Body.String())
		}
	}
	g, _ := namespaceGatewayForHops(t, "")
	if rec, reached := serveHop(g, keyHop(t, g, http.MethodPost, "/v1/pubsub/publish", "pubsub")); !reached {
		t.Errorf("a pubsub key was refused publishing: %d %s", rec.Code, rec.Body.String())
	}
}

func TestNoGrantWallet_narrowedGrantStillGovernsPublishing(t *testing.T) {
	g, registry := namespaceGatewayForHops(t, "runtime")
	registry.resource = "pubsub:topic=chat.*"

	for topic, want := range map[string]int{
		"chat.general":     http.StatusOK,
		"billing.invoices": http.StatusForbidden,
	} {
		status, reached := serveHopAuthorizing(g, hop(t, g, http.MethodPost, "/v1/pubsub/publish", hopNamespace, hopWallet),
			auth.Resource{Domain: auth.SelectorPubsub, Name: topic, Action: auth.ActionWrite})
		if !reached || status != want {
			t.Errorf("publish to %s: reached %v status %d, want %d", topic, reached, status, want)
		}
	}
}

func TestNoGrantWallet_aReaderIsStillRefusedPublishing(t *testing.T) {
	g, _ := namespaceGatewayForHops(t, "reader")

	rec, reached := serveHop(g, hop(t, g, http.MethodPost, "/v1/pubsub/publish", hopNamespace, hopWallet))

	if reached || rec.Code != http.StatusForbidden {
		t.Errorf("reached %v, status %d; want a reader refused", reached, rec.Code)
	}
	if strings.Contains(rec.Body.String(), "holds no grant") {
		t.Errorf("a reader holds a grant, but was told it holds none: %s", rec.Body.String())
	}
}

// With a topic listed for end users, publishing is the only data-plane loss;
// reading pub/sub is limited to what the namespace lists (bugboard #733).
func TestNoGrantPermissions_publishIsTheOnlyDataPlaneLoss(t *testing.T) {
	noGrant, full := auth.NoGrantPermissions([]string{"presence/*"}), auth.DataPlanePermissions()
	if noGrant.PermitsDomain(auth.DomainPubsub, auth.ActionWrite) {
		t.Error("a wallet with no grant may publish")
	}
	for _, p := range []struct {
		d auth.Domain
		a auth.Action
	}{
		{auth.DomainPubsub, auth.ActionRead}, {auth.DomainStorage, auth.ActionWrite}, {auth.DomainCache, auth.ActionWrite},
		{auth.DomainPush, auth.ActionWrite}, {auth.DomainWebRTC, auth.ActionRead}, {auth.DomainProxy, auth.ActionWrite},
		{auth.DomainFn, auth.ActionInvoke},
	} {
		if !noGrant.PermitsDomain(p.d, p.a) {
			t.Errorf("a wallet with no grant lost %s:%s", p.d, p.a)
		}
		if !full.PermitsDomain(p.d, p.a) {
			t.Errorf("the data plane lacks %s:%s", p.d, p.a)
		}
	}
	if noGrant.PermitsDomain(auth.DomainDB, auth.ActionRead) || noGrant.IsAdmin() {
		t.Error("a wallet with no grant holds more than the data plane")
	}
}

// bugboard #733: an end user reads the topics the namespace lists and no
// other, and none at all when it lists none.
func TestNoGrantPermissions_readsOnlyTheListedTopics(t *testing.T) {
	read := func(topic string) auth.Resource {
		return auth.Resource{Domain: auth.DomainPubsub, Action: auth.ActionRead, Name: topic}
	}
	listed := auth.NoGrantPermissions([]string{"presence/*", "users/global"})
	for topic, want := range map[string]bool{
		"presence/global": true, "presence/rooms/a": true, "users/global": true,
		"messages:new": false, "user/0xbob/feed": false, "users/global2": false,
	} {
		if got := listed.Permits(read(topic)); got != want {
			t.Errorf("listed topics: may read %q = %v, want %v", topic, got, want)
		}
	}
	if none := auth.NoGrantPermissions(nil); none.PermitsDomain(auth.DomainPubsub, auth.ActionRead) {
		t.Error("an end user of a namespace that lists no topic reads pub/sub")
	}
}
