package turn

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	pionTurn "github.com/pion/turn/v5"
	"go.uber.org/zap"
)

// stealthConfigFieldCount is the number of stealth TLS config fields that must
// be set together (StealthDomain, TLSStealthCertPath, TLSStealthKeyPath). Any
// other count is a partial config and fails server startup.
const stealthConfigFieldCount = 3

// Server wraps a Pion TURN server with namespace-scoped HMAC-SHA1 authentication.
type Server struct {
	config      *Config
	logger      *zap.Logger
	turnServer  *pionTurn.Server
	conn        net.PacketConn // UDP listener on primary port (3478)
	tcpListener net.Listener   // Plain TCP listener on primary port (3478)
	tlsListener net.Listener   // TLS TCP listener for TURNS (port 5349)

	certReloader        *certReloader // hot-reloads the primary TURNS cert; nil when TURNS disabled
	stealthCertReloader *certReloader // hot-reloads the stealth-SNI cert; nil when stealth disabled
	certStop            chan struct{} // closed to stop the cert-reload watcher goroutine(s)

	// tenants is the live snapshot of who this server serves (bugboard #283),
	// swapped wholesale by reloadTenants. It is read on every authentication and
	// every stealth-SNI handshake, so it is guarded rather than copied: a shared
	// server must be able to gain and lose tenants without a restart, because
	// restarting drops every OTHER tenant's active relays.
	tenantMu sync.RWMutex
	tenants  *tenantSet
	// servedPath is where the live tenant set is published; empty publishes
	// nothing. Set by WatchTenantConfig before the watcher starts.
	servedPath string
}

// NewServer creates and starts a TURN server.
func NewServer(cfg *Config, logger *zap.Logger) (*Server, error) {
	if errs := cfg.Validate(); len(errs) > 0 {
		return nil, fmt.Errorf("invalid TURN config: %v", errs[0])
	}

	relayIP := net.ParseIP(cfg.PublicIP)
	if relayIP == nil {
		return nil, fmt.Errorf("turn.public_ip: %q is not a valid IP address", cfg.PublicIP)
	}

	s := &Server{
		config: cfg,
		logger: logger.With(zap.String("component", "turn"), zap.String("namespace", cfg.Namespace)),
	}

	// Build the initial tenant set before any listener exists, so a bad tenant
	// or an unreadable stealth cert fails startup instead of opening a port that
	// authorizes nobody. Each tenant's stealth watcher owns its own stop channel,
	// so they start here and are stopped by closeListeners.
	initial, pendingWatchers, err := s.buildTenantSet(cfg, nil)
	if err != nil {
		return nil, err
	}
	s.tenants = initial
	startPendingWatchers(pendingWatchers)

	// Create primary UDP listener (port 3478)
	conn, err := net.ListenPacket("udp4", cfg.ListenAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on %s: %w", cfg.ListenAddr, err)
	}
	s.conn = conn

	packetConfigs := []pionTurn.PacketConnConfig{
		{
			PacketConn: conn,
			RelayAddressGenerator: &pionTurn.RelayAddressGeneratorPortRange{
				RelayAddress: relayIP,
				Address:      "0.0.0.0",
				MinPort:      uint16(cfg.RelayPortStart),
				MaxPort:      uint16(cfg.RelayPortEnd),
			},
		},
	}

	// Plain TCP listener on same port as UDP (3478) for TCP TURN fallback
	var listenerConfigs []pionTurn.ListenerConfig
	tcpListener, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to listen TCP on %s: %w", cfg.ListenAddr, err)
	}
	s.tcpListener = tcpListener

	listenerConfigs = append(listenerConfigs, pionTurn.ListenerConfig{
		Listener: tcpListener,
		RelayAddressGenerator: &pionTurn.RelayAddressGeneratorPortRange{
			RelayAddress: relayIP,
			Address:      "0.0.0.0",
			MinPort:      uint16(cfg.RelayPortStart),
			MaxPort:      uint16(cfg.RelayPortEnd),
		},
	})

	// TURNS: TLS over TCP listener (port 5349) if configured.
	//
	// The cert is served via a hot-reloading GetCertificate callback rather
	// than a static Certificates slice, so a Caddy-renewed cert is picked up
	// in-process without restarting TURN (a restart drops every active relay
	// ~30s). See certReloader / plans/platform/04_STEALTH_TURN.md.
	if cfg.TURNSListenAddr != "" && cfg.TLSCertPath != "" && cfg.TLSKeyPath != "" {
		reloader, err := newCertReloader(cfg.TLSCertPath, cfg.TLSKeyPath, s.logger)
		if err != nil {
			s.closeListeners()
			return nil, fmt.Errorf("failed to load TLS cert/key: %w", err)
		}
		s.certReloader = reloader

		// Stealth SNI: when configured, terminate TLS for a second (neutral)
		// hostname using its own hot-reloading cert. The SNI router forwards the
		// raw stealth-domain bytes to this listener; selection is by ServerName.
		if err := s.loadStealthCertReloader(cfg); err != nil {
			s.closeListeners()
			return nil, err
		}

		tlsConfig := &tls.Config{
			GetCertificate: newGetCertificateMulti(cfg.StealthDomain, reloader, s.stealthCertReloader, s.stealthCertFor),
			MinVersion:     tls.VersionTLS12,
		}
		tlsListener, err := tls.Listen("tcp", cfg.TURNSListenAddr, tlsConfig)
		if err != nil {
			s.closeListeners()
			return nil, fmt.Errorf("failed to listen on %s: %w", cfg.TURNSListenAddr, err)
		}
		s.tlsListener = tlsListener
		s.certStop = make(chan struct{})
		go reloader.watch(turnCertReloadInterval, s.certStop)
		if s.stealthCertReloader != nil {
			go s.stealthCertReloader.watch(turnCertReloadInterval, s.certStop)
		}

		listenerConfigs = append(listenerConfigs, pionTurn.ListenerConfig{
			Listener: tlsListener,
			RelayAddressGenerator: &pionTurn.RelayAddressGeneratorPortRange{
				RelayAddress: relayIP,
				Address:      "0.0.0.0",
				MinPort:      uint16(cfg.RelayPortStart),
				MaxPort:      uint16(cfg.RelayPortEnd),
			},
		})
	}

	// Create TURN server with HMAC-SHA1 auth
	serverConfig := pionTurn.ServerConfig{
		Realm: cfg.Realm,
		AuthHandler: func(ra *pionTurn.RequestAttributes) (string, []byte, bool) {
			// The username doubles as the allocation's user ID, as it did
			// before pion/turn v5 stopped deriving it implicitly.
			key, ok := s.authHandler(ra.Username, ra.Realm, ra.SrcAddr)
			return ra.Username, key, ok
		},
		PacketConnConfigs: packetConfigs,
	}
	if len(listenerConfigs) > 0 {
		serverConfig.ListenerConfigs = listenerConfigs
	}
	turnServer, err := pionTurn.NewServer(serverConfig)
	if err != nil {
		s.closeListeners()
		return nil, fmt.Errorf("failed to create TURN server: %w", err)
	}
	s.turnServer = turnServer

	s.logger.Info("TURN server started",
		zap.String("listen_addr_udp", cfg.ListenAddr),
		zap.String("listen_addr_tcp", cfg.ListenAddr),
		zap.String("turns_listen_addr", cfg.TURNSListenAddr),
		zap.String("public_ip", cfg.PublicIP),
		zap.String("realm", cfg.Realm),
		zap.Int("relay_port_start", cfg.RelayPortStart),
		zap.Int("relay_port_end", cfg.RelayPortEnd),
	)

	return s, nil
}

// loadStealthCertReloader sets up the second cert reloader used for the stealth
// SNI hostname, storing it on s.stealthCertReloader. The three stealth fields
// (StealthDomain, TLSStealthCertPath, TLSStealthKeyPath) are all-or-nothing: a
// partial config is an operator mistake and fails startup rather than silently
// running without the stealth endpoint. When none are set, stealth is disabled
// and the primary TLS path is byte-for-byte unchanged.
func (s *Server) loadStealthCertReloader(cfg *Config) error {
	set := 0
	if cfg.StealthDomain != "" {
		set++
	}
	if cfg.TLSStealthCertPath != "" {
		set++
	}
	if cfg.TLSStealthKeyPath != "" {
		set++
	}
	if set == 0 {
		return nil // stealth disabled
	}
	if set != stealthConfigFieldCount {
		var missing []string
		if cfg.StealthDomain == "" {
			missing = append(missing, "stealth_domain")
		}
		if cfg.TLSStealthCertPath == "" {
			missing = append(missing, "tls_stealth_cert_path")
		}
		if cfg.TLSStealthKeyPath == "" {
			missing = append(missing, "tls_stealth_key_path")
		}
		return fmt.Errorf("turn: partial stealth config — set all of [stealth_domain, tls_stealth_cert_path, tls_stealth_key_path] or none; missing: %s", strings.Join(missing, ", "))
	}

	reloader, err := newCertReloader(cfg.TLSStealthCertPath, cfg.TLSStealthKeyPath, s.logger)
	if err != nil {
		return fmt.Errorf("failed to load stealth TLS cert/key (cert=%s, key=%s): %w", cfg.TLSStealthCertPath, cfg.TLSStealthKeyPath, err)
	}
	s.stealthCertReloader = reloader
	return nil
}

// newGetCertificate builds the tls.Config.GetCertificate callback. When the
// ClientHello ServerName equals stealthDomain (case-insensitively), it serves
// the stealth cert; every other case — including empty SNI and the primary TURN
// domain — serves the primary cert, preserving the pre-stealth behavior. When
// stealth is disabled (stealthReloader nil) it is exactly primary.GetCertificate.
func newGetCertificate(stealthDomain string, primary, stealth *certReloader) func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	return newGetCertificateMulti(stealthDomain, primary, stealth, nil)
}

// newGetCertificateMulti is the multi-tenant form (bugboard #283): a shared
// server answers TURNS for several tenants, so stealth hostnames are matched
// against the per-tenant map first. Everything else — including empty SNI and the
// primary TURN domain, which is covered by the zone wildcard cert — serves the
// primary cert exactly as before.
// The per-tenant lookup is a function rather than a captured map because the
// tenant set changes at runtime: a namespace that enables stealth after this
// server started must be served without a restart, and a captured map would
// pin the set as of process start.
func newGetCertificateMulti(stealthDomain string, primary, stealth *certReloader, byHost func(string) (*certReloader, bool)) func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	return func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
		if hello != nil {
			if byHost != nil {
				if r, ok := byHost(hello.ServerName); ok && r != nil {
					return r.GetCertificate(hello)
				}
			}
			if stealth != nil && strings.EqualFold(hello.ServerName, stealthDomain) {
				return stealth.GetCertificate(hello)
			}
		}
		return primary.GetCertificate(hello)
	}
}

// authHandler validates HMAC-SHA1 credentials.
// Username format: {expiry_unix}:{namespace}:{subject_id} (see parseUsername)
// Password: base64(HMAC-SHA1(shared_secret, username))
func (s *Server) authHandler(username, realm string, srcAddr net.Addr) ([]byte, bool) {
	u, ok := parseUsername(username)
	if !ok {
		s.logger.Debug("Malformed TURN username: expected timestamp:namespace:subject",
			zap.String("username", username),
			zap.String("src_addr", srcAddr.String()))
		return nil, false
	}

	// Resolve THIS namespace's secret (bugboard #283). A shared server serves
	// several tenants, so authorization is the per-tenant lookup: a namespace we
	// do not serve is rejected exactly as a mismatch was before, and a miss must
	// never fall through to another tenant's secret.
	secret, ok := s.tenantSecret(u.namespace)
	if !ok {
		s.logger.Debug("TURN credential for a namespace this server does not serve",
			zap.String("credential_namespace", u.namespace),
			zap.String("src_addr", srcAddr.String()))
		return nil, false
	}

	// Check expiry — credential must not be expired. This runs on every
	// authenticated request, so an allocation is not refreshed past it.
	if u.expiry <= time.Now().Unix() {
		s.logger.Debug("TURN credential expired",
			zap.String("username", username),
			zap.Int64("expired_at", u.expiry),
			zap.String("src_addr", srcAddr.String()))
		return nil, false
	}

	// Generate expected password and derive auth key, using the secret belonging
	// to the credential's OWN namespace.
	password := GeneratePassword(secret, username)
	key := pionTurn.GenerateAuthKey(username, realm, password)

	s.logger.Debug("TURN auth accepted",
		zap.String("namespace", u.namespace),
		zap.String("subject", u.subject),
		zap.String("src_addr", srcAddr.String()))

	return key, true
}

// Close gracefully shuts down the TURN server.
func (s *Server) Close() error {
	s.logger.Info("Stopping TURN server")

	if s.turnServer != nil {
		if err := s.turnServer.Close(); err != nil {
			s.logger.Warn("Error closing TURN server", zap.Error(err))
		}
	}

	s.closeListeners()

	s.logger.Info("TURN server stopped")
	return nil
}

// closeListeners stops the cert watcher and closes all listeners. It is
// idempotent (every field is nil-guarded and nil'd after use) but is NOT
// mutex-protected — it relies on its call sites being single-threaded relative
// to each other (sequential construction, plus a single Close() from main).
func (s *Server) closeListeners() {
	// Per-tenant stealth watchers hold their own stop channels (they must
	// outlive/undercut individual reloads), so shutting down means closing those
	// too, not just the shared one.
	s.stopAllStealthWatchers()
	if s.certStop != nil {
		close(s.certStop)
		s.certStop = nil
	}
	if s.conn != nil {
		s.conn.Close()
		s.conn = nil
	}
	if s.tcpListener != nil {
		s.tcpListener.Close()
		s.tcpListener = nil
	}
	if s.tlsListener != nil {
		s.tlsListener.Close()
		s.tlsListener = nil
	}
	s.certReloader = nil
	s.stealthCertReloader = nil
}

// DefaultCredentialTTL is the lifetime of a minted TURN credential when no
// per-namespace override is configured. It MUST comfortably outlast any
// realistic call: the one-shot REST/host-fn credential paths mint once at call
// setup and never refresh the credential, so once it expires coturn/pion 401s
// the periodic TURN allocation Refresh and a relay-only call's media path dies
// (bugboard #155 — the "every call drops at 10 minutes" bug was a 10-minute
// TTL). 24h is the conventional coturn ephemeral-credential horizon and leaves
// a wide margin over the longest plausible call while still bounding replay
// exposure of a leaked credential.
const DefaultCredentialTTL = 24 * time.Hour

// A credential's username follows the TURN REST convention (an expiry, then
// who it is for), with the namespace between them because one TURN server
// serves several namespaces:
//
//	<expiry unix>:<namespace>:<subject id>
//
// The subject id names the user the credential was minted for (bugboard #391).
// Before it, every user of a namespace who asked in the same second got the
// same credential, nothing in a relay log said whose a relay was, and one
// leaked credential relayed for anyone. It is an opaque keyed hash of the
// caller's identity rather than the identity itself, because a STUN username
// crosses the network in the clear on UDP and TCP TURN, and an account or
// wallet must not.
//
// A credential cannot be revoked one by one: TURN's long-term credential
// mechanism carries no state for a server to cancel, which is the same for
// coturn's use-auth-secret. What bounds it is the expiry, which the server
// checks on every authenticated request (Allocate, Refresh, CreatePermission,
// ChannelBind), so an allocation is not refreshed past it; and the namespace's
// shared secret, whose replacement ends every credential at once. A user who
// loses access is not issued another.

// subjectIDHexLen is the length of a subject id: 64 bits of the keyed hash,
// enough that two users of one namespace do not collide by chance.
const subjectIDHexLen = 16

// subjectIDPurpose domain-separates the subject hash from the password HMAC
// keyed with the same secret.
const subjectIDPurpose = "orama-turn-subject-v1"

// ErrNoSubject is a credential asked for nobody. It would be shared by every
// caller with no identity, which is what a subject is there to prevent.
var ErrNoSubject = errors.New("turn: a credential must name the subject it is for")

// SubjectID is the opaque id a credential carries for subject: the first
// subjectIDHexLen hex digits of HMAC-SHA256(secret, purpose || 0 || subject).
// The namespace's own secret keys it, so the id of one user differs between
// namespaces and only the platform can tell whose it is.
func SubjectID(secret, subject string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(subjectIDPurpose))
	h.Write([]byte{0})
	h.Write([]byte(subject))
	return hex.EncodeToString(h.Sum(nil))[:subjectIDHexLen]
}

// GenerateCredentials creates time-limited HMAC-SHA1 TURN credentials for
// subject, the caller's identity. Returns username and password suitable for
// WebRTC ICE server configuration.
func GenerateCredentials(secret, namespace, subject string, ttl time.Duration) (username, password string, err error) {
	if strings.TrimSpace(subject) == "" {
		return "", "", ErrNoSubject
	}
	expiry := time.Now().Add(ttl).Unix()
	username = fmt.Sprintf("%d:%s:%s", expiry, namespace, SubjectID(secret, subject))
	return username, GeneratePassword(secret, username), nil
}

// credentialUsername is a parsed TURN username.
type credentialUsername struct {
	expiry    int64
	namespace string
	// subject is the subject id, "" for a username minted before subjects.
	subject string
}

// parseUsername reads <expiry>:<namespace>:<subject id>.
//
// It also reads the two-field <expiry>:<namespace> form for the release that
// introduces subjects (bugboard #391): credentials minted by the release
// before it are still in flight for up to their 24 hour lifetime while the
// fleet upgrades. The release after it accepts the three-field form only
// (bugboard chg-948).
func parseUsername(username string) (credentialUsername, bool) {
	parts := strings.SplitN(username, ":", 3)
	if len(parts) < 2 {
		return credentialUsername{}, false
	}
	expiry, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || parts[1] == "" {
		return credentialUsername{}, false
	}
	u := credentialUsername{expiry: expiry, namespace: parts[1]}
	if len(parts) == 3 {
		if !isSubjectID(parts[2]) {
			return credentialUsername{}, false
		}
		u.subject = parts[2]
	}
	return u, true
}

// isSubjectID reports whether s has the shape SubjectID produces.
func isSubjectID(s string) bool {
	if len(s) != subjectIDHexLen {
		return false
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// GeneratePassword computes the HMAC-SHA1 password for a TURN username.
func GeneratePassword(secret, username string) string {
	h := hmac.New(sha1.New, []byte(secret))
	h.Write([]byte(username))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// ValidateCredentials checks if TURN credentials are valid and not expired.
func ValidateCredentials(secret, username, password, expectedNamespace string) bool {
	u, ok := parseUsername(username)
	if !ok || u.namespace != expectedNamespace || u.expiry <= time.Now().Unix() {
		return false
	}
	expected := GeneratePassword(secret, username)
	return hmac.Equal([]byte(password), []byte(expected))
}
