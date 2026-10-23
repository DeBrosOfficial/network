package update

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
	"golang.org/x/crypto/sha3"
)

// SignerAddress is the Ethereum address authorized to sign OramaOS updates.
// Updates signed by any other address are rejected.
const SignerAddress = "0xb5d8a496c8b2412990d7D467E17727fdF5954afC"

const (
	// evmSignatureSize is r (32 bytes) || s (32 bytes) || v (1 byte).
	evmSignatureSize = 65
	// evmRecoveryOffset is what personal_sign adds to the recovery id in v
	// (27 or 28); signers that omit it write 0 or 1.
	evmRecoveryOffset = 27
	// compactRecoveryOffset is what a secp256k1 compact signature adds to the
	// recovery id in its first byte, for an uncompressed public key.
	compactRecoveryOffset = 27
	// addressHashOffset is where the 20-byte address starts in the keccak256
	// of the uncompressed public key.
	addressHashOffset = 12
)

// verifySignature verifies an EVM personal_sign signature against
// SignerAddress. message is the signed text: the image's hex SHA-256, as
// VersionInfo.SHA256 carries it. signatureHex is the 65-byte hex-encoded
// signature (r || s || v).
func verifySignature(message, signatureHex string) error {
	if SignerAddress == "0x0000000000000000000000000000000000000000" {
		return fmt.Errorf("signer address not configured — refusing unsigned update")
	}
	signer, err := recoverSigner(message, signatureHex)
	if err != nil {
		return err
	}
	if !strings.EqualFold(signer, SignerAddress) {
		return fmt.Errorf("update signed by %s, expected %s", signer, SignerAddress)
	}
	return nil
}

// recoverSigner returns the Ethereum address whose key made signatureHex, an
// EVM personal_sign signature over message.
//
// The recovery is secp256k1's, from github.com/decred/dcrd/dcrec/secp256k1:
// the curve has a = 0, which the generic crypto/elliptic arithmetic (written
// for a = -3) does not compute, and v is the parity of R's y coordinate, not
// a choice between x = r and x = r + N.
func recoverSigner(message, signatureHex string) (string, error) {
	sig, err := hex.DecodeString(strings.TrimPrefix(signatureHex, "0x"))
	if err != nil {
		return "", fmt.Errorf("invalid signature hex: %w", err)
	}
	if len(sig) != evmSignatureSize {
		return "", fmt.Errorf("invalid signature length: got %d, expected %d", len(sig), evmSignatureSize)
	}
	v := sig[evmSignatureSize-1]
	if v >= evmRecoveryOffset {
		v -= evmRecoveryOffset
	}
	if v > 1 {
		return "", fmt.Errorf("invalid signature recovery id: %d", v)
	}

	compact := make([]byte, evmSignatureSize)
	compact[0] = compactRecoveryOffset + v
	copy(compact[1:], sig[:evmSignatureSize-1])
	pub, _, err := ecdsa.RecoverCompact(compact, personalSignHash(message))
	if err != nil {
		return "", fmt.Errorf("public key recovery failed: %w", err)
	}

	h := sha3.NewLegacyKeccak256()
	h.Write(pub.SerializeUncompressed()[1:]) // skip the 0x04 prefix
	return "0x" + hex.EncodeToString(h.Sum(nil)[addressHashOffset:]), nil
}

// personalSignHash computes keccak256("\x19Ethereum Signed Message:\n" + len(msg) + msg).
func personalSignHash(message string) []byte {
	prefix := fmt.Sprintf("\x19Ethereum Signed Message:\n%d", len(message))
	h := sha3.NewLegacyKeccak256()
	h.Write([]byte(prefix))
	h.Write([]byte(message))
	return h.Sum(nil)
}
