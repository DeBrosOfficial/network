-- =============================================================================
-- 090_namespace_end_user_topics.sql
--
-- The pub/sub topics a namespace's end users may read (bugboard #733).
--
-- An end user is a wallet that signed in to an open namespace and holds no
-- grant in it. It could subscribe to every topic of the namespace, list them
-- and see who was present on them, including topics only the application's
-- functions are meant to consume. The namespace now lists the topics its end
-- users read, as a JSON array of permission-resource patterns (`*` matches any
-- run of characters), at most 64. A namespace with no row, and a row from
-- before this migration, lists none: an end user reads no topic until the
-- owner names it. Members holding a grant are unaffected.
--
-- It lives on the row that holds the namespace's device and sign-in policies,
-- in the cluster registry, and is read where an end user's pub/sub request is
-- authorized, through a short cache.
--
-- The runner treats a repeated ADD COLUMN as applied, so a re-run changes
-- nothing.
-- =============================================================================

ALTER TABLE namespace_session_policy ADD COLUMN end_user_topics TEXT NOT NULL DEFAULT '[]';

INSERT OR IGNORE INTO schema_migrations(version) VALUES (90);
