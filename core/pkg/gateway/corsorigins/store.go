package corsorigins

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Orama-Network/orama/core/pkg/client"
	"github.com/Orama-Network/orama/core/pkg/rqlite"
)

// ErrNamespaceNotFound is a namespace the registry has no row for.
var ErrNamespaceNotFound = errors.New("no such namespace")

// Store reads and writes namespace_cors_origins in the cluster registry
// (migration 089). The row is keyed by the namespace's id, so a namespace
// deleted and created again under the same name starts with no origins.
type Store struct {
	db rqlite.Client
}

// NewStore wraps the cluster registry.
func NewStore(db rqlite.Client) *Store {
	return &Store{db: db}
}

// Origins returns the origins namespace lists, nil when it lists none.
func (s *Store) Origins(ctx context.Context, namespace string) ([]string, error) {
	var rows []struct {
		Origins string `db:"origins"`
	}
	if err := s.db.Query(client.WithInternalAuth(ctx), &rows,
		`SELECT c.origins AS origins FROM namespace_cors_origins c
		   JOIN namespaces n ON n.id = c.namespace_id
		  WHERE n.name = ? LIMIT 1`, namespace); err != nil {
		return nil, fmt.Errorf("read the allowed origins of %q: %w", namespace, err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	var origins []string
	if err := json.Unmarshal([]byte(rows[0].Origins), &origins); err != nil {
		return nil, fmt.Errorf("the allowed origins of %q are unreadable: %w", namespace, err)
	}
	return origins, nil
}

// All returns every namespace's list, keyed by namespace name.
func (s *Store) All(ctx context.Context) (map[string][]string, error) {
	var rows []struct {
		Namespace string `db:"namespace"`
		Origins   string `db:"origins"`
	}
	if err := s.db.Query(client.WithInternalAuth(ctx), &rows,
		`SELECT n.name AS namespace, c.origins AS origins FROM namespace_cors_origins c
		   JOIN namespaces n ON n.id = c.namespace_id`); err != nil {
		return nil, fmt.Errorf("read the allowed origins: %w", err)
	}
	out := make(map[string][]string, len(rows))
	for _, row := range rows {
		var origins []string
		if err := json.Unmarshal([]byte(row.Origins), &origins); err != nil {
			return nil, fmt.Errorf("the allowed origins of %q are unreadable: %w", row.Namespace, err)
		}
		out[normalizedNamespace(row.Namespace)] = origins
	}
	return out, nil
}

// Set replaces namespace's list. An empty list removes the row.
func (s *Store) Set(ctx context.Context, namespace string, origins []string, actor string) error {
	ctx = client.WithInternalAuth(ctx)
	var ids []struct {
		ID int64 `db:"id"`
	}
	if err := s.db.Query(ctx, &ids, `SELECT id FROM namespaces WHERE name = ? LIMIT 1`, namespace); err != nil {
		return fmt.Errorf("resolve namespace %q: %w", namespace, err)
	}
	if len(ids) == 0 {
		return fmt.Errorf("%w: %q", ErrNamespaceNotFound, namespace)
	}
	if len(origins) == 0 {
		if _, err := s.db.Exec(ctx, `DELETE FROM namespace_cors_origins WHERE namespace_id = ?`, ids[0].ID); err != nil {
			return fmt.Errorf("clear the allowed origins of %q: %w", namespace, err)
		}
		return nil
	}
	encoded, err := json.Marshal(origins)
	if err != nil {
		return fmt.Errorf("encode the allowed origins: %w", err)
	}
	if _, err := s.db.Exec(ctx,
		`INSERT INTO namespace_cors_origins(namespace_id, origins, updated_by, updated_at)
		 VALUES (?, ?, ?, datetime('now'))
		 ON CONFLICT(namespace_id) DO UPDATE SET
		   origins = excluded.origins,
		   updated_by = excluded.updated_by,
		   updated_at = excluded.updated_at`,
		ids[0].ID, string(encoded), actor); err != nil {
		return fmt.Errorf("record the allowed origins of %q: %w", namespace, err)
	}
	return nil
}

// Normalize validates a requested list and returns it canonical, in the order
// given and without repeats.
func Normalize(raw []string) ([]string, error) {
	seen := map[string]bool{}
	out := make([]string, 0, len(raw))
	for _, r := range raw {
		c, err := Canonical(r)
		if err != nil {
			return nil, err
		}
		if seen[c] {
			continue
		}
		seen[c] = true
		out = append(out, c)
	}
	if len(out) > MaxOriginsPerNamespace {
		return nil, fmt.Errorf("a namespace lists at most %d origins (got %d)", MaxOriginsPerNamespace, len(out))
	}
	return out, nil
}

// normalizedNamespace is how names are compared.
func normalizedNamespace(ns string) string {
	return strings.ToLower(strings.TrimSpace(ns))
}
