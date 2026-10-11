package tornet

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

const (
	// ed25519SecretTag opens tor's ed25519 secret key files, padded to ed25519SecretHeaderLen.
	ed25519SecretTag       = "== ed25519v1-secret: type0 =="
	ed25519SecretHeaderLen = 32
	// Ed25519ExpandedLen is the length of tor's expanded ed25519 secret: the clamped scalar and the nonce prefix.
	Ed25519ExpandedLen = 64
)

// RelayKeys is what a relay's DataDirectory holds that registering the relay on the chain needs.
type RelayKeys struct {
	// Fingerprint is the 20-byte SHA-1 digest of the relay's RSA identity key.
	Fingerprint []byte
	// Ed25519Expanded is tor's 64-byte expanded secret of the relay's ed25519 master identity. It is
	// secret: the caller signs with it and prints none of it.
	Ed25519Expanded []byte
	// Ed25519ID is the unpadded base64 identity tor wrote to fingerprint-ed25519, or "" when tor has
	// not written the file; the caller checks it against the key the secret derives.
	Ed25519ID string
}

// ReadRelayKeys reads the identity keys of the relay whose DataDirectory is home. The RSA key is
// the source of the fingerprint, and the fingerprint tor wrote must agree with it. A key tor has
// not generated yet (the relay has not started) is an error that says so, not an empty value.
func ReadRelayKeys(home string) (RelayKeys, error) {
	pemKey, ok, err := readIfExists(filepath.Join(home, ceremonyKeysDir, KeyRelayIdentity))
	if err != nil {
		return RelayKeys{}, err
	}
	if !ok {
		return RelayKeys{}, fmt.Errorf("%s has no relay identity key yet: the relay has not started", home)
	}
	fingerprint, err := RelayFingerprint(pemKey)
	if err != nil {
		return RelayKeys{}, err
	}
	if raw, found, err := readIfExists(filepath.Join(home, dataDirFingerprint)); err != nil {
		return RelayKeys{}, err
	} else if found {
		_, written, err := parseFingerprintFile(raw)
		if err != nil {
			return RelayKeys{}, fmt.Errorf("%s: %w", dataDirFingerprint, err)
		}
		if written != fingerprint {
			return RelayKeys{}, fmt.Errorf("%s names %s but the relay identity key is %s", dataDirFingerprint, written, fingerprint)
		}
	}
	digest, err := hex.DecodeString(fingerprint)
	if err != nil {
		return RelayKeys{}, fmt.Errorf("the relay fingerprint %q is not hex: %w", fingerprint, err)
	}
	expanded, err := readEd25519Secret(filepath.Join(home, ceremonyKeysDir, KeyEd25519Master))
	if err != nil {
		return RelayKeys{}, err
	}
	keys := RelayKeys{Fingerprint: digest, Ed25519Expanded: expanded}
	if raw, found, err := readIfExists(filepath.Join(home, dataDirFingerprintEd)); err != nil {
		return RelayKeys{}, err
	} else if found {
		_, id, _ := strings.Cut(strings.TrimSpace(string(raw)), " ")
		if err := checkEd25519ID(strings.TrimSpace(id)); err != nil {
			return RelayKeys{}, fmt.Errorf("%s: %w", dataDirFingerprintEd, err)
		}
		keys.Ed25519ID = strings.TrimSpace(id)
	}
	return keys, nil
}

// readEd25519Secret reads tor's ed25519 secret key file: a 32-byte header, then the 64-byte expanded secret.
func readEd25519Secret(path string) ([]byte, error) {
	raw, ok, err := readIfExists(path)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("the relay has no ed25519 master identity key (an offline master key is not supported: the relay's own key signs the registration)")
	}
	if len(raw) != ed25519SecretHeaderLen+Ed25519ExpandedLen || !bytes.HasPrefix(raw, []byte(ed25519SecretTag)) {
		return nil, fmt.Errorf("%s is not a tor ed25519 secret key file", path)
	}
	return append([]byte(nil), raw[ed25519SecretHeaderLen:]...), nil
}
