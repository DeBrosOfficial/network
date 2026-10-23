package auth

import (
	"errors"
	"testing"
)

func TestHashAPIKey_withSecret_differsFromRaw(t *testing.T) {
	s := &Service{apiKeyHMACSecret: "test-hmac-secret"}
	const raw = "ak_live_example:ns"
	got := s.HashAPIKey(raw)
	if got == raw {
		t.Fatal("hashed key must differ from raw when the HMAC secret is set")
	}
	if want := HmacSHA256Hex(raw, "test-hmac-secret"); got != want {
		t.Errorf("HashAPIKey = %q, want %q", got, want)
	}
}

// A service cannot be built without the secret, so there is no raw-key
// fallback left to reach: HashAPIKey used to return the key itself when no
// secret was configured, which stored and compared keys in plaintext.
func TestNewService_refusesAMissingAPIKeyHMACSecret(t *testing.T) {
	for _, secret := range []string{"", "  \n"} {
		if s, err := NewService(nil, nil, "", "default", secret); !errors.Is(err, ErrNoAPIKeyHMACSecret) || s != nil {
			t.Errorf("NewService with secret %q = %v, %v; want ErrNoAPIKeyHMACSecret", secret, s, err)
		}
	}
}
