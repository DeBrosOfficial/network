package graphene

import (
	"bytes"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/pem"
	"errors"
	"fmt"

	"golang.org/x/crypto/ssh"
)

// SSHSIG (OpenSSH PROTOCOL.sshsig): what `ssh-keygen -Y sign` writes and
// `ssh-keygen -Y verify` checks.
const (
	sshsigMagic   = "SSHSIG"
	sshsigVersion = 1
	sshsigPEMType = "SSH SIGNATURE"
)

// sshsigBlob is the armored signature's contents after the magic preamble.
type sshsigBlob struct {
	Version       uint32
	PublicKey     []byte
	Namespace     string
	Reserved      string
	HashAlgorithm string
	Signature     []byte
}

// signedData is what the key actually signs.
type signedData struct {
	Namespace     string
	Reserved      string
	HashAlgorithm string
	Hash          []byte
}

// verifySSHSig checks that armored is an SSHSIG by signer, in namespace,
// over message, as `ssh-keygen -Y verify -n namespace` would.
func verifySSHSig(signer ssh.PublicKey, namespace string, message, armored []byte) error {
	block, rest := pem.Decode(armored)
	if block == nil || block.Type != sshsigPEMType || len(bytes.TrimSpace(rest)) != 0 {
		return errors.New("signature is not one armored SSH SIGNATURE")
	}
	raw, ok := bytes.CutPrefix(block.Bytes, []byte(sshsigMagic))
	if !ok {
		return errors.New("signature lacks the SSHSIG preamble")
	}
	var blob sshsigBlob
	if err := ssh.Unmarshal(raw, &blob); err != nil {
		return fmt.Errorf("signature: %w", err)
	}
	if blob.Version != sshsigVersion {
		return fmt.Errorf("SSHSIG version %d, want %d", blob.Version, sshsigVersion)
	}
	if !bytes.Equal(blob.PublicKey, signer.Marshal()) {
		return errors.New("signed by another key than GrapheneOS's")
	}
	if blob.Namespace != namespace {
		return fmt.Errorf("signature namespace %q, want %q", blob.Namespace, namespace)
	}
	var digest []byte
	switch blob.HashAlgorithm {
	case "sha512":
		h := sha512.Sum512(message)
		digest = h[:]
	case "sha256":
		h := sha256.Sum256(message)
		digest = h[:]
	default:
		return fmt.Errorf("signature hash algorithm %q", blob.HashAlgorithm)
	}
	var sig ssh.Signature
	if err := ssh.Unmarshal(blob.Signature, &sig); err != nil {
		return fmt.Errorf("signature: %w", err)
	}
	data := append([]byte(sshsigMagic), ssh.Marshal(signedData{
		Namespace: blob.Namespace, Reserved: blob.Reserved, HashAlgorithm: blob.HashAlgorithm, Hash: digest,
	})...)
	if err := signer.Verify(data, &sig); err != nil {
		return fmt.Errorf("signature does not verify: %w", err)
	}
	return nil
}
