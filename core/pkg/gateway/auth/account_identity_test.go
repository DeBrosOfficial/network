package auth

import "testing"

func TestAccountIdentity_namespaceAccountWinsOverTheSubject(t *testing.T) {
	cases := []struct {
		name   string
		claims JWTClaims
		want   string
	}{
		{"account named", JWTClaims{Sub: "0xwallet", Custom: map[string]string{AccountIDClaim: "acct-1"}}, "acct-1"},
		{"no custom claims", JWTClaims{Sub: "0xwallet"}, "0xwallet"},
		{"other claims only", JWTClaims{Sub: "0xwallet", Custom: map[string]string{"tier": "pro"}}, "0xwallet"},
		{"empty account", JWTClaims{Sub: "0xwallet", Custom: map[string]string{AccountIDClaim: ""}}, "0xwallet"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.claims.AccountIdentity(); got != c.want {
				t.Fatalf("AccountIdentity() = %q, want %q", got, c.want)
			}
		})
	}
}
