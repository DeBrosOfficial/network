-- =============================================================================
-- 087_push_devices_token_fp_cluster_key.sql
--
-- push_devices.token_fp is re-keyed from the cluster secret (bugboard #435).
--
-- The fingerprint key was derived from the encryption root's current IKM. A
-- secrets rotate replaces that IKM and re-encrypts token_encrypted, but it
-- cannot recompute a fingerprint, so after a rotate every gateway that
-- restarted fingerprinted tokens under a different key than the rows already
-- stored. Token-exclusive eviction (migration 033) and dedup stopped matching
-- them. The key now comes from the cluster secret, which a rotate does not
-- change, as push_topics.token_fp's already does (migration 059).
--
-- A stored fingerprint does not say which key made it, so every one is cleared
-- here. The gateway's post-schema step BackfillTokenFP then decrypts each token
-- and writes its fingerprint under the cluster-secret key. A registration made
-- before the backfill reaches its row writes its own fingerprint as usual.
-- =============================================================================

UPDATE push_devices SET token_fp = NULL WHERE token_fp IS NOT NULL;
