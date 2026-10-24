-- =============================================================================
-- 088_revoked_tokens_unique_jti.sql
--
-- One row per revoked token, and a namespace to count capability revocations
-- by (bug-750, bug-436).
--
-- `jti` had a plain index. The revoke paths check the list and then INSERT,
-- which is not atomic, so two concurrent revokes of one token wrote two rows,
-- and every gateway reloads every row every few seconds. A UNIQUE index lets
-- the write itself be the check (`ON CONFLICT(jti) DO NOTHING`). SQLite lets
-- any number of NULLs through a UNIQUE index, so the subject rows, which carry
-- no jti, are unaffected.
--
-- `namespace` is set on the rows a namespace's capabilities write (WebSocket
-- and fetch capabilities, named `cap:<namespace>:<id>`). A namespace may hold
-- auth.MaxLiveCapabilityRevocations of them in force at once, counted on
-- (namespace, expires_at), so one tenant looping mint and revoke cannot grow
-- the table every gateway reloads. Rows past expires_at deny nothing, are not
-- loaded, are not counted, and are pruned.
--
-- Order: the duplicates go first, keeping for each jti the row that lasts
-- longest, or the newest of equals, or the UNIQUE index cannot be built.
--
-- Expand only: a gateway that predates this keeps working during a rolling
-- upgrade (its plain INSERT of a token already revoked fails on the index,
-- which it reports, and the token is revoked either way). Replay-safe: the
-- DELETE and UPDATE are idempotent, the indexes are IF NOT EXISTS, and an ADD
-- COLUMN that already happened is "duplicate column name", which the runner
-- treats as applied.
--
-- Cluster registry only.
-- =============================================================================

DELETE FROM revoked_tokens
 WHERE jti IS NOT NULL
   AND EXISTS (
       SELECT 1 FROM revoked_tokens AS keep
        WHERE keep.jti = revoked_tokens.jti
          AND (keep.expires_at > revoked_tokens.expires_at
               OR (keep.expires_at = revoked_tokens.expires_at AND keep.id > revoked_tokens.id)));

DROP INDEX IF EXISTS idx_revoked_tokens_jti;
CREATE UNIQUE INDEX IF NOT EXISTS idx_revoked_tokens_jti_unique ON revoked_tokens(jti);

ALTER TABLE revoked_tokens ADD COLUMN namespace TEXT;

UPDATE revoked_tokens
   SET namespace = substr(jti, 5, instr(substr(jti, 5), ':') - 1)
 WHERE namespace IS NULL AND jti LIKE 'cap:%:%';

CREATE INDEX IF NOT EXISTS idx_revoked_tokens_namespace_expires ON revoked_tokens(namespace, expires_at);

INSERT OR IGNORE INTO schema_migrations(version) VALUES (88);
