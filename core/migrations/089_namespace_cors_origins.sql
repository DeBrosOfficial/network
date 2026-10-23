-- =============================================================================
-- 089_namespace_cors_origins.sql
--
-- The browser origins a namespace admits beyond the built-in rule (bug-742).
--
-- Every gateway lets a page read its answers when the page is served from the
-- base domain or a name under it, or from this machine (localhost, any
-- *.localhost name, a loopback address, and Tauri's webviews). An app served
-- from its own domain had no way in: the rule was fixed per gateway. A row
-- here lists that namespace's own origins, canonical (scheme://host[:port]),
-- as a JSON array of at most 20; the index gateway answers ns-<name> hosts
-- from an in-memory copy reread every 30 s.
--
-- Keyed by the namespace's id, not its name, and dropped with the namespace:
-- a namespace created again under a deleted one's name must not admit the
-- previous tenant's origins.
--
-- Cluster registry only. Replay-safe (IF NOT EXISTS).
-- =============================================================================

CREATE TABLE IF NOT EXISTS namespace_cors_origins (
    namespace_id INTEGER PRIMARY KEY,
    origins      TEXT NOT NULL,
    updated_by   TEXT NOT NULL,
    updated_at   TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(namespace_id) REFERENCES namespaces(id) ON DELETE CASCADE
);

INSERT OR IGNORE INTO schema_migrations(version) VALUES (89);
