package update

import (
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
	"golang.org/x/crypto/sha3"
)

// signScript is the release signer, os/scripts/sign.sh.
var signScript = filepath.Join("..", "..", "..", "scripts", "sign.sh")

// evmPersonalSign signs message as `rw sign --chain evm` does: EIP-191
// personal_sign, r || s || v with v = 27 + recovery id.
func evmPersonalSign(key *secp256k1.PrivateKey, message string) string {
	compact := ecdsa.SignCompact(key, personalSignHash(message), false)
	sig := append(append([]byte{}, compact[1:]...), compact[0])
	return "0x" + hex.EncodeToString(sig)
}

// The message sign.sh signs is the one the agent verifies: the SHA-256 of
// the payload it downloads, which it also checks the bytes against. A
// signature made over the script's message therefore recovers to the signer
// in the agent's code. The script once signed the hash of the checksum file
// instead, and every update it signed was refused.
func TestSignScript_signsWhatTheAgentVerifies(t *testing.T) {
	prefix := filepath.Join(t.TempDir(), "orama-os-1.2.3-amd64")
	payload := prefix + ".rootfs.squashfs"
	if err := os.WriteFile(payload, []byte("squashfs payload bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("bash", signScript, "--message", prefix).CombinedOutput()
	if err != nil {
		t.Fatalf("sign.sh --message: %v\n%s", err, out)
	}
	message := strings.TrimSpace(string(out))

	if err := verifyChecksum(payload, message); err != nil {
		t.Fatalf("the agent's checksum check refuses the message sign.sh signs: %v", err)
	}

	key, err := secp256k1.GeneratePrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	h := sha3.NewLegacyKeccak256()
	h.Write(key.PubKey().SerializeUncompressed()[1:])
	signer := "0x" + hex.EncodeToString(h.Sum(nil)[addressHashOffset:])

	sig := evmPersonalSign(key, message)
	got, err := recoverSigner(message, sig)
	if err != nil || got != signer {
		t.Fatalf("a signature over sign.sh's message recovers to %s (%v), want its signer %s", got, err, signer)
	}
	if other, err := recoverSigner(sha256Hex([]byte("another payload")), sig); err == nil && other == signer {
		t.Fatal("a signature over sign.sh's message verified over another digest")
	}
}

func TestSignScript_refusesAMissingPayload(t *testing.T) {
	prefix := filepath.Join(t.TempDir(), "orama-os-1.2.3-amd64")
	if out, err := exec.Command("bash", signScript, "--message", prefix).CombinedOutput(); err == nil {
		t.Fatalf("sign.sh produced a message with no payload: %s", out)
	}
}
