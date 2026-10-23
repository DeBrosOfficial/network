package turn

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestGenerateCredentials(t *testing.T) {
	secret := "test-secret-key-32bytes-long!!!!"
	namespace := "test-namespace"
	ttl := 10 * time.Minute

	username, password, err := GenerateCredentials(secret, namespace, "0xalice", ttl)
	if err != nil {
		t.Fatal(err)
	}

	if username == "" {
		t.Fatal("username should not be empty")
	}
	if password == "" {
		t.Fatal("password should not be empty")
	}

	// Username should be "{timestamp}:{namespace}:{subject id}"
	parts := strings.Split(username, ":")
	if len(parts) != 3 {
		t.Fatalf("username format should be timestamp:namespace:subject, got %q", username)
	}
	ts, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		t.Fatalf("timestamp in %q: %v", username, err)
	}
	if parts[1] != namespace {
		t.Fatalf("namespace in username should be %q, got %q", namespace, parts[1])
	}
	if parts[2] != SubjectID(secret, "0xalice") {
		t.Fatalf("subject in username should be the caller's subject id, got %q", parts[2])
	}
	if strings.Contains(username, "0xalice") {
		t.Fatalf("username %q carries the caller's identity in the clear", username)
	}

	// Timestamp should be ~10 minutes in the future
	now := time.Now().Unix()
	expectedExpiry := now + int64(ttl.Seconds())
	if ts < expectedExpiry-2 || ts > expectedExpiry+2 {
		t.Fatalf("expiry timestamp should be ~%d, got %d", expectedExpiry, ts)
	}
}

func TestGeneratePassword(t *testing.T) {
	secret := "test-secret"
	username := "1234567890:test-ns"

	password1 := GeneratePassword(secret, username)
	password2 := GeneratePassword(secret, username)

	// Same inputs should produce same output
	if password1 != password2 {
		t.Fatal("GeneratePassword should be deterministic")
	}

	// Different secret should produce different output
	password3 := GeneratePassword("different-secret", username)
	if password1 == password3 {
		t.Fatal("different secrets should produce different passwords")
	}

	// Different username should produce different output
	password4 := GeneratePassword(secret, "9999999999:other-ns")
	if password1 == password4 {
		t.Fatal("different usernames should produce different passwords")
	}
}

func TestValidateCredentials(t *testing.T) {
	secret := "test-secret-key"
	namespace := "my-namespace"
	ttl := 10 * time.Minute

	// Generate valid credentials
	username, password, err := GenerateCredentials(secret, namespace, "0xalice", ttl)
	if err != nil {
		t.Fatal(err)
	}
	legacy := fmt.Sprintf("%d:%s", time.Now().Add(ttl).Unix(), namespace)

	tests := []struct {
		name      string
		secret    string
		username  string
		password  string
		namespace string
		wantValid bool
	}{
		{
			name:      "valid credentials",
			secret:    secret,
			username:  username,
			password:  password,
			namespace: namespace,
			wantValid: true,
		},
		{
			// bugboard #391: the release that adds subjects still accepts a
			// credential minted by the one before it.
			name:      "a two-field credential from the previous release",
			secret:    secret,
			username:  legacy,
			password:  GeneratePassword(secret, legacy),
			namespace: namespace,
			wantValid: true,
		},
		{
			name:      "a subject that is not a subject id",
			secret:    secret,
			username:  legacy + ":0xalice",
			password:  GeneratePassword(secret, legacy+":0xalice"),
			namespace: namespace,
			wantValid: false,
		},
		{
			name:      "wrong secret",
			secret:    "wrong-secret",
			username:  username,
			password:  password,
			namespace: namespace,
			wantValid: false,
		},
		{
			name:      "wrong password",
			secret:    secret,
			username:  username,
			password:  "wrongpassword",
			namespace: namespace,
			wantValid: false,
		},
		{
			name:      "wrong namespace",
			secret:    secret,
			username:  username,
			password:  password,
			namespace: "other-namespace",
			wantValid: false,
		},
		{
			name:      "expired credentials",
			secret:    secret,
			username:  fmt.Sprintf("%d:%s", time.Now().Unix()-60, namespace),
			password:  GeneratePassword(secret, fmt.Sprintf("%d:%s", time.Now().Unix()-60, namespace)),
			namespace: namespace,
			wantValid: false,
		},
		{
			name:      "malformed username - no colon",
			secret:    secret,
			username:  "badusername",
			password:  "whatever",
			namespace: namespace,
			wantValid: false,
		},
		{
			name:      "malformed username - non-numeric timestamp",
			secret:    secret,
			username:  "notanumber:my-namespace",
			password:  "whatever",
			namespace: namespace,
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidateCredentials(tt.secret, tt.username, tt.password, tt.namespace)
			if got != tt.wantValid {
				t.Errorf("ValidateCredentials() = %v, want %v", got, tt.wantValid)
			}
		})
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		wantErrs int
	}{
		{
			name: "valid config",
			config: Config{
				ListenAddr:     "0.0.0.0:3478",
				PublicIP:       "1.2.3.4",
				Realm:          "dbrs.space",
				AuthSecret:     "secret123",
				RelayPortStart: 49152,
				RelayPortEnd:   50000,
				Namespace:      "test-ns",
			},
			wantErrs: 0,
		},
		{
			name:   "missing all fields",
			config: Config{},
			// bugboard #283: the separate auth_secret and namespace errors collapsed
			// into one "no tenants configured" — a config with neither the legacy
			// pair nor a tenants list serves nobody, which is a single fault.
			wantErrs: 5, // listen_addr, public_ip, realm, no-tenants, relay_port_range
		},
		{
			name: "invalid public IP",
			config: Config{
				ListenAddr:     "0.0.0.0:3478",
				PublicIP:       "not-an-ip",
				Realm:          "dbrs.space",
				AuthSecret:     "secret",
				RelayPortStart: 49152,
				RelayPortEnd:   50000,
				Namespace:      "test-ns",
			},
			wantErrs: 1,
		},
		{
			name: "relay range too small",
			config: Config{
				ListenAddr:     "0.0.0.0:3478",
				PublicIP:       "1.2.3.4",
				Realm:          "dbrs.space",
				AuthSecret:     "secret",
				RelayPortStart: 49152,
				RelayPortEnd:   49200,
				Namespace:      "test-ns",
			},
			wantErrs: 1,
		},
		{
			name: "relay range inverted",
			config: Config{
				ListenAddr:     "0.0.0.0:3478",
				PublicIP:       "1.2.3.4",
				Realm:          "dbrs.space",
				AuthSecret:     "secret",
				RelayPortStart: 50000,
				RelayPortEnd:   49152,
				Namespace:      "test-ns",
			},
			wantErrs: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := tt.config.Validate()
			if len(errs) != tt.wantErrs {
				t.Errorf("Validate() returned %d errors, want %d: %v", len(errs), tt.wantErrs, errs)
			}
		})
	}
}

// bugboard #391: a credential names the user it was minted for, so two users
// asking in the same second get different credentials, and one minted for one
// user is not another's.
func TestGenerateCredentials_namesTheSubject(t *testing.T) {
	const secret, namespace = "test-secret-key", "my-namespace"
	alice, alicePass, err := GenerateCredentials(secret, namespace, "0xalice", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	bob, _, err := GenerateCredentials(secret, namespace, "0xbob", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if alice == bob {
		t.Fatalf("two users got the same credential %q", alice)
	}
	// Alice's password under Bob's username is not a credential.
	if ValidateCredentials(secret, bob, alicePass, namespace) {
		t.Fatal("a password minted for one subject validated for another")
	}
	if SubjectID(secret, "0xalice") == SubjectID("another-namespace-secret", "0xalice") {
		t.Fatal("one user's subject id is the same in two namespaces")
	}
}

func TestGenerateCredentials_refusesNoSubject(t *testing.T) {
	for _, subject := range []string{"", "  "} {
		if _, _, err := GenerateCredentials("s", "ns", subject, time.Hour); !errors.Is(err, ErrNoSubject) {
			t.Errorf("subject %q: err = %v, want ErrNoSubject", subject, err)
		}
	}
}
