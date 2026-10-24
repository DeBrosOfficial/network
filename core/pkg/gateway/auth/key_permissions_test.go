package auth

import "testing"

func TestKeyPermissions(t *testing.T) {
	appRuntime, _ := ProfileGrants("app-runtime")
	appScopes := ScopeSet(setOf(appRuntime)).Canonical()

	cases := []struct {
		name     string
		scopes   string
		role     Role
		selector string
		permits  map[Resource]bool
	}{
		{
			name: "an app-runtime key holds no pubsub, cache or storage", scopes: appScopes, role: RoleRuntime,
			permits: map[Resource]bool{
				{Domain: DomainPubsub, Action: ActionWrite}:  false,
				{Domain: DomainCache, Action: ActionWrite}:   false,
				{Domain: DomainStorage, Action: ActionRead}:  false,
				{Domain: DomainStorage, Action: ActionWrite}: false,
				{Domain: DomainFn, Action: ActionInvoke}:     true,
				{Domain: DomainPush, Action: ActionWrite}:    true,
				{Domain: DomainPush, Action: ActionSend}:     false,
			},
		},
		{
			name: "a key holding push registers and does not send", scopes: ScopePush, role: RoleRuntime,
			permits: map[Resource]bool{
				{Domain: DomainPush, Action: ActionWrite}: true,
				{Domain: DomainPush, Action: ActionSend}:  false,
			},
		},
		{
			name: "a server's push-send key sends and registers nothing", scopes: ScopePushSend, role: RoleRuntime,
			permits: map[Resource]bool{
				{Domain: DomainPush, Action: ActionSend}:  true,
				{Domain: DomainPush, Action: ActionWrite}: false,
				{Domain: DomainFn, Action: ActionInvoke}:  false,
			},
		},
		{
			name: "a function's storage key holds storage write", scopes: ScopeStorage, role: RoleRuntime,
			permits: map[Resource]bool{
				{Domain: DomainStorage, Action: ActionWrite}: true,
				{Domain: DomainFn, Action: ActionInvoke}:     false,
			},
		},
		{
			name: "a cache key holds the cache and nothing else", scopes: ScopeCache, role: RoleRuntime,
			permits: map[Resource]bool{
				{Domain: DomainCache, Action: ActionWrite}:   true,
				{Domain: DomainPubsub, Action: ActionWrite}:  false,
				{Domain: DomainStorage, Action: ActionRead}:  false,
				{Domain: DomainDeploy, Action: ActionManage}: false,
			},
		},
		{
			name: "an admin key holds everything its admin role does", scopes: ScopeAdmin, role: RoleAdmin,
			permits: map[Resource]bool{
				{Domain: DomainPubsub, Action: ActionWrite}: true,
				{Domain: DomainDB, Action: ActionWrite}:     true,
			},
		},
		{
			name: "a role bounds the scopes too", scopes: ScopeAdmin, role: RoleReader,
			permits: map[Resource]bool{{Domain: DomainPubsub, Action: ActionWrite}: false},
		},
		{
			name: "a selector narrows a scope the key holds", scopes: ScopePubsub, role: RoleRuntime,
			selector: "pubsub:topic=jobs.*",
			permits: map[Resource]bool{
				{Domain: DomainPubsub, Action: ActionWrite, Name: "jobs.a"}: true,
				{Domain: DomainPubsub, Action: ActionWrite, Name: "chat.a"}: false,
			},
		},
		{
			name: "a selector cannot widen a key past its scopes", scopes: ScopeCache, role: RoleRuntime,
			selector: "pubsub:topic=jobs.*",
			permits:  map[Resource]bool{{Domain: DomainPubsub, Action: ActionWrite, Name: "jobs.a"}: false},
		},
		{
			name: "no scopes, nothing", scopes: "", role: RoleRuntime,
			permits: map[Resource]bool{{Domain: DomainCache, Action: ActionRead}: false},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			perms := KeyPermissions(tc.scopes, tc.role, tc.selector)
			for r, want := range tc.permits {
				if got := perms.Permits(r); got != want {
					t.Errorf("%s permitted = %v, want %v (holds %s)", r, got, want, perms)
				}
			}
		})
	}
}

// bugboard #434: sending a push reaches every user's devices, so only the
// namespace's server side holds it. A runtime member and a signed-in user
// register their own devices and do not send.
func TestPushSend_heldByTheServerSideOnly(t *testing.T) {
	send := Resource{Domain: DomainPush, Action: ActionSend}
	register := Resource{Domain: DomainPush, Action: ActionWrite}
	for name, c := range map[string]struct {
		set              PermissionSet
		sends, registers bool
	}{
		"owner":                  {PermissionsFor(RoleOwner, ""), true, true},
		"admin":                  {PermissionsFor(RoleAdmin, ""), true, true},
		"developer":              {PermissionsFor(RoleDeveloper, ""), true, true},
		"runtime member":         {PermissionsFor(RoleRuntime, ""), false, true},
		"runtime deployed app":   {AppPermissions(RoleRuntime, ""), true, true},
		"reader deployed app":    {AppPermissions(RoleReader, ""), false, false},
		"reader":                 {PermissionsFor(RoleReader, ""), false, false},
		"signed in, no grant":    {NoGrantPermissions(nil), false, true},
		"push key, no grant row": {PermissionsFromScopes(ScopePush), false, true},
	} {
		if got := c.set.Permits(send); got != c.sends {
			t.Errorf("%s: may send = %v, want %v", name, got, c.sends)
		}
		if got := c.set.Permits(register); got != c.registers {
			t.Errorf("%s: may register = %v, want %v", name, got, c.registers)
		}
	}
}
