package gateway

// bugboard #733: an end user's pub/sub permissions are the namespace's
// end-user topics, loaded once per pub/sub request before the scope gate.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Orama-Network/orama/core/pkg/gateway/auth"
	"github.com/Orama-Network/orama/core/pkg/gateway/routepolicy"
)

func endUserRequest(ns string) *http.Request {
	r := reqWithJWT(&auth.JWTClaims{Sub: "0xenduser"})
	return r.WithContext(context.WithValue(r.Context(), CtxKeyNamespaceOverride, ns))
}

func TestCallerPermissions_anEndUserReadsTheLoadedTopics(t *testing.T) {
	g := &Gateway{}
	read := func(r *http.Request, topic string) bool {
		return g.callerPermissions(r).Permits(auth.Resource{Domain: auth.DomainPubsub, Action: auth.ActionRead, Name: topic})
	}
	r := endUserRequest("anchat")
	if read(r, "presence/global") {
		t.Error("an end user read a topic on a request that loaded no list")
	}
	listed := r.WithContext(context.WithValue(r.Context(), endUserTopicsKey{}, []string{"presence/*"}))
	if !read(listed, "presence/global") || read(listed, "messages:new") {
		t.Error("an end user's reads are not the loaded list")
	}
}

func TestIsEndUserSession(t *testing.T) {
	member := markGrant(endUserRequest("anchat"), &auth.Grant{PrincipalType: auth.PrincipalWallet, Role: auth.RoleRuntime})
	key := reqWithJWT(&auth.JWTClaims{Sub: "ak_x:anchat"})
	key = key.WithContext(context.WithValue(key.Context(), CtxKeyNamespaceOverride, "anchat"))
	for name, c := range map[string]struct {
		r    *http.Request
		want bool
	}{
		"a grantless wallet":    {endUserRequest("anchat"), true},
		"a member":              {member, false},
		"a key's token":         {key, false},
		"a lobby session":       {endUserRequest(auth.LobbyNamespace), false},
		"a session naming none": {endUserRequest(""), false},
	} {
		if got := isEndUserSession(c.r); got != c.want {
			t.Errorf("%s: %v, want %v", name, got, c.want)
		}
	}
}

// A list that cannot be read refuses the request rather than read as empty or
// as everything; a request that needs no list passes untouched.
func TestWithEndUserTopics_refusesWhenTheListCannotBeRead(t *testing.T) {
	g := &Gateway{}
	pubsub := routepolicy.Policy{Domain: string(auth.DomainPubsub), Action: string(auth.ActionRead)}

	rec := httptest.NewRecorder()
	if _, ok := g.withEndUserTopics(rec, endUserRequest("anchat"), pubsub); ok || rec.Code != http.StatusServiceUnavailable {
		t.Errorf("an end user with no auth service: continued %v, status %d; want 503", ok, rec.Code)
	}
	member := markGrant(endUserRequest("anchat"), &auth.Grant{PrincipalType: auth.PrincipalWallet, Role: auth.RoleRuntime})
	if _, ok := g.withEndUserTopics(httptest.NewRecorder(), member, pubsub); !ok {
		t.Error("a member's pub/sub request asked for the end-user list")
	}
	storage := routepolicy.Policy{Domain: string(auth.DomainStorage), Action: string(auth.ActionWrite)}
	if _, ok := g.withEndUserTopics(httptest.NewRecorder(), endUserRequest("anchat"), storage); !ok {
		t.Error("an end user's storage request asked for the end-user list")
	}
}
