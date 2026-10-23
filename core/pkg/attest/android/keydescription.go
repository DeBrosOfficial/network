package android

import (
	"errors"
	"fmt"

	"golang.org/x/crypto/cryptobyte"
	cbasn1 "golang.org/x/crypto/cryptobyte/asn1"
)

// AuthorizationList tags this package reads (KeyMint Tag.aidl, with the type
// bits masked off). Every other tag is skipped.
const (
	tagRootOfTrust              = 704
	tagOSVersion                = 705
	tagOSPatchLevel             = 706
	tagAttestationApplicationID = 709
	tagVendorPatchLevel         = 718
	tagBootPatchLevel           = 719
)

// keyDescriptionFields is the number of fields in KeyDescription, the same
// in every attestation version.
const keyDescriptionFields = 8

// SecurityLevel is where a key lives (KeyDescription's SecurityLevel).
type SecurityLevel int

const (
	SecuritySoftware  SecurityLevel = 0
	SecurityTEE       SecurityLevel = 1
	SecurityStrongBox SecurityLevel = 2
)

// BootState is RootOfTrust's VerifiedBootState.
type BootState int

const (
	BootVerified   BootState = 0
	BootSelfSigned BootState = 1
	BootUnverified BootState = 2
	BootFailed     BootState = 3
)

// RootOfTrust is the device's verified-boot state, as the secure hardware
// attests it.
type RootOfTrust struct {
	VerifiedBootKey   []byte
	DeviceLocked      bool
	VerifiedBootState BootState
	VerifiedBootHash  []byte
}

// PackageInfo is one AttestationPackageInfo.
type PackageInfo struct {
	Name    string
	Version int64
}

// ApplicationID is AttestationApplicationId: the packages sharing the key's
// UID and the digests of the certificates they are signed with.
type ApplicationID struct {
	Packages         []PackageInfo
	SignatureDigests [][]byte
}

// AuthorizationList holds the fields of an AuthorizationList this package
// reads. A zero field was absent.
type AuthorizationList struct {
	RootOfTrust      *RootOfTrust
	OSVersion        int64
	OSPatchLevel     int64
	ApplicationID    *ApplicationID
	VendorPatchLevel int64
	BootPatchLevel   int64
}

// KeyDescription is the key attestation extension (OID
// 1.3.6.1.4.1.11129.2.1.17), as AOSP's "Key and ID attestation" schema
// defines it.
type KeyDescription struct {
	AttestationVersion       int64
	AttestationSecurityLevel SecurityLevel
	KeyMintVersion           int64
	KeyMintSecurityLevel     SecurityLevel
	AttestationChallenge     []byte
	UniqueID                 []byte
	SoftwareEnforced         AuthorizationList
	HardwareEnforced         AuthorizationList
}

// ParseKeyDescription parses the DER of the key attestation extension. It
// is strict: DER only, nothing trailing, AuthorizationList tags in strictly
// ascending order (as KeyMint emits them), each read field well-formed.
func ParseKeyDescription(der []byte) (*KeyDescription, error) {
	in := cryptobyte.String(der)
	var seq cryptobyte.String
	if !in.ReadASN1(&seq, cbasn1.SEQUENCE) || !in.Empty() {
		return nil, errors.New("KeyDescription is not one DER SEQUENCE")
	}
	var kd KeyDescription
	var attLevel, kmLevel int
	var sw, hw cryptobyte.String
	if !seq.ReadASN1Int64WithTag(&kd.AttestationVersion, cbasn1.INTEGER) ||
		!seq.ReadASN1Enum(&attLevel) ||
		!seq.ReadASN1Int64WithTag(&kd.KeyMintVersion, cbasn1.INTEGER) ||
		!seq.ReadASN1Enum(&kmLevel) ||
		!seq.ReadASN1Bytes(&kd.AttestationChallenge, cbasn1.OCTET_STRING) ||
		!seq.ReadASN1Bytes(&kd.UniqueID, cbasn1.OCTET_STRING) ||
		!seq.ReadASN1(&sw, cbasn1.SEQUENCE) ||
		!seq.ReadASN1(&hw, cbasn1.SEQUENCE) ||
		!seq.Empty() {
		return nil, fmt.Errorf("KeyDescription does not have its %d fields", keyDescriptionFields)
	}
	var err error
	if kd.AttestationSecurityLevel, err = securityLevel(attLevel); err != nil {
		return nil, fmt.Errorf("attestationSecurityLevel: %w", err)
	}
	if kd.KeyMintSecurityLevel, err = securityLevel(kmLevel); err != nil {
		return nil, fmt.Errorf("keyMintSecurityLevel: %w", err)
	}
	if kd.SoftwareEnforced, err = parseAuthorizationList(sw); err != nil {
		return nil, fmt.Errorf("softwareEnforced: %w", err)
	}
	if kd.HardwareEnforced, err = parseAuthorizationList(hw); err != nil {
		return nil, fmt.Errorf("hardwareEnforced: %w", err)
	}
	return &kd, nil
}

func securityLevel(v int) (SecurityLevel, error) {
	switch l := SecurityLevel(v); l {
	case SecuritySoftware, SecurityTEE, SecurityStrongBox:
		return l, nil
	default:
		return 0, fmt.Errorf("unknown security level %d", v)
	}
}

func parseAuthorizationList(s cryptobyte.String) (AuthorizationList, error) {
	var al AuthorizationList
	last := -1
	for !s.Empty() {
		tag, body, err := readExplicit(&s)
		if err != nil {
			return al, err
		}
		if int(tag) <= last {
			return al, fmt.Errorf("tag [%d] after [%d]: tags are not in ascending order", tag, last)
		}
		last = int(tag)
		switch tag {
		case tagRootOfTrust:
			al.RootOfTrust, err = parseRootOfTrust(body)
		case tagOSVersion:
			al.OSVersion, err = readOnlyInt(body)
		case tagOSPatchLevel:
			al.OSPatchLevel, err = readOnlyInt(body)
		case tagAttestationApplicationID:
			al.ApplicationID, err = parseApplicationID(body)
		case tagVendorPatchLevel:
			al.VendorPatchLevel, err = readOnlyInt(body)
		case tagBootPatchLevel:
			al.BootPatchLevel, err = readOnlyInt(body)
		}
		if err != nil {
			return al, fmt.Errorf("tag [%d]: %w", tag, err)
		}
	}
	return al, nil
}

// readExplicit reads one context-specific, constructed ([n] EXPLICIT)
// element and returns n and its contents. KeyMint tags run past 30, so the
// identifier may use the high-tag-number form, which cryptobyte does not
// read; the length is read with DER's rules.
func readExplicit(s *cryptobyte.String) (uint32, cryptobyte.String, error) {
	var id uint8
	if !s.ReadUint8(&id) {
		return 0, nil, errors.New("truncated identifier")
	}
	if id&0xe0 != 0xa0 {
		return 0, nil, fmt.Errorf("identifier 0x%02x is not context-specific and constructed", id)
	}
	tag := uint32(id & 0x1f)
	if tag == 0x1f {
		tag = 0
		for i := 0; ; i++ {
			var b uint8
			// Five base-128 digits hold any uint32; KeyMint tags fit in two.
			if i == 5 || !s.ReadUint8(&b) {
				return 0, nil, errors.New("bad high-tag-number identifier")
			}
			if i == 0 && b == 0x80 {
				return 0, nil, errors.New("high-tag-number identifier is not minimal")
			}
			tag = tag<<7 | uint32(b&0x7f)
			if b&0x80 == 0 {
				break
			}
		}
		if tag < 0x1f {
			return 0, nil, errors.New("high-tag-number identifier for a low tag")
		}
	}
	n, err := readDERLength(s)
	if err != nil {
		return 0, nil, err
	}
	var body []byte
	if !s.ReadBytes(&body, n) {
		return 0, nil, errors.New("element runs past its list")
	}
	return tag, cryptobyte.String(body), nil
}

// readDERLength reads a DER length: short form below 128, otherwise the
// minimal long form.
func readDERLength(s *cryptobyte.String) (int, error) {
	var b uint8
	if !s.ReadUint8(&b) {
		return 0, errors.New("truncated length")
	}
	if b&0x80 == 0 {
		return int(b), nil
	}
	octets := int(b & 0x7f)
	if octets == 0 || octets > 3 {
		return 0, fmt.Errorf("length of %d octets", octets)
	}
	n := 0
	for i := 0; i < octets; i++ {
		var o uint8
		if !s.ReadUint8(&o) {
			return 0, errors.New("truncated length")
		}
		if i == 0 && o == 0 {
			return 0, errors.New("length is not minimal")
		}
		n = n<<8 | int(o)
	}
	if n < 0x80 {
		return 0, errors.New("long-form length below 128")
	}
	return n, nil
}

func readOnlyInt(body cryptobyte.String) (int64, error) {
	var v int64
	if !body.ReadASN1Int64WithTag(&v, cbasn1.INTEGER) || !body.Empty() {
		return 0, errors.New("not one INTEGER")
	}
	return v, nil
}

func parseRootOfTrust(body cryptobyte.String) (*RootOfTrust, error) {
	var seq cryptobyte.String
	if !body.ReadASN1(&seq, cbasn1.SEQUENCE) || !body.Empty() {
		return nil, errors.New("RootOfTrust is not one SEQUENCE")
	}
	var rot RootOfTrust
	var state int
	if !seq.ReadASN1Bytes(&rot.VerifiedBootKey, cbasn1.OCTET_STRING) ||
		!seq.ReadASN1Boolean(&rot.DeviceLocked) ||
		!seq.ReadASN1Enum(&state) {
		return nil, errors.New("RootOfTrust lacks verifiedBootKey, deviceLocked or verifiedBootState")
	}
	// verifiedBootHash was added in attestation version 3.
	if !seq.Empty() && (!seq.ReadASN1Bytes(&rot.VerifiedBootHash, cbasn1.OCTET_STRING) || !seq.Empty()) {
		return nil, errors.New("RootOfTrust has trailing fields")
	}
	switch s := BootState(state); s {
	case BootVerified, BootSelfSigned, BootUnverified, BootFailed:
		rot.VerifiedBootState = s
	default:
		return nil, fmt.Errorf("unknown verifiedBootState %d", state)
	}
	return &rot, nil
}

// parseApplicationID reads attestationApplicationId: an OCTET STRING holding
// the DER of AttestationApplicationId ::= SEQUENCE { package_infos SET OF
// AttestationPackageInfo, signature_digests SET OF OCTET_STRING }.
func parseApplicationID(body cryptobyte.String) (*ApplicationID, error) {
	var inner, seq, pkgs, digests cryptobyte.String
	if !body.ReadASN1(&inner, cbasn1.OCTET_STRING) || !body.Empty() ||
		!inner.ReadASN1(&seq, cbasn1.SEQUENCE) || !inner.Empty() ||
		!seq.ReadASN1(&pkgs, cbasn1.SET) ||
		!seq.ReadASN1(&digests, cbasn1.SET) || !seq.Empty() {
		return nil, errors.New("attestationApplicationId is not OCTET STRING { SEQUENCE { SET, SET } }")
	}
	var app ApplicationID
	for !pkgs.Empty() {
		var p, name cryptobyte.String
		var info PackageInfo
		if !pkgs.ReadASN1(&p, cbasn1.SEQUENCE) ||
			!p.ReadASN1(&name, cbasn1.OCTET_STRING) ||
			!p.ReadASN1Int64WithTag(&info.Version, cbasn1.INTEGER) || !p.Empty() {
			return nil, errors.New("AttestationPackageInfo is not SEQUENCE { OCTET STRING, INTEGER }")
		}
		info.Name = string(name)
		app.Packages = append(app.Packages, info)
	}
	for !digests.Empty() {
		var d []byte
		if !digests.ReadASN1Bytes(&d, cbasn1.OCTET_STRING) {
			return nil, errors.New("signature digest is not an OCTET STRING")
		}
		app.SignatureDigests = append(app.SignatureDigests, d)
	}
	return &app, nil
}
