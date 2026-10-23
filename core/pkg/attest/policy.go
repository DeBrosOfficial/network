package attest

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// bundleVersionMaxParts is the most period-separated integers Apple allows
// in CFBundleVersion.
const bundleVersionMaxParts = 3

// ValidateIOS reports whether p carries what verifying an iOS proof needs.
func (p Policy) ValidateIOS() error {
	if p.AppleTeamID == "" || p.AppleBundleID == "" {
		return errors.New("an iOS policy needs both the Apple team id and the bundle id")
	}
	if strings.Contains(p.AppleTeamID, ".") {
		return fmt.Errorf("Apple team id %q contains a period", p.AppleTeamID)
	}
	if p.MinIOSBundleVersion != "" {
		if _, err := parseBundleVersion(p.MinIOSBundleVersion); err != nil {
			return fmt.Errorf("minimum iOS bundle version: %w", err)
		}
	}
	return nil
}

// ValidateAndroid reports whether p carries what verifying an Android proof
// needs.
func (p Policy) ValidateAndroid() error {
	if p.AndroidPackage == "" {
		return errors.New("an Android policy needs the package name")
	}
	if len(p.AndroidSigningCertDigests) == 0 {
		return errors.New("an Android policy needs at least one signing-certificate digest")
	}
	if _, err := p.signingDigests(); err != nil {
		return err
	}
	if p.MinAndroidVersionCode < 0 {
		return fmt.Errorf("minimum Android versionCode %d is negative", p.MinAndroidVersionCode)
	}
	if p.MaxPatchAgeMonths < 0 {
		return fmt.Errorf("maximum patch age %d months is negative", p.MaxPatchAgeMonths)
	}
	return nil
}

// appleAppID is the App ID whose SHA-256 is an iOS proof's RP ID hash.
func (p Policy) appleAppID() string { return p.AppleTeamID + "." + p.AppleBundleID }

// signingDigests decodes AndroidSigningCertDigests into a set.
func (p Policy) signingDigests() (map[string]bool, error) {
	set := make(map[string]bool, len(p.AndroidSigningCertDigests))
	for _, d := range p.AndroidSigningCertDigests {
		raw, err := hex.DecodeString(d)
		if err != nil || len(raw) != 32 || d != strings.ToLower(d) {
			return nil, fmt.Errorf("signing-certificate digest %q is not a lowercase-hex SHA-256", d)
		}
		set[string(raw)] = true
	}
	return set, nil
}

// bundleVersion is a parsed CFBundleVersion: one to three non-negative
// integers, missing trailing parts reading as zero.
type bundleVersion [bundleVersionMaxParts]uint64

func parseBundleVersion(s string) (bundleVersion, error) {
	var v bundleVersion
	parts := strings.Split(s, ".")
	if len(parts) > bundleVersionMaxParts {
		return v, fmt.Errorf("bundle version %q has more than %d parts", s, bundleVersionMaxParts)
	}
	for i, part := range parts {
		n, err := strconv.ParseUint(part, 10, 64)
		if err != nil || part == "" || part[0] == '+' {
			return v, fmt.Errorf("bundle version %q is not period-separated integers", s)
		}
		v[i] = n
	}
	return v, nil
}

// less reports whether v is an older build than w.
func (v bundleVersion) less(w bundleVersion) bool {
	for i := range v {
		if v[i] != w[i] {
			return v[i] < w[i]
		}
	}
	return false
}
