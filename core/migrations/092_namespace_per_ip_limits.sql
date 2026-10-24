-- =============================================================================
-- 092_namespace_per_ip_limits.sql
--
-- Per-network ceilings a namespace turns on (bugboard #749).
--
-- With open sign-in a wallet costs nothing, so one host can hold as many
-- identities as it likes: the per-wallet invoke limit multiplies by their
-- number, and every new wallet is a new end user of the application. A
-- namespace may now bound what one client network does whatever identities it
-- presents:
--
--   per_ip_limits            'off' (the default) or 'on'.
--   new_wallets_per_ip_hour  when on, how many wallets that never held a
--                            session in the namespace one client network may
--                            sign in as end users per hour (default 20).
--   invokes_per_ip_minute    when on, how many authenticated invokes one
--                            client network may make per minute on a gateway
--                            (default 1200, twice one wallet's limit).
--
-- Off by default: a default ceiling would refuse offices and carrier NAT that
-- put many people behind one address. The counts themselves are not here: new
-- sign-ins are counted in Olric, invokes in each gateway's limiter.
--
-- The runner treats a repeated ADD COLUMN as applied, so a re-run changes
-- nothing.
-- =============================================================================

ALTER TABLE namespace_session_policy ADD COLUMN per_ip_limits TEXT NOT NULL DEFAULT 'off' CHECK (per_ip_limits IN ('off','on'));
ALTER TABLE namespace_session_policy ADD COLUMN new_wallets_per_ip_hour INTEGER NOT NULL DEFAULT 20;
ALTER TABLE namespace_session_policy ADD COLUMN invokes_per_ip_minute INTEGER NOT NULL DEFAULT 1200;

INSERT OR IGNORE INTO schema_migrations(version) VALUES (92);
