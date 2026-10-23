package cli

import "testing"

func TestSessionPolicyFields(t *testing.T) {
	topics := []string{"presence/*", "users/global"}
	none := []string{}
	ten, zero := 10, 0
	for name, tc := range map[string]struct {
		change SessionPolicyChange
		want   string
	}{
		"nothing reads":     {SessionPolicyChange{}, ""},
		"sign-in only":      {SessionPolicyChange{SignIn: "open"}, `{"sign_in":"open"}`},
		"device only":       {SessionPolicyChange{DevicePolicy: "required"}, `{"device_policy":"required"}`},
		"both":              {SessionPolicyChange{SignIn: "members", DevicePolicy: "approval"}, `{"device_policy":"approval","sign_in":"members"}`},
		"unvalidated value": {SessionPolicyChange{SignIn: "everyone"}, `{"sign_in":"everyone"}`},
		"end-user topics":   {SessionPolicyChange{EndUserTopics: &topics}, `{"end_user_topics":["presence/*","users/global"]}`},
		"clearing topics":   {SessionPolicyChange{EndUserTopics: &none}, `{"end_user_topics":[]}`},
		"per-ip switch":     {SessionPolicyChange{PerIPLimits: "on"}, `{"per_ip_limits":"on"}`},
		"per-ip ceilings": {SessionPolicyChange{NewWalletsPerIPHour: &ten, InvokesPerIPMinute: &ten},
			`{"invokes_per_ip_minute":10,"new_wallets_per_ip_hour":10}`},
		"a zero is sent for the gateway to refuse": {SessionPolicyChange{NewWalletsPerIPHour: &zero}, `{"new_wallets_per_ip_hour":0}`},
	} {
		got, err := sessionPolicyFields(tc.change)
		if err != nil || string(got) != tc.want {
			t.Errorf("%s: got %q, %v; want %q", name, got, err, tc.want)
		}
		if tc.want == "" && got != nil {
			t.Errorf("%s: a read carried a body %q", name, got)
		}
	}
}

func TestPerIPLimitsLine(t *testing.T) {
	on := perIPLimitsLine(map[string]any{"per_ip_limits": "on", "new_wallets_per_ip_hour": 20.0, "invokes_per_ip_minute": 1200.0})
	if on != "on: 20 new wallets per IP per hour, 1200 authenticated invokes per IP per minute" {
		t.Errorf("on: %q", on)
	}
	off := perIPLimitsLine(map[string]any{"per_ip_limits": "off", "new_wallets_per_ip_hour": 20.0, "invokes_per_ip_minute": 1200.0})
	if off != "off (when on: 20 new wallets per IP per hour, 1200 invokes per IP per minute)" {
		t.Errorf("off: %q", off)
	}
}
