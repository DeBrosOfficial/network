package globalbind

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/DeBrosOfficial/network/pkg/clusterreg"
)

// RelayIdentity is what registering a Tor relay on the chain needs from the relay's machine: the
// RSA fingerprint, the binding that makes the relay's ed25519 master identity a service key of the
// operator's node (service "relay"), and the cross-certificate that ties the fingerprint to the
// node. It carries signatures and public keys only; the secrets stay in the relay's DataDirectory.
type RelayIdentity struct {
	// Fingerprint is the 20-byte SHA-1 digest of the relay's RSA identity key.
	Fingerprint []byte
	// Binding is the ed25519 binding of the relay's master identity for the operator on a chain.
	Binding Binding
	// CrossCert is the relay's ed25519 signature over clusterreg.RelayCrossCertMessage.
	CrossCert []byte
}

// relayIdentityDoc is RelayIdentity as the node prints it: all binary values in hex.
type relayIdentityDoc struct {
	RSAFingerprint string `json:"rsa_fingerprint"`
	Service        string `json:"service"`
	KeyType        string `json:"key_type"`
	Pubkey         string `json:"pubkey"`
	Signature      string `json:"signature"`
	CrossCert      string `json:"cross_cert"`
}

// NewRelayIdentity signs, with tor's expanded ed25519 secret, the relay binding for operator on
// chainID and the cross-certificate of fingerprint for nodeID.
func NewRelayIdentity(expanded, fingerprint []byte, chainID, operator, nodeID string) (RelayIdentity, error) {
	binding, err := SignExpandedEd25519(expanded, chainID, operator, clusterreg.RelayService)
	if err != nil {
		return RelayIdentity{}, err
	}
	cert, err := SignRelayCrossCert(expanded, nodeID, fingerprint)
	if err != nil {
		return RelayIdentity{}, err
	}
	return RelayIdentity{Fingerprint: append([]byte(nil), fingerprint...), Binding: binding, CrossCert: cert}, nil
}

// MarshalJSON prints the identity in the form ParseRelayIdentity reads.
func (r RelayIdentity) MarshalJSON() ([]byte, error) {
	return json.Marshal(relayIdentityDoc{
		RSAFingerprint: strings.ToUpper(hex.EncodeToString(r.Fingerprint)),
		Service:        r.Binding.Service,
		KeyType:        r.Binding.KeyType,
		Pubkey:         hex.EncodeToString(r.Binding.Pubkey),
		Signature:      hex.EncodeToString(r.Binding.Signature),
		CrossCert:      hex.EncodeToString(r.CrossCert),
	})
}

// ParseRelayIdentity reads what a relay's machine printed and checks it for the operator on chainID
// and the node nodeID, before anything is signed or sent: the binding must be the relay service's
// ed25519 key, its signature must verify, and the cross-certificate must verify under that same
// key for this node and fingerprint. A value the chain would refuse is refused here.
func ParseRelayIdentity(raw []byte, chainID, operator, nodeID string) (RelayIdentity, error) {
	var doc relayIdentityDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return RelayIdentity{}, fmt.Errorf("the relay identity is not JSON: %w", err)
	}
	fingerprint, err := hex.DecodeString(doc.RSAFingerprint)
	if err != nil || len(fingerprint) != clusterreg.RelayFingerprintLen || doc.RSAFingerprint != strings.ToUpper(doc.RSAFingerprint) {
		return RelayIdentity{}, fmt.Errorf("the relay fingerprint %q is not %d bytes of upper-case hex", doc.RSAFingerprint, clusterreg.RelayFingerprintLen)
	}
	pub, err := hex.DecodeString(doc.Pubkey)
	if err != nil {
		return RelayIdentity{}, fmt.Errorf("the relay key is not hex: %w", err)
	}
	sig, err := hex.DecodeString(doc.Signature)
	if err != nil {
		return RelayIdentity{}, fmt.Errorf("the relay binding's signature is not hex: %w", err)
	}
	cert, err := hex.DecodeString(doc.CrossCert)
	if err != nil {
		return RelayIdentity{}, fmt.Errorf("the relay cross-certificate is not hex: %w", err)
	}
	if doc.Service != clusterreg.RelayService || doc.KeyType != KeyTypeEd25519 {
		return RelayIdentity{}, fmt.Errorf("the relay binding is for %s key of service %q, not an ed25519 key of service %q", doc.KeyType, doc.Service, clusterreg.RelayService)
	}
	binding := Binding{Service: doc.Service, KeyType: doc.KeyType, Pubkey: pub, Signature: sig}
	if err := Verify(binding, chainID, operator); err != nil {
		return RelayIdentity{}, fmt.Errorf("the relay binding for operator %s on chain %s: %w", operator, chainID, err)
	}
	if len(cert) != clusterreg.RelaySignatureLen || !ed25519.Verify(pub, clusterreg.RelayCrossCertMessage(nodeID, fingerprint), cert) {
		return RelayIdentity{}, fmt.Errorf("the relay cross-certificate of node %q does not verify under the relay's key", nodeID)
	}
	return RelayIdentity{Fingerprint: fingerprint, Binding: binding, CrossCert: cert}, nil
}
