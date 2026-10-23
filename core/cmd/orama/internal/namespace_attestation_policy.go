package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/Orama-Network/orama/core/cmd/orama/internal/clierr"
	"github.com/Orama-Network/orama/core/cmd/orama/internal/printer"
)

// attestationPolicyPath is the route that reads and sets which app builds a
// namespace admits (device attestation, feat-941).
const attestationPolicyPath = "/v1/namespace/attestation-policy"

// AttestationPolicyChange is what the caller asked to change. A nil field is
// left as the gateway has it. Nothing is validated here: the gateway checks
// the whole policy before writing it and names what it refused.
type AttestationPolicyChange struct {
	Mode              *string
	IOSTeamID         *string
	IOSBundleID       *string
	AllowDevelopment  *bool
	MinIOSVersion     *string
	AndroidPackage    *string
	SigningDigests    *[]string
	AllowGrapheneOS   *bool
	MinVersionCode    *int64
	MaxPatchAgeMonths *int
	ReattestInterval  *string
}

// body is the PUT carrying only the fields that were named, or nil when none
// was and the command reads instead.
func (c AttestationPolicyChange) body() ([]byte, error) {
	ios, android, top := map[string]any{}, map[string]any{}, map[string]any{}
	set := func(m map[string]any, key string, v any, present bool) {
		if present {
			m[key] = v
		}
	}
	set(top, "mode", deref(c.Mode), c.Mode != nil)
	set(top, "reattest_interval", deref(c.ReattestInterval), c.ReattestInterval != nil)
	set(ios, "team_id", deref(c.IOSTeamID), c.IOSTeamID != nil)
	set(ios, "bundle_id", deref(c.IOSBundleID), c.IOSBundleID != nil)
	set(ios, "allow_development", deref(c.AllowDevelopment), c.AllowDevelopment != nil)
	set(ios, "min_bundle_version", deref(c.MinIOSVersion), c.MinIOSVersion != nil)
	set(android, "package", deref(c.AndroidPackage), c.AndroidPackage != nil)
	set(android, "signing_cert_digests", deref(c.SigningDigests), c.SigningDigests != nil)
	set(android, "allow_grapheneos", deref(c.AllowGrapheneOS), c.AllowGrapheneOS != nil)
	set(android, "min_version_code", deref(c.MinVersionCode), c.MinVersionCode != nil)
	set(android, "max_patch_age_months", deref(c.MaxPatchAgeMonths), c.MaxPatchAgeMonths != nil)
	if len(ios) > 0 {
		top["ios"] = ios
	}
	if len(android) > 0 {
		top["android"] = android
	}
	if len(top) == 0 {
		return nil, nil
	}
	raw, err := json.Marshal(top)
	if err != nil {
		return nil, clierr.Failure("failed to encode the attestation policy: %w", err)
	}
	return raw, nil
}

func deref[T any](p *T) T {
	if p == nil {
		var zero T
		return zero
	}
	return *p
}

// NamespaceAttestationPolicy shows a namespace's attestation policy, or
// changes the fields the caller named.
func NamespaceAttestationPolicy(out *printer.Printer, ns string, change AttestationPolicyChange) error {
	gatewayURL, token, err := loadAuthForNamespace(ns)
	if err != nil {
		return err
	}
	fields, err := change.body()
	if err != nil {
		return err
	}
	method, what := http.MethodGet, "read the attestation policy"
	var body io.Reader
	if fields != nil {
		method, what, body = http.MethodPut, "set the attestation policy", bytes.NewReader(fields)
	}
	result, err := nsRequest(what, method, gatewayURL+attestationPolicyPath, token, body)
	if err != nil {
		return err
	}
	if out.JSONMode() {
		return out.JSON(result)
	}
	printAttestationPolicy(out.Out(), result)
	return nil
}

func printAttestationPolicy(w io.Writer, p map[string]any) {
	ios, _ := p["ios"].(map[string]any)
	android, _ := p["android"].(map[string]any)
	fmt.Fprintf(w, "Attestation policy for namespace '%v'\n\n", p["namespace"])
	fmt.Fprintf(w, "  Mode:                %v\n", p["mode"])
	fmt.Fprintf(w, "  Re-attest (Android): every %v\n\n", p["reattest_interval"])
	fmt.Fprintf(w, "  iOS app:             %s\n", orNone(joinNonEmpty(".", ios["team_id"], ios["bundle_id"])))
	fmt.Fprintf(w, "    development:       %v\n", ios["allow_development"])
	fmt.Fprintf(w, "    minimum build:     %s\n", orNone(fmt.Sprint(ios["min_bundle_version"])))
	fmt.Fprintf(w, "  Android package:     %s\n", orNone(fmt.Sprint(android["package"])))
	fmt.Fprintf(w, "    signing certs:     %s\n", orNone(joinList(android["signing_cert_digests"])))
	fmt.Fprintf(w, "    GrapheneOS:        %v\n", android["allow_grapheneos"])
	fmt.Fprintf(w, "    minimum version:   %v\n", android["min_version_code"])
	fmt.Fprintf(w, "    max patch age:     %v months\n", android["max_patch_age_months"])
}

func joinNonEmpty(sep string, parts ...any) string {
	var out []string
	for _, p := range parts {
		if s := fmt.Sprint(p); p != nil && s != "" {
			out = append(out, s)
		}
	}
	return strings.Join(out, sep)
}

func joinList(v any) string {
	items, _ := v.([]any)
	parts := make([]string, 0, len(items))
	for _, it := range items {
		parts = append(parts, fmt.Sprint(it))
	}
	return strings.Join(parts, ", ")
}

func orNone(s string) string {
	if s == "" || s == "<nil>" || s == "0" {
		return "(none)"
	}
	return s
}
