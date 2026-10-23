package turn

import (
	"fmt"
	"net"
	"testing"
	"time"

	pionTurn "github.com/pion/turn/v5"
	"go.uber.org/zap"
)

const (
	authTestNamespace  = "auth-ns"
	authTestSecret     = "auth-test-secret"
	authTestRealm      = "turn.test.local"
	authTestRelayStart = 46000
	authTestRelayEnd   = 46200
)

// startAuthTestServer runs a real TURN server on loopback so the pion
// AuthHandler adapter is exercised through an actual Allocate exchange.
func startAuthTestServer(t *testing.T) *Server {
	t.Helper()
	srv, err := NewServer(&Config{
		ListenAddr:     "127.0.0.1:0",
		PublicIP:       "127.0.0.1",
		Realm:          authTestRealm,
		Namespace:      authTestNamespace,
		AuthSecret:     authTestSecret,
		RelayPortStart: authTestRelayStart,
		RelayPortEnd:   authTestRelayEnd,
	}, zap.NewNop())
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	t.Cleanup(func() { _ = srv.Close() })
	return srv
}

func allocateWith(t *testing.T, srv *Server, username, password string) error {
	t.Helper()
	clientConn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("client ListenPacket: %v", err)
	}
	defer clientConn.Close()

	client, err := pionTurn.NewClient(&pionTurn.ClientConfig{
		TURNServerAddr: srv.conn.LocalAddr().String(),
		Conn:           clientConn,
		Username:       username,
		Password:       password,
		Realm:          authTestRealm,
		RTO:            200 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer client.Close()
	if err := client.Listen(); err != nil {
		t.Fatalf("client.Listen: %v", err)
	}

	relay, err := client.Allocate()
	if err != nil {
		return err
	}
	return relay.Close()
}

func TestServer_allocateWithValidCredentials(t *testing.T) {
	srv := startAuthTestServer(t)
	user, pass := mustCredentials(t, authTestNamespace, time.Hour)
	if err := allocateWith(t, srv, user, pass); err != nil {
		t.Fatalf("Allocate with valid credentials: %v", err)
	}
	// bugboard #391: during the transition a credential from the release
	// before subjects still allocates.
	legacy := fmt.Sprintf("%d:%s", time.Now().Add(time.Hour).Unix(), authTestNamespace)
	if err := allocateWith(t, srv, legacy, GeneratePassword(authTestSecret, legacy)); err != nil {
		t.Fatalf("Allocate with a two-field credential: %v", err)
	}
}

func TestServer_allocateRejectsBadCredentials(t *testing.T) {
	srv := startAuthTestServer(t)
	validUser, _ := mustCredentials(t, authTestNamespace, time.Hour)
	expiredUser, expiredPass := mustCredentials(t, authTestNamespace, -time.Hour)
	otherUser, otherPass := mustCredentials(t, "other-ns", time.Hour)
	_, bobPass := mustCredentials(t, authTestNamespace, time.Hour, "0xbob")

	tests := []struct {
		name, user, pass string
	}{
		{"wrong password", validUser, "not-the-password"},
		{"another subject's password", validUser, bobPass},
		{"expired credential", expiredUser, expiredPass},
		{"namespace this server does not serve", otherUser, otherPass},
		{"malformed username", "no-colon", "x"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := allocateWith(t, srv, tt.user, tt.pass); err == nil {
				t.Fatal("Allocate succeeded, want authentication failure")
			}
		})
	}
}

// mustCredentials mints a credential in namespace for subject, 0xalice when
// none is named.
func mustCredentials(t *testing.T, namespace string, ttl time.Duration, subject ...string) (string, string) {
	t.Helper()
	who := "0xalice"
	if len(subject) > 0 {
		who = subject[0]
	}
	user, pass, err := GenerateCredentials(authTestSecret, namespace, who, ttl)
	if err != nil {
		t.Fatal(err)
	}
	return user, pass
}
