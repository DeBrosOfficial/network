-- =============================================================================
-- 083_function_rate_limits.sql
--
-- A function's own invoke limits (function.yaml `rate_limit`, bug-740 and
-- feat-938). Each rule is a token bucket of `requests` refilled over
-- `window_sec` seconds:
--
--   rate_limit_caller_*  charged to each caller of the function (a wallet; a
--                        credential with no wallet by key and address; an
--                        anonymous caller by address)
--   rate_limit_ip_*      charged to each client address, whoever signed in
--
-- Zero requests is no rule, so every function deployed before these columns,
-- and every function that declares none, keeps only the gateway-wide tiers.
-- The values live on the row of the version that declared them, like every
-- other function.yaml setting: a redeploy changes them, `name@N` keeps its own.
--
-- Expand only: NOT NULL with a default, so a gateway that predates the columns
-- keeps inserting functions during a rolling upgrade. Replay-safe: an ADD
-- COLUMN that already happened is "duplicate column name", which the runner
-- treats as applied.
-- =============================================================================

ALTER TABLE functions ADD COLUMN rate_limit_caller_requests INTEGER NOT NULL DEFAULT 0;
ALTER TABLE functions ADD COLUMN rate_limit_caller_window_sec INTEGER NOT NULL DEFAULT 0;
ALTER TABLE functions ADD COLUMN rate_limit_ip_requests INTEGER NOT NULL DEFAULT 0;
ALTER TABLE functions ADD COLUMN rate_limit_ip_window_sec INTEGER NOT NULL DEFAULT 0;

INSERT OR IGNORE INTO schema_migrations(version) VALUES (83);
