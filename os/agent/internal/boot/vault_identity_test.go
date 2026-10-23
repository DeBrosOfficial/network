package boot

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func withVaultKeyPath(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "orama", "vault.ed25519")
	old := vaultKeyPath
	vaultKeyPath = path
	t.Cleanup(func() { vaultKeyPath = old })
	return path
}

// The identity must survive a reboot: the shares the guardians hold are
// filed under it, so a new key on every boot would orphan them.
func TestVaultIdentity_createdOnceThenReused(t *testing.T) {
	path := withVaultKeyPath(t)
	first, err := loadOrCreateVaultIdentity()
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 || info.Size() != ed25519.PrivateKeySize {
		t.Errorf("key file mode %v size %d, want 0600 and %d bytes", info.Mode().Perm(), info.Size(), ed25519.PrivateKeySize)
	}
	again, err := loadOrCreateVaultIdentity()
	if err != nil {
		t.Fatal(err)
	}
	if again.identity != first.identity || again.pubHex != first.pubHex {
		t.Errorf("identity changed across loads: %s then %s", first.identity, again.identity)
	}
}

func TestVaultIdentity_aDamagedKeyFileIsRefused(t *testing.T) {
	path := withVaultKeyPath(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("short"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadOrCreateVaultIdentity(); err == nil {
		t.Fatal("a truncated key file loaded")
	}
	// Refusing must not mean replacing: the original key is what the shares
	// are filed under.
	if data, _ := os.ReadFile(path); string(data) != "short" {
		t.Error("the damaged key file was overwritten")
	}
}

// The guardian (vault/src/auth/ownership.zig) accepts a request when
// identity == lowercase hex SHA-256 of the public key and the Ed25519
// signature over the domain-separated message verifies under that key.
func TestVaultIdentity_whatTheGuardianChecks(t *testing.T) {
	seed := make([]byte, ed25519.SeedSize)
	for i := range seed {
		seed[i] = byte(i)
	}
	id := identityFromPrivate(ed25519.NewKeyFromSeed(seed))

	pub, err := hex.DecodeString(id.pubHex)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		t.Fatalf("pubHex %q is not a hex public key", id.pubHex)
	}
	sum := sha256.Sum256(pub)
	if id.identity != hex.EncodeToString(sum[:]) || id.identity != strings.ToLower(id.identity) || len(id.identity) != 64 {
		t.Errorf("identity %s is not the lowercase hex SHA-256 of the public key", id.identity)
	}

	put := fmt.Sprintf("vault-secret-put-v1:%s:%s:%d", id.identity, "luks-share", 1)
	sig, err := hex.DecodeString(id.sign(put))
	if err != nil {
		t.Fatal(err)
	}
	if !ed25519.Verify(pub, []byte(put), sig) {
		t.Error("the signature does not verify under the identity's public key")
	}
	get := fmt.Sprintf("vault-secret-get-v1:%s:%s:%d", id.identity, "luks-share", 1)
	if ed25519.Verify(pub, []byte(get), sig) {
		t.Error("a put signature verifies as a get: the messages are not domain-separated")
	}
}
