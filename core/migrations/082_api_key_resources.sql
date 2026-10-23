-- =============================================================================
-- 082_api_key_resources.sql
--
-- An API key narrowed to named resources.
--
--   resources  the selectors the key is narrowed to, canonical and
--              comma-separated (auth.KeyResources): `fn:premium-status`,
--              `fn:premium-*`. NULL is a key that reaches the whole of its
--              scopes, which is every key minted before this column.
--
-- It sits beside `scopes` because a key's authority is its own row and not the
-- grant recording its membership of the namespace: the lookup that reads a
-- key's scopes on every request reads this in the same statement, and caches
-- it with them.
--
-- Expand only: nullable, no backfill, so a gateway that predates the column
-- keeps inserting keys during a rolling upgrade. Replay-safe: an ADD COLUMN that
-- already happened is "duplicate column name", which the runner treats as
-- applied.
--
-- Cluster registry only.
-- =============================================================================

ALTER TABLE api_keys ADD COLUMN resources TEXT;

INSERT OR IGNORE INTO schema_migrations(version) VALUES (82);
