package gateway

import (
	"context"
	"strings"
)

func (g *Gateway) resolveNamespaceID(ctx context.Context, ns string) (interface{}, error) {
	db := g.client.Database()
	if _, err := db.Query(ctx, "INSERT OR IGNORE INTO namespaces(name) VALUES (?)", ns); err != nil {
		return nil, err
	}
	res, err := db.Query(ctx, "SELECT id FROM namespaces WHERE name = ? LIMIT 1", ns)
	if err != nil || res == nil || res.Count == 0 || len(res.Rows) == 0 || len(res.Rows[0]) == 0 {
		return nil, err
	}
	return res.Rows[0][0], nil
}

func (g *Gateway) seedConfiguredAPIKeys(ctx context.Context) error {
	db := g.client.Database()
	for key, nsOverride := range g.cfg.APIKeys {
		ns := strings.TrimSpace(nsOverride)
		if ns == "" {
			ns = strings.TrimSpace(g.cfg.ClientNamespace)
			if ns == "" {
				ns = "default"
			}
		}

		// Ensure namespace exists
		if _, err := db.Query(ctx, "INSERT OR IGNORE INTO namespaces(name) VALUES (?)", ns); err != nil {
			return err
		}
		// Lookup namespace id
		nres, err := db.Query(ctx, "SELECT id FROM namespaces WHERE name = ? LIMIT 1", ns)
		if err != nil {
			return err
		}
		var nsID interface{}
		if nres != nil && nres.Count > 0 && len(nres.Rows) > 0 && len(nres.Rows[0]) > 0 {
			nsID = nres.Rows[0][0]
		} else {
			// Should not happen, but guard
			continue
		}

		// Upsert API key
		if _, err := db.Query(ctx, "INSERT OR IGNORE INTO api_keys(key, name, namespace_id) VALUES (?, ?, ?)", key, "", nsID); err != nil {
			return err
		}
		// Record namespace ownership for API key (best-effort)
		_, _ = db.Query(ctx, "INSERT OR IGNORE INTO namespace_ownership(namespace_id, owner_type, owner_id) VALUES (?, 'api_key', ?)", nsID, key)
	}
	return nil
}
