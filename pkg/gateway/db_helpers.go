package gateway

import (
    "context"
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

// Deprecated: seeding API keys from config is removed.
