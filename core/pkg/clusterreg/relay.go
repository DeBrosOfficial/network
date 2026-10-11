package clusterreg

import (
	"fmt"
)

const (
	// RegisterRelayTypeURL is the Any type URL of orama.relay.v1.MsgRegisterRelay.
	RegisterRelayTypeURL = "/orama.relay.v1.MsgRegisterRelay"
	// UpdateRelayTypeURL is the Any type URL of orama.relay.v1.MsgUpdateRelay.
	UpdateRelayTypeURL = "/orama.relay.v1.MsgUpdateRelay"

	// RelayService is the service name of the node binding that holds a Tor relay's ed25519
	// identity. x/relay reads the relay's key from the binding of this name and from no other.
	RelayService = "relay"

	// RelayFingerprintLen is the SHA-1 digest length of a Tor relay's RSA identity.
	RelayFingerprintLen = 20
	// RelaySignatureLen is the length of an ed25519 signature.
	RelaySignatureLen = 64

	// relayCrossCertDomain separates the relay cross-certificate from every other ed25519 message
	// the same key signs. chain/x/relay/types/crosscert.go holds the same string, and the
	// cross-cert test in both modules pins the bytes.
	relayCrossCertDomain = "orama/relay/rsa-cross-cert/v1"
)

// RelayRegistration is MsgRegisterRelay and MsgUpdateRelay, which carry the same fields: the
// operator of NodeID registers (or rotates) the Tor relay that runs on it. RSAFingerprint is the
// relay's 20-byte RSA identity digest and Ed25519Signature the relay's ed25519 identity, the key the
// node binds under RelayService, signing RelayCrossCertMessage over the node id and the fingerprint.
type RelayRegistration struct {
	Operator         string
	NodeID           string
	RSAFingerprint   []byte
	Exit             bool
	Ed25519Signature []byte
}

// RelayCrossCertMessage is the byte string the relay's ed25519 identity signs to bind an RSA
// fingerprint to a node id: the domain, the node id and the fingerprint, each after a zero byte.
func RelayCrossCertMessage(nodeID string, rsaFingerprint []byte) []byte {
	msg := make([]byte, 0, len(relayCrossCertDomain)+1+len(nodeID)+1+len(rsaFingerprint))
	msg = append(msg, relayCrossCertDomain...)
	msg = append(msg, 0)
	msg = append(msg, nodeID...)
	msg = append(msg, 0)
	return append(msg, rsaFingerprint...)
}

// ValidateRelay checks the operator, the node id and the shapes of the fingerprint and signature.
// The chain verifies the signature itself.
func ValidateRelay(r RelayRegistration) error {
	if _, err := CanonicalAccount(r.Operator); err != nil {
		return fmt.Errorf("operator: %w", err)
	}
	if !idPattern.MatchString(r.NodeID) {
		return fmt.Errorf("node id %q must match %s", r.NodeID, idPattern.String())
	}
	if len(r.RSAFingerprint) != RelayFingerprintLen {
		return fmt.Errorf("rsa fingerprint is %d bytes, want %d", len(r.RSAFingerprint), RelayFingerprintLen)
	}
	if len(r.Ed25519Signature) != RelaySignatureLen {
		return fmt.Errorf("ed25519 signature is %d bytes, want %d", len(r.Ed25519Signature), RelaySignatureLen)
	}
	return nil
}

// EncodeRegisterRelay is the protobuf orama.relay.v1.MsgRegisterRelay. Field numbers match
// chain/x/relay/types/tx.pb.go; exit, field 4, is written only when set, as proto3 omits a false bool.
func EncodeRegisterRelay(r RelayRegistration) []byte {
	out := appendStringField(nil, 1, r.Operator)
	out = appendStringField(out, 2, r.NodeID)
	out = appendBytesField(out, 3, r.RSAFingerprint)
	if r.Exit {
		out = appendUvarintField(out, 4, 1)
	}
	return appendBytesField(out, 5, r.Ed25519Signature)
}

// EncodeUpdateRelay is the protobuf orama.relay.v1.MsgUpdateRelay, which has MsgRegisterRelay's fields.
func EncodeUpdateRelay(r RelayRegistration) []byte {
	return EncodeRegisterRelay(r)
}
