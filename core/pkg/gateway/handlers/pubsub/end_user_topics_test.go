package pubsub

// bugboard #733: an end user (a signed-in wallet holding no grant) reads only
// the topics its namespace lists for end users. Its permissions carry the list
// (gwauth.NoGrantPermissions), and every read route applies them: subscribe,
// the topic listing and presence.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	gwauth "github.com/Orama-Network/orama/core/pkg/gateway/auth"
	"github.com/Orama-Network/orama/core/pkg/gateway/ctxkeys"
)

func endUser(listed ...string) gwauth.PermissionSet { return gwauth.NoGrantPermissions(listed) }

func TestWebsocketHandler_anEndUserSubscribesOnlyToListedTopics(t *testing.T) {
	h := newTestHandlers(&mockNetworkClient{pubsub: &mockPubSubClient{}})
	w := httptest.NewRecorder()
	h.WebsocketHandler(w, subscribeRequest("messages:new", "", endUser("presence/*")))
	if w.Code != http.StatusForbidden {
		t.Fatalf("subscribing to an unlisted topic: status %d, want 403", w.Code)
	}
}

func TestTopicsHandler_anEndUserSeesOnlyListedTopics(t *testing.T) {
	h := newTestHandlers(&mockNetworkClient{pubsub: &mockPubSubClient{
		ListTopicsFunc: func(context.Context) ([]string, error) {
			return []string{"presence/global", "messages:new", "user/0xbob/feed"}, nil
		},
	}})
	r := withNamespace(httptest.NewRequest(http.MethodGet, "/v1/pubsub/topics", nil), "anchat")
	r = r.WithContext(context.WithValue(r.Context(), ctxkeys.Permissions, endUser("presence/*")))
	rr := httptest.NewRecorder()
	h.TopicsHandler(rr, r)
	var body struct{ Topics []string }
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Topics) != 1 || body.Topics[0] != "presence/global" {
		t.Errorf("an end user was shown %v, want [presence/global]", body.Topics)
	}
}

func TestPresenceHandler_needsReadOnTheTopic(t *testing.T) {
	h := newTestHandlers(&mockNetworkClient{pubsub: &mockPubSubClient{}})
	presence := func(topic string, perms gwauth.PermissionSet) int {
		r := withNamespace(httptest.NewRequest(http.MethodGet, "/v1/pubsub/presence?topic="+topic, nil), "anchat")
		r = r.WithContext(context.WithValue(r.Context(), ctxkeys.Permissions, perms))
		rr := httptest.NewRecorder()
		h.PresenceHandler(rr, r)
		return rr.Code
	}
	for name, c := range map[string]struct {
		topic string
		perms gwauth.PermissionSet
		want  int
	}{
		"an end user on a listed topic":      {"presence/global", endUser("presence/*"), http.StatusOK},
		"an end user on an unlisted topic":   {"messages:new", endUser("presence/*"), http.StatusForbidden},
		"an end user, nothing listed":        {"presence/global", endUser(), http.StatusForbidden},
		"an end user on a reserved topic":    {"_orama/webrtc/r1", endUser("*"), http.StatusForbidden},
		"a member narrowed to another topic": {"messages:new", gwauth.PermissionsFor(gwauth.RoleRuntime, "pubsub:topic=chat.*"), http.StatusForbidden},
		"a runtime member on any topic":      {"messages:new", gwauth.DataPlanePermissions(), http.StatusOK},
		"a runtime member on a reserved one": {"_orama/webrtc/r1", gwauth.DataPlanePermissions(), http.StatusOK},
	} {
		if got := presence(c.topic, c.perms); got != c.want {
			t.Errorf("%s: status %d, want %d", name, got, c.want)
		}
	}
}
