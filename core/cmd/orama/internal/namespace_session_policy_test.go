package cli

import "testing"

func TestSessionPolicyFields(t *testing.T) {
	topics := []string{"presence/*", "users/global"}
	none := []string{}
	for name, tc := range map[string]struct {
		signIn, device string
		topics         *[]string
		want           string
	}{
		"nothing reads":     {"", "", nil, ""},
		"sign-in only":      {"open", "", nil, `{"sign_in":"open"}`},
		"device only":       {"", "required", nil, `{"device_policy":"required"}`},
		"both":              {"members", "approval", nil, `{"device_policy":"approval","sign_in":"members"}`},
		"unvalidated value": {"everyone", "", nil, `{"sign_in":"everyone"}`},
		"end-user topics":   {"", "", &topics, `{"end_user_topics":["presence/*","users/global"]}`},
		"clearing topics":   {"", "", &none, `{"end_user_topics":[]}`},
	} {
		got, err := sessionPolicyFields(tc.signIn, tc.device, tc.topics)
		if err != nil || string(got) != tc.want {
			t.Errorf("%s: got %q, %v; want %q", name, got, err, tc.want)
		}
		if tc.want == "" && got != nil {
			t.Errorf("%s: a read carried a body %q", name, got)
		}
	}
}
