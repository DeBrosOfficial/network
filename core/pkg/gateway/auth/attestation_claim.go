package auth

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Orama-Network/orama/core/pkg/attest"
)

// AttestationClaim is what a device's attestation proved, as the access token
// carries it (the `att` claim). Only the gateway sets it — from a verdict of
// package attest, never from anything a client sent — and it is signed into
// the same token as the device it was proved for (`did`), so it cannot be
// moved onto another device's session.
//
// Carrying the verdict in the token is what keeps judging a request off the
// registry: a request is judged by its own token and the namespace's policy,
// both in memory. A raised minimum version, a withdrawn platform or a closed
// development environment reaches every token at once through the policy; a
// revoked device reaches its tokens through the revocation list.
type AttestationClaim struct {
	// Platform is attest.PlatformIOS or attest.PlatformAndroid.
	Platform string `json:"p"`
	// AppID is "TEAMID.bundle.id" on iOS and the package name on Android.
	AppID string `json:"app"`
	// AppVersion is the build the platform signed: CFBundleVersion on iOS 27
	// and later, versionCode on Android; "" when it signed none.
	AppVersion string `json:"v,omitempty"`
	// Development marks an iOS key of the App Attest development environment.
	Development bool `json:"dev,omitempty"`
	// GrapheneOS marks an Android device booting GrapheneOS.
	GrapheneOS bool `json:"gos,omitempty"`
	// AttestedAt is when the device last attested, unix seconds. An Android
	// attestation lasts the policy's ReattestInterval from here.
	AttestedAt int64 `json:"at"`
}

// AttestationRefusal is why a namespace's policy does not admit a caller.
type AttestationRefusal int

const (
	// AttestationAdmitted is no refusal.
	AttestationAdmitted AttestationRefusal = iota
	// AttestationRequired: the caller has not attested, or its attestation
	// has run out. The app attests (again) and carries on.
	AttestationRequired
	// AttestationInvalid: the caller's attestation is of something the
	// namespace does not accept — another app, a platform it does not take,
	// an environment or OS it no longer allows.
	AttestationInvalid
	// AttestationUpdateRequired: the build is older than the namespace
	// accepts.
	AttestationUpdateRequired
)

// String is the refusal's label in counts and logs.
func (r AttestationRefusal) String() string {
	switch r {
	case AttestationRequired:
		return "required"
	case AttestationInvalid:
		return "invalid"
	case AttestationUpdateRequired:
		return "app_update_required"
	default:
		return "admitted"
	}
}

// JudgeAttestation decides whether policy admits a caller whose token carries
// claim (nil when it carries none) at now, and says why not.
func JudgeAttestation(policy AttestationPolicy, claim *AttestationClaim, now time.Time) (AttestationRefusal, string) {
	if policy.Mode == AttestationOff {
		return AttestationAdmitted, ""
	}
	if claim == nil {
		return AttestationRequired, "this namespace admits only its own app on an attested device: attest this device, then refresh the session"
	}
	switch attest.Platform(claim.Platform) {
	case attest.PlatformIOS:
		return judgeIOS(policy, claim)
	case attest.PlatformAndroid:
		return judgeAndroid(policy, claim, now)
	default:
		return AttestationInvalid, fmt.Sprintf("this session's attestation names an unknown platform %q", claim.Platform)
	}
}

func judgeIOS(policy AttestationPolicy, claim *AttestationClaim) (AttestationRefusal, string) {
	if !policy.AcceptsIOS() {
		return AttestationInvalid, "this namespace admits no iOS build"
	}
	if claim.AppID != policy.IOS.TeamID+"."+policy.IOS.BundleID {
		return AttestationInvalid, "this session attested another app than the one this namespace admits"
	}
	if claim.Development && !policy.IOS.AllowDevelopment {
		return AttestationInvalid, "this namespace does not admit development builds"
	}
	if min := policy.IOS.MinBundleVersion; min != "" && claim.AppVersion != "" {
		older, err := bundleVersionOlder(claim.AppVersion, min)
		if err != nil {
			return AttestationInvalid, "this session's attested build version cannot be read: " + err.Error()
		}
		if older {
			return AttestationUpdateRequired, "build " + claim.AppVersion + " is older than this namespace admits (" + min + "): update the app"
		}
	}
	return AttestationAdmitted, ""
}

func judgeAndroid(policy AttestationPolicy, claim *AttestationClaim, now time.Time) (AttestationRefusal, string) {
	if !policy.AcceptsAndroid() {
		return AttestationInvalid, "this namespace admits no Android build"
	}
	if claim.AppID != policy.Android.Package {
		return AttestationInvalid, "this session attested another app than the one this namespace admits"
	}
	if claim.GrapheneOS && !policy.Android.AllowGrapheneOS {
		return AttestationInvalid, "this namespace does not admit GrapheneOS"
	}
	if min := policy.Android.MinVersionCode; min > 0 {
		code, err := strconv.ParseInt(claim.AppVersion, 10, 64)
		if err != nil {
			return AttestationInvalid, "this session's attested versionCode cannot be read"
		}
		if code < min {
			return AttestationUpdateRequired, fmt.Sprintf("versionCode %d is older than this namespace admits (%d): update the app", code, min)
		}
	}
	if now.After(time.Unix(claim.AttestedAt, 0).Add(policy.ReattestInterval)) {
		return AttestationRequired, "this device's attestation is older than this namespace accepts: attest it again, then refresh the session"
	}
	return AttestationAdmitted, ""
}

// bundleVersionMaxParts is how many period-separated integers a
// CFBundleVersion has at most.
const bundleVersionMaxParts = 3

// bundleVersionOlder reports whether CFBundleVersion v is an older build than
// min: one to three period-separated integers, compared part by part, a
// missing trailing part reading as zero. It is the order package attest
// applies when it verifies, applied again here because a minimum raised after
// a device attested must reach that device's tokens too.
func bundleVersionOlder(v, min string) (bool, error) {
	a, err := parseBundleVersion(v)
	if err != nil {
		return false, err
	}
	b, err := parseBundleVersion(min)
	if err != nil {
		return false, err
	}
	for i := range a {
		if a[i] != b[i] {
			return a[i] < b[i], nil
		}
	}
	return false, nil
}

func parseBundleVersion(s string) ([bundleVersionMaxParts]uint64, error) {
	var out [bundleVersionMaxParts]uint64
	parts := strings.Split(s, ".")
	if len(parts) > bundleVersionMaxParts {
		return out, fmt.Errorf("bundle version %q has more than %d parts", s, bundleVersionMaxParts)
	}
	for i, part := range parts {
		n, err := strconv.ParseUint(part, 10, 64)
		if err != nil || part == "" || part[0] == '+' {
			return out, fmt.Errorf("bundle version %q is not period-separated integers", s)
		}
		out[i] = n
	}
	return out, nil
}
