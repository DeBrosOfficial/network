-- =============================================================================
-- 084_app_runtime_keys_drop_storage.sql
--
-- A key minted from the app-runtime profile loses the storage grant
-- (bugboard #946).
--
-- app-runtime is the profile for the key an app ships, and it held storage. A
-- token exchanged for such a key may unpin (bugboard #151, the userless reclaim
-- a server-side function needs), and the unpin handler reaches every object
-- the namespace owns, so anyone who pulled the key out of an app could empty
-- the namespace's storage. The profile holds no storage from this release on;
-- this takes it off the keys already minted from it.
--
-- Profiles are expanded when a key is minted, so a row records the grants and
-- not the profile. The profile's canonical set is the one written by every
-- app-runtime mint, and that exact set is what this matches. A key minted with
-- an explicit grant list, `storage` alone included, is a different set and
-- keeps what it has: the storage key a function holds in the namespace's
-- secrets still unpins. A key whose explicit list happened to equal the
-- profile's is indistinguishable from it and is treated as what it says it is.
--
-- Revoked keys are left alone: they authenticate nothing, and rewriting them
-- would destroy the record of what they could do. A gateway reads a key's row
-- again once its cached copy is CredentialStaleness old, so the change reaches
-- every gateway within that bound of this applying.
--
-- A gateway from before this release still mints app-runtime with storage
-- during a rolling upgrade; a key minted in that window is outside this
-- statement. Re-mint any app-runtime key created while the upgrade ran.
--
-- Replay-safe: on a second apply no live row holds the old set.
--
-- Cluster registry only.
-- =============================================================================

UPDATE api_keys
   SET scopes = 'invoke,proxy,push,webrtc'
 WHERE revoked_at IS NULL
   AND scopes = 'invoke,proxy,push,storage,webrtc';

INSERT OR IGNORE INTO schema_migrations(version) VALUES (84);
