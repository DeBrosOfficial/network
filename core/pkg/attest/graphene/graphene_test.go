package graphene

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha512"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"golang.org/x/crypto/ssh"
)

// The compiled-in snapshot is GrapheneOS's real list and signature
// (timestamp 1776709772), verified here with the pinned signer exactly as
// `ssh-keygen -Y verify -f allowed_signers -I contact@grapheneos.org -n
// "attestation keys"` does.
func TestVerify_realSignedSnapshot(t *testing.T) {
	l, err := Verify(PinnedSigner(), snapshotList, snapshotSig)
	if err != nil {
		t.Fatal(err)
	}
	if l.Timestamp != 1776709772 || l.Len() != 21 {
		t.Fatalf("timestamp %d, %d keys", l.Timestamp, l.Len())
	}
	pixel9, _ := hex.DecodeString("9e6a8f3e0d761a780179f93acd5721ba1ab7c8c537c7761073c0a754b0e932de")
	if !l.Contains(pixel9) {
		t.Fatal("the Pixel 9 key is not listed")
	}
	if l.Contains(make([]byte, bootKeyLen)) {
		t.Fatal("a zero key is listed")
	}
}

func TestVerify_refusesTamperingAndOtherSigners(t *testing.T) {
	tampered := []byte(strings.Replace(string(snapshotList), "9e6a8f3e", "9e6a8f3f", 1))
	if _, err := Verify(PinnedSigner(), tampered, snapshotSig); err == nil {
		t.Fatal("a tampered list verified")
	}
	k := newKey(t)
	if _, err := Verify(k.pub, snapshotList, snapshotSig); err == nil {
		t.Fatal("GrapheneOS's signature verified as another signer's")
	}
	if _, err := Verify(PinnedSigner(), snapshotList, k.sign(t, "attestation keys", snapshotList)); err == nil {
		t.Fatal("another signer's signature verified as GrapheneOS's")
	}
	if _, err := Verify(k.pub, snapshotList, k.sign(t, "file", snapshotList)); err == nil {
		t.Fatal("a signature in another namespace verified")
	}
	for name, list := range map[string]string{
		"no timestamp":  `{"verifiedBootKeys":[]}`,
		"no keys":       `{"timestamp":1}`,
		"short key":     `{"verifiedBootKeys":["abcd"],"timestamp":1}`,
		"unknown field": `{"verifiedBootKeys":[],"timestamp":1,"extra":true}`,
		"non-hex key":   `{"verifiedBootKeys":["` + strings.Repeat("zz", 32) + `"],"timestamp":1}`,
	} {
		if _, err := Verify(k.pub, []byte(list), k.sign(t, signNamespace, []byte(list))); err == nil {
			t.Errorf("%s: verified", name)
		}
	}
}

func TestRefresher_acceptsNewerListsRefusesDowngradesAndCaches(t *testing.T) {
	k := newKey(t)
	keyA, keyB := strings.Repeat("aa", 32), strings.Repeat("bb", 32)
	list := func(ts int64, key string) []byte {
		return []byte(fmt.Sprintf(`{"verifiedBootKeys":["%s"],"timestamp":%d}`, key, ts))
	}
	var served atomic.Pointer[[]byte]
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := *served.Load()
		w.Header().Set("Cache-Control", "public, max-age=1800")
		if strings.HasSuffix(r.URL.Path, ".sig") {
			w.Write(k.sign(t, signNamespace, body))
			return
		}
		w.Write(body)
	}))
	defer srv.Close()
	cache := filepath.Join(t.TempDir(), "graphene.json")
	cfg := Config{
		ListURL: srv.URL + "/attestation.json", SignatureURL: srv.URL + "/attestation.json.sig",
		Client: srv.Client(), CacheFile: cache, Signer: k.pub,
		SnapshotList: list(100, keyA), SnapshotSig: k.sign(t, signNamespace, list(100, keyA)),
	}
	r, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := hex.DecodeString(keyA)
	b, _ := hex.DecodeString(keyB)
	if !r.Contains(a) || !r.Stale() {
		t.Fatal("the snapshot is in use and nothing has been fetched yet")
	}

	newer := list(200, keyB)
	served.Store(&newer)
	r.RefreshOnce(t.Context())
	if !r.Contains(b) || r.Contains(a) || r.Stale() {
		t.Fatal("the newer list was not installed")
	}

	older := list(150, keyA)
	served.Store(&older)
	r.RefreshOnce(t.Context())
	if r.Current().Timestamp != 200 || r.Status().LastError == nil {
		t.Fatalf("a downgrade to timestamp 150 was accepted or not reported: %+v", r.Status())
	}

	restarted, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if restarted.Current().Timestamp != 200 {
		t.Fatalf("after a restart the list is %d, want the cached 200", restarted.Current().Timestamp)
	}

	cfg.SnapshotList, cfg.SnapshotSig = list(300, keyA), k.sign(t, signNamespace, list(300, keyA))
	upgraded, err := New(cfg)
	if err != nil {
		t.Fatalf("a cache older than a newer compiled-in snapshot: %v", err)
	}
	if upgraded.Current().Timestamp != 300 {
		t.Fatalf("list %d, want the newer snapshot 300", upgraded.Current().Timestamp)
	}
}

func TestNew_refusesABadSnapshot(t *testing.T) {
	k := newKey(t)
	if _, err := New(Config{Signer: k.pub}); err == nil {
		t.Fatal("GrapheneOS's snapshot verified under another signer")
	}
}

// testKey signs SSHSIGs the way `ssh-keygen -Y sign` does.
type testKey struct {
	signer ssh.Signer
	pub    ssh.PublicKey
}

func newKey(t *testing.T) testKey {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	s, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	return testKey{signer: s, pub: s.PublicKey()}
}

func (k testKey) sign(t *testing.T, namespace string, message []byte) []byte {
	t.Helper()
	h := sha512.Sum512(message)
	data := append([]byte(sshsigMagic), ssh.Marshal(signedData{Namespace: namespace, HashAlgorithm: "sha512", Hash: h[:]})...)
	sig, err := k.signer.Sign(rand.Reader, data)
	if err != nil {
		t.Fatal(err)
	}
	blob := append([]byte(sshsigMagic), ssh.Marshal(sshsigBlob{
		Version: sshsigVersion, PublicKey: k.pub.Marshal(), Namespace: namespace,
		HashAlgorithm: "sha512", Signature: ssh.Marshal(sig),
	})...)
	return pem.EncodeToMemory(&pem.Block{Type: sshsigPEMType, Bytes: blob})
}
