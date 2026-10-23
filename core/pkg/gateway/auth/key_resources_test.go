package auth

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
)

// The owner's case: one key for exactly these five functions, which share no
// single pattern.
var fiveFunctionSelectors = []string{
	"fn:subscription-read",
	"fn:subscription-intent-create",
	"fn:subscription-record-payment",
	"fn:user-find-by-username",
	"fn:user-find-by-wallet",
}

func TestNormalizeKeyResources_acceptsWhatTheKeyHoldsAndIsApplied(t *testing.T) {
	got, err := NormalizeKeyResources("invoke", append([]string{" fn:user-find-by-wallet "}, fiveFunctionSelectors...))
	if err != nil {
		t.Fatalf("five function selectors on an invoke key: %v", err)
	}
	if len(got) != len(fiveFunctionSelectors) {
		t.Fatalf("got %v: a repeated selector is one selector", got)
	}
	for i := 1; i < len(got); i++ {
		if got[i-1] >= got[i] {
			t.Fatalf("got %v, want it sorted so the stored form is canonical", got)
		}
	}
	if round := KeyResourcesFromStored(got.Stored()); strings.Join(round, " ") != strings.Join(got, " ") {
		t.Errorf("stored %q read back as %v", got.Stored(), round)
	}

	if got, err := NormalizeKeyResources("invoke", []string{"fn:premium-*"}); err != nil || got.Stored() != "fn:premium-*" {
		t.Errorf("a pattern: %v, %v", got, err)
	}
	if got, err := NormalizeKeyResources("invoke,storage", []string{"fn:a", "storage:avatars/*"}); err != nil || len(got) != 2 {
		t.Errorf("one selector per scope: %v, %v", got, err)
	}
	if got, err := NormalizeKeyResources("invoke", nil); err != nil || got != nil {
		t.Errorf("no resources is the whole of the scopes: %v, %v", got, err)
	}
}

// Each refusal is one a grant's selector gets, or one that only a key can
// run into, and each is the caller's to fix.
func TestNormalizeKeyResources_refusals(t *testing.T) {
	tooMany := make([]string, 0, maxKeyResources+1)
	for i := 0; i <= maxKeyResources; i++ {
		tooMany = append(tooMany, "fn:f"+strings.Repeat("x", i))
	}
	for name, tc := range map[string]struct {
		scopes    string
		resources []string
	}{
		"a domain the key does not hold":             {"invoke", []string{"storage:avatars/*"}},
		"a domain nothing applies":                   {"push", []string{"push:topic=x"}},
		"an unknown domain":                          {"invoke", []string{"deploy:web"}},
		"not a selector":                             {"invoke", []string{"premium-status"}},
		"an admin key":                               {"admin", []string{"fn:a"}},
		"a scope no resource narrows":                {"invoke,storage", []string{"fn:a"}},
		"the separator inside a selector":            {"invoke", []string{"fn:a,fn:b"}},
		"more than a key carries":                    {"invoke", tooMany},
		"a selector with a space":                    {"invoke", []string{"fn:a b"}},
		"a role's whole data plane, narrowed in one": {"invoke,proxy,push,storage,webrtc", []string{"fn:a"}},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := NormalizeKeyResources(tc.scopes, tc.resources)
			var invalid *ErrInvalidGrant
			if !errors.As(err, &invalid) {
				t.Fatalf("got %v, want *ErrInvalidGrant", err)
			}
		})
	}
}

func TestKeyResources_narrowToExactlyWhatTheyMatch(t *testing.T) {
	held := PermissionsFromScopes("invoke")
	narrowed := KeyResources(fiveFunctionSelectors).Narrow(held)
	for _, sel := range fiveFunctionSelectors {
		name := strings.TrimPrefix(sel, "fn:")
		if !narrowed.Permits(Resource{Domain: DomainFn, Action: ActionInvoke, Name: name}) {
			t.Errorf("%s refused", name)
		}
	}
	for _, name := range []string{"user-delete", "subscription-read-all", "subscription", ""} {
		if narrowed.Permits(Resource{Domain: DomainFn, Action: ActionInvoke, Name: name}) {
			t.Errorf("%q permitted", name)
		}
	}
	if narrowed.Permits(Resource{Domain: DomainFn, Action: ActionManage, Name: "subscription-read"}) {
		t.Error("a narrowed key may manage a function it may only invoke")
	}

	if got := KeyResources(nil).Narrow(held); got.String() != held.String() {
		t.Errorf("no resources changed %s into %s", held, got)
	}

	// A selector can only narrow: one in a domain the credential never
	// reached, or one it reads as wider than what it holds, is dropped.
	if got := (KeyResources{"storage:avatars/*"}).Narrow(held); len(got) != 0 {
		t.Errorf("a storage selector widened an invoke key to %s", got)
	}
	narrowGrant := PermissionsFor(RoleRuntime, "fn:name=premium-*")
	if got := (KeyResources{"fn:*"}).Narrow(narrowGrant); len(got) != 0 {
		t.Errorf("fn:* widened a grant narrowed to premium-* to %s", got)
	}
	if got := (KeyResources{"fn:premium-status"}).Narrow(narrowGrant); len(got) != 1 {
		t.Errorf("a name inside the grant's pattern was dropped: %s", got)
	}
	if got := (KeyResources{"fn:"}).Narrow(held); len(got) != 0 {
		t.Errorf("an unreadable selector permitted %s", got)
	}
}

// Stored, listed, carried over by a rotation, and refused before anything is
// written — against the real schema.
func TestIssueScopedKey_storesItsResourcesAgainstTheRealSchema(t *testing.T) {
	s, db, _ := realRegistry(t)
	ctx := context.Background()

	_, id, err := s.IssueScopedKey(ctx, "anchat", "invoke", KeyOptions{Label: "app", Resources: fiveFunctionSelectors})
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	want, _ := NormalizeKeyResources("invoke", fiveFunctionSelectors)
	if got := storedResources(t, db, id); got.String != want.Stored() {
		t.Errorf("api_keys.resources = %q, want %q", got.String, want.Stored())
	}

	_, plain, err := s.IssueScopedKey(ctx, "anchat", "invoke", KeyOptions{Label: "plain"})
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if got := storedResources(t, db, plain); got.Valid {
		t.Errorf("a key with no resources stored %q; NULL is the whole of its scopes", got.String)
	}

	successor, newID, err := s.RotateKey(ctx, "anchat", id, RotateOptions{})
	if err != nil || successor == "" {
		t.Fatalf("rotate: %v", err)
	}
	if got := storedResources(t, db, newID); got.String != want.Stored() {
		t.Errorf("the successor's resources = %q, want %q — a rotation that dropped them would be the whole of the scopes",
			got.String, want.Stored())
	}

	keys, err := s.ListKeys(ctx, "anchat")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	listed := map[int64]KeyResources{}
	for _, k := range keys {
		listed[k.ID] = k.Resources
	}
	if listed[id].Stored() != want.Stored() || listed[newID].Stored() != want.Stored() || len(listed[plain]) != 0 {
		t.Errorf("listed resources %v", listed)
	}

	var before int
	if err := db.db.QueryRow(`SELECT COUNT(*) FROM api_keys`).Scan(&before); err != nil {
		t.Fatalf("count: %v", err)
	}
	_, _, err = s.IssueScopedKey(ctx, "anchat", "invoke", KeyOptions{Resources: []string{"storage:avatars/*"}})
	var invalid *ErrInvalidGrant
	if !errors.As(err, &invalid) {
		t.Fatalf("a selector the key does not hold: %v, want *ErrInvalidGrant", err)
	}
	var after int
	if err := db.db.QueryRow(`SELECT COUNT(*) FROM api_keys`).Scan(&after); err != nil {
		t.Fatalf("count: %v", err)
	}
	if after != before {
		t.Error("a refused key was written anyway")
	}
}

func storedResources(t *testing.T, db *sqliteDatabase, id int64) sql.NullString {
	t.Helper()
	var got sql.NullString
	if err := db.db.QueryRow(`SELECT resources FROM api_keys WHERE id = ?`, id).Scan(&got); err != nil {
		t.Fatalf("read key %d: %v", id, err)
	}
	return got
}
