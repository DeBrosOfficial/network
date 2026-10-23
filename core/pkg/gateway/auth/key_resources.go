package auth

import (
	"fmt"
	"sort"
	"strings"
)

// An API key narrowed to named resources: `fn:premium-status`, `fn:premium-*`.
//
// A key's authority is its own row — the scopes written when it was minted —
// and not the grant that records its membership of the namespace (see
// grantServiceAccount). Its resources live beside its scopes for the same
// reason, in api_keys.resources (migration 082), and are read in the same
// lookup, cached with them and forwarded with them across the proxy hop, so a
// narrowed key costs no round trip an unnarrowed one does not.
//
// A key may carry several selectors where a grant carries one: what an
// application ships with is usually a handful of functions that share no
// pattern, and minting one key per function would put five keys in one client.
// The rule is the grant's, applied to each: a key with no resources reaches
// the whole of its scopes, and a key with some reaches exactly what one of
// them matches (KeyResources.Narrow), refused through the same
// AuthorizeResource a narrowed wallet is.

// maxKeyResources bounds how many selectors one key carries. The list is read
// on every request the key makes and travels in a header across the proxy hop;
// a key that needs more than this is a credential for the whole domain.
const maxKeyResources = 32

// keyResourceSeparator joins a key's selectors in the column and in the hop
// header, as the scopes column joins its words.
const keyResourceSeparator = ","

// KeyResources is the canonical list of selectors an API key is narrowed to:
// sorted, without duplicates. Empty is a key that reaches the whole of its
// scopes.
type KeyResources []string

// Stored is the list as api_keys.resources and the hop header hold it.
func (k KeyResources) Stored() string { return strings.Join(k, keyResourceSeparator) }

// KeyResourcesFromStored reads the column or the hop header. It does not
// validate: the list was validated when it was written, and a selector this
// binary cannot read permits nothing (Narrow) rather than being dropped from a
// list that would then be shorter, or empty and so unrestricted.
func KeyResourcesFromStored(raw string) KeyResources {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, keyResourceSeparator)
	out := make(KeyResources, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// NormalizeKeyResources validates the selectors a key is to be minted with
// against its stored scope set and returns them canonical.
//
// Each selector must be one the data path applies and must narrow a scope the
// key holds — `fn:` needs `invoke` — exactly as a grant's selector must narrow
// its role (validSelector). And each scope the key holds must be narrowed by
// one of them: a narrowed key reaches only what its selectors match, so a
// scope none of them narrows would be listed on the key and reach nothing.
// `admin` is never narrowed this way — it is the control plane, which no
// selector reaches.
//
// Refusals are *ErrInvalidGrant, the caller's to fix.
func NormalizeKeyResources(storedScopes string, requested []string) (KeyResources, error) {
	set := map[string]struct{}{}
	for _, raw := range requested {
		if raw = strings.TrimSpace(raw); raw != "" {
			set[raw] = struct{}{}
		}
	}
	if len(set) == 0 {
		return nil, nil
	}
	if len(set) > maxKeyResources {
		return nil, invalidGrant("a key is narrowed to at most %d resources; %d were asked for",
			maxKeyResources, len(set))
	}

	scopes := ScopesFromStored(storedScopes)
	if scopes.IsAdmin() {
		return nil, invalidGrant("an admin key cannot be narrowed to resources: admin is the control plane, " +
			"which no selector reaches; narrow a data-plane key such as invoke-only instead")
	}
	holder := fmt.Sprintf("a key with scopes %q", scopes.Canonical())

	narrowed := ScopeSet{}
	out := make(KeyResources, 0, len(set))
	for raw := range set {
		if strings.Contains(raw, keyResourceSeparator) {
			return nil, invalidGrant("selector %q holds a %q, which separates a key's selectors; "+
				"pass each selector on its own", raw, keyResourceSeparator)
		}
		selector, err := validSelector(raw, scopes, holder)
		if err != nil {
			return nil, err
		}
		narrowed[selector.RequiredScope()] = struct{}{}
		out = append(out, selector.String())
	}
	for _, scope := range scopes.List() {
		if !narrowed.Has(scope) {
			return nil, invalidGrant("scope %q is not narrowed by any resource, and a narrowed key reaches only "+
				"what its resources match, so it would reach nothing: drop it from the scopes or add a "+
				"resource in it", scope)
		}
	}
	sort.Strings(out)
	return out, nil
}

// Narrow is what remains of a credential's permissions once this key's
// selectors are applied: each selector's permission, where the credential
// already reaches all of it. A key with no selectors is left as it was.
//
// A selector can only narrow. Its permission is kept when the held set covers
// its pattern taken literally, so `fn:invoke:*` keeps `premium-*`, and a held
// set itself narrowed elsewhere keeps nothing it did not already reach.
func (k KeyResources) Narrow(held PermissionSet) PermissionSet {
	if len(k) == 0 {
		return held
	}
	out := PermissionSet{}
	for _, selector := range k {
		p, err := PermissionFromSelector(selector)
		if err != nil {
			// A selector this binary cannot read is not a licence.
			continue
		}
		if held.Permits(Resource{Domain: p.Domain, Action: p.Action, Name: p.Resource}) {
			out = append(out, p)
		}
	}
	return out
}
