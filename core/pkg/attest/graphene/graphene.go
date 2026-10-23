// Package graphene keeps GrapheneOS's signed list of verified-boot keys
// (https://grapheneos.org/attestation.json) fresh in the background. An
// Android device whose OS is signed with its own key attests
// verifiedBootState SelfSigned; such a device is accepted only when that key
// is on this list.
//
// The list is verified as GrapheneOS's attestation compatibility guide
// specifies: an SSHSIG (attestation.json.sig) by the key in GrapheneOS's
// allowed_signers, identity contact@grapheneos.org, namespace
// "attestation keys". Both the signer key and a signed snapshot of the list
// are compiled in, so SelfSigned devices verify from the first boot, and a
// list whose timestamp is lower than the last accepted one is refused (a
// downgrade). A GrapheneOS signing-key rotation needs a release with the new
// key.
package graphene

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	"golang.org/x/crypto/ssh"

	"github.com/Orama-Network/orama/core/pkg/attest/internal/feed"
)

const (
	// ListURL is GrapheneOS's list; SignatureURL is its signature.
	ListURL      = "https://grapheneos.org/attestation.json"
	SignatureURL = ListURL + ".sig"
	// StaleAfter is the age past which the list is reported stale.
	StaleAfter = 72 * time.Hour

	signerIdentity = "contact@grapheneos.org"
	signNamespace  = "attestation keys"
	maxListBytes   = 1 << 20
	maxSigBytes    = 16 << 10
	bootKeyLen     = 32
)

var (
	//go:embed allowed_signers
	allowedSigners []byte
	//go:embed attestation.json
	snapshotList []byte
	//go:embed attestation.json.sig
	snapshotSig []byte
)

// errSuperseded marks a list older than the one already accepted.
var errSuperseded = errors.New("the list is older than the one already accepted")

// List is a verified list of accepted SelfSigned verified-boot keys.
type List struct {
	keys      map[string]bool
	Timestamp int64
}

// Contains reports whether key (raw verifiedBootKey bytes) is listed.
func (l *List) Contains(key []byte) bool { return l.keys[string(key)] }

// Len is the number of listed keys.
func (l *List) Len() int { return len(l.keys) }

// Verify checks that sig is GrapheneOS's signature over list and parses it.
func Verify(signer ssh.PublicKey, list, sig []byte) (*List, error) {
	if err := verifySSHSig(signer, signNamespace, list, sig); err != nil {
		return nil, fmt.Errorf("GrapheneOS attestation list: %w", err)
	}
	var doc struct {
		VerifiedBootKeys []string `json:"verifiedBootKeys"`
		Timestamp        *int64   `json:"timestamp"`
	}
	dec := json.NewDecoder(bytes.NewReader(list))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&doc); err != nil {
		return nil, fmt.Errorf("parse the GrapheneOS attestation list: %w", err)
	}
	if doc.Timestamp == nil || doc.VerifiedBootKeys == nil {
		return nil, errors.New("the GrapheneOS attestation list lacks verifiedBootKeys or timestamp")
	}
	l := &List{keys: make(map[string]bool, len(doc.VerifiedBootKeys)), Timestamp: *doc.Timestamp}
	for _, k := range doc.VerifiedBootKeys {
		raw, err := hex.DecodeString(k)
		if err != nil || len(raw) != bootKeyLen {
			return nil, fmt.Errorf("verified boot key %q is not 32 bytes of hex", k)
		}
		l.keys[string(raw)] = true
	}
	return l, nil
}

// PinnedSigner is GrapheneOS's signing key from the compiled-in
// allowed_signers.
func PinnedSigner() ssh.PublicKey {
	line := strings.TrimSpace(string(allowedSigners))
	principal, key, ok := strings.Cut(line, " ")
	if !ok || principal != signerIdentity {
		panic("graphene: the compiled-in allowed_signers does not name " + signerIdentity)
	}
	pub, _, _, _, err := ssh.ParseAuthorizedKey([]byte(key))
	if err != nil {
		panic(fmt.Sprintf("graphene: the compiled-in allowed_signers key does not parse: %v", err))
	}
	return pub
}

// envelope is what the refresher caches: the list and its signature, so a
// cached copy is re-verified on load.
type envelope struct {
	List      []byte `json:"list"`
	Signature []byte `json:"signature"`
}

// Status is how fresh the list is.
type Status = feed.Status

// Refresher keeps the list fresh. Build it with New, call Run in a
// goroutine, and pass it to the Android verifier.
type Refresher struct {
	signer ssh.PublicKey
	feed   feed.Feed
	list   atomic.Pointer[List]
}

// Config configures a Refresher. Zero fields take the defaults.
type Config struct {
	// ListURL and SignatureURL default to GrapheneOS's.
	ListURL      string
	SignatureURL string
	Client       *http.Client
	// CacheFile, if set, keeps the newest accepted list across restarts.
	CacheFile string
	Logger    *zap.Logger
	Now       func() time.Time
	// Signer and the snapshot default to the compiled-in ones.
	Signer       ssh.PublicKey
	SnapshotList []byte
	SnapshotSig  []byte
}

// New returns a Refresher holding the compiled-in snapshot, or the cached
// list when that is newer.
func New(cfg Config) (*Refresher, error) {
	if cfg.ListURL == "" {
		cfg.ListURL = ListURL
	}
	if cfg.SignatureURL == "" {
		cfg.SignatureURL = SignatureURL
	}
	if cfg.Client == nil {
		cfg.Client = http.DefaultClient
	}
	if cfg.Logger == nil {
		cfg.Logger = zap.NewNop()
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Signer == nil {
		cfg.Signer = PinnedSigner()
	}
	if cfg.SnapshotList == nil {
		cfg.SnapshotList, cfg.SnapshotSig = snapshotList, snapshotSig
	}
	r := &Refresher{signer: cfg.Signer}
	snapshot, err := Verify(cfg.Signer, cfg.SnapshotList, cfg.SnapshotSig)
	if err != nil {
		return nil, fmt.Errorf("the compiled-in snapshot: %w", err)
	}
	r.list.Store(snapshot)
	r.feed = feed.Feed{
		Name: "GrapheneOS attestation list",
		Fetch: func(ctx context.Context) ([]byte, time.Duration, error) {
			return fetch(ctx, cfg.Client, cfg.ListURL, cfg.SignatureURL)
		},
		CacheFile: cfg.CacheFile,
		Logger:    cfg.Logger,
		Now:       cfg.Now,
		Install:   r.install,
	}
	switch err := r.feed.LoadCache(); {
	case errors.Is(err, errSuperseded):
		cfg.Logger.Info("the cached GrapheneOS list is older than the compiled-in one; using the compiled-in one")
	case err != nil:
		return nil, err
	}
	return r, nil
}

// fetch gets the list and its signature as one envelope.
func fetch(ctx context.Context, client *http.Client, listURL, sigURL string) ([]byte, time.Duration, error) {
	list, listFresh, err := feed.Get(ctx, client, listURL, maxListBytes)
	if err != nil {
		return nil, 0, err
	}
	sig, sigFresh, err := feed.Get(ctx, client, sigURL, maxSigBytes)
	if err != nil {
		return nil, 0, err
	}
	body, err := json.Marshal(envelope{List: list, Signature: sig})
	if err != nil {
		return nil, 0, err
	}
	return body, min(listFresh, sigFresh), nil
}

// install verifies an envelope and makes it current unless it is older than
// the current list. An equal timestamp is the same list again.
func (r *Refresher) install(body []byte, _ time.Time) error {
	var env envelope
	if err := json.Unmarshal(body, &env); err != nil {
		return fmt.Errorf("parse the list envelope: %w", err)
	}
	l, err := Verify(r.signer, env.List, env.Signature)
	if err != nil {
		return err
	}
	if cur := r.list.Load(); l.Timestamp < cur.Timestamp {
		return fmt.Errorf("timestamp %d below the accepted %d: %w", l.Timestamp, cur.Timestamp, errSuperseded)
	}
	r.list.Store(l)
	return nil
}

// Run refreshes the list until ctx is done.
func (r *Refresher) Run(ctx context.Context) { r.feed.Run(ctx) }

// RefreshOnce fetches the list once and returns when to fetch next.
func (r *Refresher) RefreshOnce(ctx context.Context) time.Duration { return r.feed.RefreshOnce(ctx) }

// Contains reports whether key is on the current list.
func (r *Refresher) Contains(key []byte) bool { return r.list.Load().Contains(key) }

// Current is the list in use.
func (r *Refresher) Current() *List { return r.list.Load() }

// Status reports the list's freshness. FetchedAt stays zero while only the
// compiled-in snapshot is in use.
func (r *Refresher) Status() Status { return r.feed.Status() }

// Stale reports whether no list has been fetched or loaded from the cache
// within StaleAfter.
func (r *Refresher) Stale() bool {
	s := r.feed.Status()
	return s.FetchedAt.IsZero() || s.Age(r.feed.Now()) > StaleAfter
}
