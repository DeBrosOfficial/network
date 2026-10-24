-- =============================================================================
-- 093_function_wasm_sha256.sql
--
-- A function version names the SHA-256 of the WASM it was deployed with
-- (bugboard chg-956).
--
-- The gateway seals every blob before it adds it to IPFS (AES-256-GCM under a
-- random nonce), so `wasm_cid` is the CID of a ciphertext: it changes on every
-- deploy of the same bytes and no local build reproduces it. `wasm_sha256` is
-- the lowercase hex SHA-256 of the bytes the gateway received, before sealing,
-- so an owner can compare it with `shasum -a 256 function.wasm`.
--
-- Nullable, with no backfill: a row deployed before this column has NULL, which
-- the API returns as null. Filling it would mean fetching and unsealing every
-- retained version's blob from IPFS in the gateway, which a SQL migration
-- cannot do; a redeploy records the hash. The CHECK keeps the column to NULL
-- or a 64-character lowercase hex digest, never an empty string.
--
-- The runner treats a repeated ADD COLUMN as applied, so a re-run changes
-- nothing.
-- =============================================================================

ALTER TABLE functions ADD COLUMN wasm_sha256 TEXT CHECK (wasm_sha256 IS NULL OR (length(wasm_sha256) = 64 AND wasm_sha256 NOT GLOB '*[^0-9a-f]*'));

INSERT OR IGNORE INTO schema_migrations(version) VALUES (93);
