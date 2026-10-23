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

// sessionPolicyPath is the route that reads and sets a namespace's session
// policy: who may sign in to it, and what its sessions must bind.
const sessionPolicyPath = "/v1/namespace/session-policy"

// SessionPolicyChange is what `orama namespace session-policy` was asked to
// set. An empty string or a nil pointer is a field the caller did not name.
type SessionPolicyChange struct {
	SignIn        string
	DevicePolicy  string
	EndUserTopics *[]string // an empty list clears it
	PerIPLimits   string    // on | off
	// NewWalletsPerIPHour and InvokesPerIPMinute are the per-IP ceilings.
	NewWalletsPerIPHour *int
	InvokesPerIPMinute  *int
}

// NamespaceSessionPolicy shows a namespace's session policy, or sets the fields
// the caller passed. A field left empty is left as the gateway has it, so
// `--sign-in open` changes nothing about devices. No field is checked here:
// the gateway validates them all before writing any and says which value it
// refused, and a second copy of its rules would be the one that drifts.
func NamespaceSessionPolicy(out *printer.Printer, ns string, change SessionPolicyChange) error {
	gatewayURL, token, err := loadAuthForNamespace(ns)
	if err != nil {
		return err
	}

	fields, err := sessionPolicyFields(change)
	if err != nil {
		return err
	}
	method, what := http.MethodGet, "read the session policy"
	var body io.Reader
	if fields != nil {
		method, what, body = http.MethodPut, "set the session policy", bytes.NewReader(fields)
	}
	result, err := nsRequest(what, method, gatewayURL+sessionPolicyPath, token, body)
	if err != nil {
		return err
	}
	if out.JSONMode() {
		return out.JSON(result)
	}
	fmt.Fprintf(out.Out(), "Session policy for namespace '%v'\n\n", result["namespace"])
	fmt.Fprintf(out.Out(), "  Sign-in:        %v\n", result["sign_in"])
	fmt.Fprintf(out.Out(), "  Device policy:  %v\n", result["device_policy"])
	fmt.Fprintf(out.Out(), "  End-user topics: %s\n", endUserTopicsLine(result["end_user_topics"]))
	fmt.Fprintf(out.Out(), "  Per-IP limits:  %s\n", perIPLimitsLine(result))
	if revoked, ok := result["revoked_sign_in_keys"].(float64); ok && revoked > 0 {
		fmt.Fprintf(out.Out(), "\n  Revoked %.0f end-user sign-in keys: requiring devices ends the keys a sign-in minted.\n", revoked)
	}
	return nil
}

// endUserTopicsLine prints the topics a namespace's end users read.
func endUserTopicsLine(v any) string {
	topics, _ := v.([]any)
	if len(topics) == 0 {
		return "none (end users read no pub/sub topic)"
	}
	parts := make([]string, len(topics))
	for i, t := range topics {
		parts[i] = fmt.Sprint(t)
	}
	return strings.Join(parts, ", ")
}

// perIPLimitsLine prints a namespace's per-IP limits: off, or the two
// ceilings in force.
func perIPLimitsLine(result map[string]any) string {
	if result["per_ip_limits"] != "on" {
		return fmt.Sprintf("off (when on: %v new wallets per IP per hour, %v invokes per IP per minute)",
			result["new_wallets_per_ip_hour"], result["invokes_per_ip_minute"])
	}
	return fmt.Sprintf("on: %v new wallets per IP per hour, %v authenticated invokes per IP per minute",
		result["new_wallets_per_ip_hour"], result["invokes_per_ip_minute"])
}

// sessionPolicyFields is the body of a PUT carrying only the fields the caller
// named, or nil when it named none and the command reads instead.
func sessionPolicyFields(change SessionPolicyChange) ([]byte, error) {
	fields := map[string]any{}
	if change.SignIn != "" {
		fields["sign_in"] = change.SignIn
	}
	if change.DevicePolicy != "" {
		fields["device_policy"] = change.DevicePolicy
	}
	if change.EndUserTopics != nil {
		fields["end_user_topics"] = *change.EndUserTopics
	}
	if change.PerIPLimits != "" {
		fields["per_ip_limits"] = change.PerIPLimits
	}
	if change.NewWalletsPerIPHour != nil {
		fields["new_wallets_per_ip_hour"] = *change.NewWalletsPerIPHour
	}
	if change.InvokesPerIPMinute != nil {
		fields["invokes_per_ip_minute"] = *change.InvokesPerIPMinute
	}
	if len(fields) == 0 {
		return nil, nil
	}
	raw, err := json.Marshal(fields)
	if err != nil {
		return nil, clierr.Failure("failed to encode the session policy: %w", err)
	}
	return raw, nil
}
