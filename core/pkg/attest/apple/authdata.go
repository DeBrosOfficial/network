package apple

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/fxamacker/cbor/v2"
)

// Authenticator data layout (WebAuthn §6.1, as App Attest fills it).
const (
	rpIDHashLen   = sha256.Size
	headerLen     = rpIDHashLen + 1 + 4 // RP ID hash, flags, counter
	aaguidLen     = 16
	credIDLenSize = 2
	// flagAttested (AT) marks attested credential data; flagExtensions (ED)
	// marks an extensions map.
	flagAttested   = 0x40
	flagExtensions = 0x80
	// maxDecodeNesting bounds CBOR nesting; nothing App Attest sends nests
	// deeper than the attestation object's x5c array inside attStmt.
	maxDecodeNesting = 4
	// maxDecodeItems bounds CBOR array and map sizes; App Attest's largest
	// is the x5c array (leaf and one intermediate).
	maxDecodeItems = 16
)

// Extension keys iOS 27 and later sign into the authenticator data.
const (
	extBundleVersion      = "apple_bundle_version_01"
	extValidationCategory = "apple_validation_category_01"
)

var (
	aaguidProduction  = [aaguidLen]byte{'a', 'p', 'p', 'a', 't', 't', 'e', 's', 't'}
	aaguidDevelopment = [aaguidLen]byte{'a', 'p', 'p', 'a', 't', 't', 'e', 's', 't', 'd', 'e', 'v', 'e', 'l', 'o', 'p'}
)

// strict decodes CBOR the way App Attest must have encoded it: definite
// lengths only, no tags, no duplicate map keys, valid UTF-8, bounded size.
var strict = func() cbor.DecMode {
	dm, err := cbor.DecOptions{
		DupMapKey:        cbor.DupMapKeyEnforcedAPF,
		IndefLength:      cbor.IndefLengthForbidden,
		TagsMd:           cbor.TagsForbidden,
		MaxNestedLevels:  maxDecodeNesting,
		MaxArrayElements: maxDecodeItems,
		MaxMapPairs:      maxDecodeItems,
		UTF8:             cbor.UTF8RejectInvalid,
		IntDec:           cbor.IntDecConvertNone,
	}.DecMode()
	if err != nil {
		panic(fmt.Sprintf("apple: strict CBOR options: %v", err))
	}
	return dm
}()

// Extensions are the values iOS 27 and later sign into the authenticator
// data. Both are zero on an older iOS, which signs none.
type Extensions struct {
	// BundleVersion is the build's CFBundleVersion.
	BundleVersion string
	// ValidationCategory is the build's launch validation category
	// (Apple's LightweightCodeRequirements.ValidationCategory).
	ValidationCategory uint32
}

// authData is parsed authenticator data. The credential fields are set only
// for an attestation's.
type authData struct {
	rpIDHash     [rpIDHashLen]byte
	flags        byte
	counter      uint32
	aaguid       [aaguidLen]byte
	credentialID []byte
	coseKey      []byte
	ext          Extensions
}

// parseAttestationAuthData parses an attestation's authenticator data: the
// header, the attested credential data, then any extensions map.
func parseAttestationAuthData(b []byte) (authData, error) {
	var a authData
	rest, err := a.parseHeader(b)
	if err != nil {
		return a, err
	}
	if a.flags&flagAttested == 0 {
		return a, errors.New("authenticator data has no attested credential data")
	}
	if len(rest) < aaguidLen+credIDLenSize {
		return a, errors.New("authenticator data ends inside the attested credential data")
	}
	copy(a.aaguid[:], rest)
	rest = rest[aaguidLen:]
	n := int(binary.BigEndian.Uint16(rest))
	rest = rest[credIDLenSize:]
	if n == 0 || len(rest) < n {
		return a, fmt.Errorf("credential id length %d does not fit the authenticator data", n)
	}
	a.credentialID, rest = rest[:n], rest[n:]
	var key cbor.RawMessage
	rest, err = strict.UnmarshalFirst(rest, &key)
	if err != nil {
		return a, fmt.Errorf("credential public key: %w", err)
	}
	a.coseKey = key
	a.ext, err = parseExtensions(a.flags, rest)
	return a, err
}

// parseAssertionAuthData parses an assertion's authenticator data: the
// header, then any extensions map. App Attest sets the AT flag on assertions
// without sending attested credential data, so the flag is not read here.
func parseAssertionAuthData(b []byte) (authData, error) {
	var a authData
	rest, err := a.parseHeader(b)
	if err != nil {
		return a, err
	}
	a.ext, err = parseExtensions(a.flags, rest)
	return a, err
}

func (a *authData) parseHeader(b []byte) ([]byte, error) {
	if len(b) < headerLen {
		return nil, fmt.Errorf("authenticator data is %d bytes, shorter than its %d-byte header", len(b), headerLen)
	}
	copy(a.rpIDHash[:], b)
	a.flags = b[rpIDHashLen]
	a.counter = binary.BigEndian.Uint32(b[rpIDHashLen+1:])
	return b[headerLen:], nil
}

// parseExtensions reads the extensions map that may close the
// authenticator data. Apple documents the map but not the ED flag, and iOS
// 27 builds have been seen both with and without ED set, so what decides is
// whether bytes remain: they must be exactly one CBOR map. ED set with no
// map is refused. Keys other than Apple's two are ignored, so a key Apple
// adds later does not refuse genuine devices; Apple's two keys must have the
// types Apple documents (UInt32 and String; see decodeUInt32).
func parseExtensions(flags byte, rest []byte) (Extensions, error) {
	var ext Extensions
	if len(rest) == 0 {
		if flags&flagExtensions != 0 {
			return ext, errors.New("authenticator data flags extensions but carries none")
		}
		return ext, nil
	}
	var m map[string]cbor.RawMessage
	trailing, err := strict.UnmarshalFirst(rest, &m)
	if err != nil {
		return ext, fmt.Errorf("extensions: %w", err)
	}
	if len(trailing) != 0 {
		return ext, fmt.Errorf("%d bytes follow the extensions map", len(trailing))
	}
	if raw, ok := m[extBundleVersion]; ok {
		if err := strict.Unmarshal(raw, &ext.BundleVersion); err != nil {
			return ext, fmt.Errorf("extension %s: %w", extBundleVersion, err)
		}
		if ext.BundleVersion == "" {
			return ext, fmt.Errorf("extension %s is empty", extBundleVersion)
		}
	}
	if raw, ok := m[extValidationCategory]; ok {
		c, err := decodeUInt32(raw)
		if err != nil {
			return ext, fmt.Errorf("extension %s: %w", extValidationCategory, err)
		}
		ext.ValidationCategory = c
	}
	return ext, nil
}

// decodeUInt32 reads a UInt32 extension value. Apple documents the value as
// a UInt32 but its own sample attestation (the Attestation Object Validation
// Guide) encodes it as a 4-byte little-endian byte string, so both encodings
// are read; any other is refused.
func decodeUInt32(raw cbor.RawMessage) (uint32, error) {
	var n uint32
	if err := strict.Unmarshal(raw, &n); err == nil {
		return n, nil
	}
	var b []byte
	if err := strict.Unmarshal(raw, &b); err != nil {
		return 0, errors.New("is neither an unsigned integer nor a byte string")
	}
	if len(b) != 4 {
		return 0, fmt.Errorf("is a %d-byte string, want 4", len(b))
	}
	return binary.LittleEndian.Uint32(b), nil
}

// coseEC2Key is a COSE_Key (RFC 9052 §7, RFC 9053 §7.1.1) for an EC2 key.
type coseEC2Key struct {
	Kty   int    `cbor:"1,keyasint"`
	Alg   int    `cbor:"3,keyasint"`
	Curve int    `cbor:"-1,keyasint"`
	X     []byte `cbor:"-2,keyasint"`
	Y     []byte `cbor:"-3,keyasint"`
}

// COSE values for an ES256 key on P-256.
const (
	coseKtyEC2   = 2
	coseAlgES256 = -7
	coseCrvP256  = 1
	p256CoordLen = 32
)

// coseKeyMatches reports whether a COSE key is the P-256 key whose X9.62
// uncompressed point is point.
func coseKeyMatches(raw []byte, point []byte) error {
	var k coseEC2Key
	if err := strict.Unmarshal(raw, &k); err != nil {
		return fmt.Errorf("credential public key: %w", err)
	}
	if k.Kty != coseKtyEC2 || k.Alg != coseAlgES256 || k.Curve != coseCrvP256 {
		return fmt.Errorf("credential public key is kty %d alg %d crv %d, not an ES256 P-256 key", k.Kty, k.Alg, k.Curve)
	}
	if len(k.X) != p256CoordLen || len(k.Y) != p256CoordLen {
		return errors.New("credential public key coordinates are not 32 bytes")
	}
	want := make([]byte, 0, 1+2*p256CoordLen)
	want = append(want, 0x04)
	want = append(append(want, k.X...), k.Y...)
	if !bytes.Equal(want, point) {
		return errors.New("credential public key differs from the certificate's")
	}
	return nil
}
