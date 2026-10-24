-- =============================================================================
-- 091_device_attestation.sql
--
-- Device attestation (feat-941): a namespace may admit to its functions and
-- data plane only the genuine build of its app on a genuine device.
--
-- namespace_attestation_policy is what a namespace requires. No row is mode
-- 'off', which is how every namespace worked before.
--
--   mode                     off | monitor (count what would be refused,
--                            refuse nothing) | enforce
--   ios_team_id,             the App ID ("TEAMID.bundle.id") whose App Attest
--   ios_bundle_id            keys are accepted
--   ios_allow_development    1 accepts the development App Attest environment
--                            and development-signed builds (a devnet
--                            namespace), 0 production only
--   ios_min_bundle_version   the oldest CFBundleVersion accepted; '' is none
--   android_package          the one package name accepted
--   android_signing_digests  JSON array of the lowercase-hex SHA-256 digests
--                            of the signing certificates accepted
--   android_allow_graphene   1 accepts a device booting GrapheneOS (a
--                            verified-boot key on its signed list) beside
--                            stock verified boot
--   android_min_version_code the oldest versionCode accepted; 0 is none
--   android_max_patch_age_months  the oldest OS security patch accepted, in
--                            months; 0 checks none
--   reattest_interval_sec    how long an Android attestation lasts before the
--                            device must attest again
--
-- device_attestations is one row per (namespace, device): the device the
-- session is bound to (the token's did), what its last attestation proved, and
-- what later ones must agree with — the iOS App Attest key and its counter,
-- and the Android verified-boot key pinned on first use.
--
-- Both are cluster registry only, beside namespace_session_policy and
-- session_devices, which they extend. Replay-safe: CREATE ... IF NOT EXISTS.
-- =============================================================================

CREATE TABLE IF NOT EXISTS namespace_attestation_policy (
    namespace_id                  INTEGER PRIMARY KEY,
    mode                          TEXT NOT NULL CHECK (mode IN ('off', 'monitor', 'enforce')),
    ios_team_id                   TEXT NOT NULL DEFAULT '',
    ios_bundle_id                 TEXT NOT NULL DEFAULT '',
    ios_allow_development         INTEGER NOT NULL DEFAULT 0,
    ios_min_bundle_version        TEXT NOT NULL DEFAULT '',
    android_package               TEXT NOT NULL DEFAULT '',
    android_signing_digests       TEXT NOT NULL DEFAULT '[]',
    android_allow_graphene        INTEGER NOT NULL DEFAULT 0,
    android_min_version_code      INTEGER NOT NULL DEFAULT 0,
    android_max_patch_age_months  INTEGER NOT NULL DEFAULT 0,
    reattest_interval_sec         INTEGER NOT NULL DEFAULT 604800,
    updated_by                    TEXT NOT NULL,
    updated_at                    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(namespace_id) REFERENCES namespaces(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS device_attestations (
    namespace_id        INTEGER NOT NULL,
    -- The RFC 7638 thumbprint of the session's device key: the token's did.
    device_id           TEXT NOT NULL,
    platform            TEXT NOT NULL CHECK (platform IN ('ios', 'android')),
    -- TeamID.bundleID on iOS, the package name on Android.
    app_id              TEXT NOT NULL,
    -- The build the platform signed: CFBundleVersion (iOS 27 and later) or
    -- versionCode; '' when it signed none (iOS 26 and older).
    app_version         TEXT NOT NULL DEFAULT '',
    -- 1 for an iOS key of the development environment.
    development         INTEGER NOT NULL DEFAULT 0,
    -- Android: the verified-boot key (hex SHA-256), pinned on the device's
    -- first attestation; a later one from another OS key is refused. 1 in
    -- grapheneos when that key is on GrapheneOS's signed list.
    verified_boot_key   TEXT NOT NULL DEFAULT '',
    grapheneos          INTEGER NOT NULL DEFAULT 0,
    -- iOS: the App Attest key id and public key (X9.62, both base64) and the
    -- counter of the last accepted assertion, which only ever rises.
    ios_key_id          TEXT NOT NULL DEFAULT '',
    ios_public_key      TEXT NOT NULL DEFAULT '',
    ios_counter         INTEGER NOT NULL DEFAULT 0,
    attested_at         TIMESTAMP NOT NULL,
    PRIMARY KEY (namespace_id, device_id),
    FOREIGN KEY(namespace_id) REFERENCES namespaces(id) ON DELETE CASCADE
);

INSERT OR IGNORE INTO schema_migrations(version) VALUES (91);
