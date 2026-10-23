// Package clustertrust is what a client must trust, beyond the system roots,
// to verify a cluster's certificates.
//
// A cluster's Caddy issues from the ACME directory in node.yaml (tls.acme_ca).
// Let's Encrypt production chains to roots every system trusts. Let's Encrypt
// staging chains to roots no system trusts, so on a staging cluster the CLI
// trusts them through its environment's CA file and the node hands them to
// every deployment (NodePath), so an app can call ORAMA_GATEWAY_URL.
//
// The roots ship in the binary rather than being fetched: nothing on the
// network can then change what is trusted. They are Let's Encrypt's published
// staging roots, https://letsencrypt.org/docs/staging-environment/.
package clustertrust

import (
	"crypto/sha256"
	"crypto/x509"
	_ "embed"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"sort"
	"strings"

	"github.com/Orama-Network/orama/core/pkg/constants"
)

// NodePath is where a node keeps the cluster's extra roots, written by install
// and upgrade from tls.acme_ca and absent when the system roots suffice. It is
// outside /opt/orama, which a deployment's mount namespace hides, and readable
// by every user: it holds public certificates only.
const NodePath = "/etc/orama/cluster-ca.pem"

//go:embed letsencrypt-staging-roots.pem
var stagingRootsPEM []byte

// stagingRootFingerprints are the SHA-256 fingerprints of the certificates
// stagingRootsPEM must hold, and nothing else: "(STAGING) Pretend Pear X1"
// (RSA, until 2035) and "(STAGING) Bogus Broccoli X2" (ECDSA, until 2040).
var stagingRootFingerprints = []string{
	"9b2a339fe6a3e85585c4cd75536cb8c1cf7cd603b9a64bec2521858ae48da85d",
	"e70570a989f8565aabdf7cae27abd1621872d6a3f811e3fef27e3dba02912198",
}

// StagingRoots returns the pinned Let's Encrypt staging roots, checked.
func StagingRoots() ([]byte, error) {
	if err := CheckStagingRoots(stagingRootsPEM); err != nil {
		return nil, err
	}
	return stagingRootsPEM, nil
}

// CheckStagingRoots refuses a bundle that is not exactly the pinned roots.
func CheckStagingRoots(bundle []byte) error {
	var got []string
	for rest := bundle; ; {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return fmt.Errorf("parse a staging root: %w", err)
		}
		if !cert.IsCA {
			return fmt.Errorf("staging root %q is not a CA", cert.Subject.CommonName)
		}
		sum := sha256.Sum256(cert.Raw)
		got = append(got, hex.EncodeToString(sum[:]))
	}
	sort.Strings(got)
	if strings.Join(got, ",") != strings.Join(stagingRootFingerprints, ",") {
		return fmt.Errorf("the staging roots are %v, not the pinned %v", got, stagingRootFingerprints)
	}
	return nil
}

// ExtraRoots returns the roots a client needs, beyond the system roots, to
// verify certificates issued from acmeDirectory ("" is Let's Encrypt
// production). known is false for a directory whose roots this release does
// not carry: an operator's own ACME CA, which the node cannot vouch for.
func ExtraRoots(acmeDirectory string) (roots []byte, known bool, err error) {
	switch acmeDirectory {
	case "", constants.LetsEncryptProductionACME:
		return nil, true, nil
	case constants.LetsEncryptStagingACME:
		roots, err := StagingRoots()
		return roots, true, err
	default:
		return nil, false, nil
	}
}
