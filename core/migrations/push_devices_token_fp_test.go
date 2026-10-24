package migrations_test

// Migration 087 clears every push_devices.token_fp so the gateway's backfill
// writes it again under the cluster-secret key (bugboard #435).

import (
	"testing"

	"github.com/Orama-Network/orama/core/migrations"
	"github.com/Orama-Network/orama/core/pkg/rqlite"
	"go.uber.org/zap"
)

func TestMigration087_clearsEveryPushTokenFingerprint(t *testing.T) {
	db := openRoundtripDB(t)
	if err := rqlite.ApplyEmbeddedMigrations(t.Context(), db, migrationsBefore(t, "087"), zap.NewNop()); err != nil {
		t.Fatalf("apply migrations before 087: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO push_devices
		(id, namespace, user_id, device_id, provider, token_encrypted, token_fp, platform, created_at, updated_at)
		VALUES ('a', 'ns', 'u', 'd1', 'apns', 'enc-a', 'fp-under-the-old-ikm', 'ios', 1, 1),
		       ('b', 'ns', 'u', 'd2', 'apns', 'enc-b', NULL, 'ios', 1, 1)`); err != nil {
		t.Fatalf("seed push devices: %v", err)
	}

	if err := rqlite.ApplyEmbeddedMigrations(t.Context(), db, migrations.FS, zap.NewNop()); err != nil {
		t.Fatalf("apply 087: %v", err)
	}

	var withFP, rows int
	if err := db.QueryRow(`SELECT COUNT(token_fp), COUNT(*) FROM push_devices`).Scan(&withFP, &rows); err != nil {
		t.Fatalf("count: %v", err)
	}
	if withFP != 0 {
		t.Errorf("%d fingerprint(s) survived 087; the backfill would never rewrite them", withFP)
	}
	if rows != 2 {
		t.Errorf("087 left %d registrations, want both: it clears fingerprints, not devices", rows)
	}
}
