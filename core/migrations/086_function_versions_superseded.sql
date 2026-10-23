-- =============================================================================
-- 086_function_versions_superseded.sql
--
-- Only a function's current version is invoked (bugboard #939, #940).
--
-- Every deploy kept the version before it `active`, so `name@N` of code the
-- owner had replaced stayed invocable for as long as the row was kept. A deploy
-- and a rollback now mark every other version `superseded`, and `name@N` of a
-- superseded version is refused. This brings the rows written before that in
-- line: of each function's versions that can run (active, or inactive because
-- the function is disabled), the highest stays as it is and every lower one
-- becomes superseded. Deleted rows are untouched.
--
-- A disabled function keeps its highest version inactive, so `enable` brings
-- back that version and not every version it ever had. A function deleted by a
-- release before deletes wrote `deleted` carries `inactive` on every row; it
-- comes out of this as a disabled function with one version, as before.
--
-- Replay-safe: after one run each function has at most one active or inactive
-- row, which no row is below, so a second run changes nothing.
-- =============================================================================

UPDATE functions SET status = 'superseded'
 WHERE status IN ('active', 'inactive')
   AND version < (SELECT MAX(kept.version) FROM functions kept
                   WHERE kept.namespace = functions.namespace
                     AND kept.name = functions.name
                     AND kept.status IN ('active', 'inactive'));

INSERT OR IGNORE INTO schema_migrations(version) VALUES (86);
