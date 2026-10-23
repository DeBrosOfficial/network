// Package revocation keeps Google's Android attestation revocation list
// (https://android.googleapis.com/attestation/status) fresh in the
// background and answers, from memory, whether a certificate on it is
// revoked. It honours the list's Cache-Control max-age, keeps the last good
// copy when a fetch fails, and reports the copy stale when it is older than
// StaleAfter; no request ever waits on a fetch.
package revocation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"github.com/Orama-Network/orama/core/pkg/attest/internal/feed"
)

const (
	// StatusURL is Google's attestation revocation list.
	StatusURL = "https://android.googleapis.com/attestation/status"
	// StaleAfter is the age past which the list is reported stale. Google
	// serves it with max-age=86400, so three missed refreshes.
	StaleAfter = 72 * time.Hour
	// maxListBytes bounds the list; it is about 180 KB in 2026.
	maxListBytes = 16 << 20
)

// Statuses an entry may have; both mean the key must not be trusted.
const (
	statusRevoked   = "REVOKED"
	statusSuspended = "SUSPENDED"
)

// ErrNoList is returned before any list has been loaded.
var ErrNoList = errors.New("no Android attestation revocation list has been loaded yet")

// Entry is one revoked or suspended key.
type Entry struct {
	Status  string `json:"status"`
	Expires string `json:"expires,omitempty"`
	Reason  string `json:"reason,omitempty"`
	Comment string `json:"comment,omitempty"`
}

// List is a parsed revocation list, keyed by lowercase-hex serial.
type List struct {
	Entries map[string]Entry
}

// Parse parses the list strictly: unknown fields and statuses are refused,
// as the schema Google publishes allows neither.
func Parse(body []byte) (*List, error) {
	var doc struct {
		Entries map[string]Entry `json:"entries"`
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&doc); err != nil {
		return nil, fmt.Errorf("parse the revocation list: %w", err)
	}
	if dec.More() {
		return nil, errors.New("the revocation list has data after its JSON object")
	}
	if doc.Entries == nil {
		return nil, errors.New("the revocation list has no entries object")
	}
	for serial, e := range doc.Entries {
		if e.Status != statusRevoked && e.Status != statusSuspended {
			return nil, fmt.Errorf("entry %s has status %q", serial, e.Status)
		}
	}
	return &List{Entries: doc.Entries}, nil
}

// Status is how fresh the list is.
type Status = feed.Status

// Refresher keeps the list fresh. Build it with New, call Run in a
// goroutine, and pass it to the Android verifier.
type Refresher struct {
	feed feed.Feed
	list atomic.Pointer[List]
}

// Config configures a Refresher. Zero fields take the defaults.
type Config struct {
	// URL defaults to StatusURL.
	URL    string
	Client *http.Client
	// CacheFile, if set, keeps the last good list across restarts.
	CacheFile string
	Logger    *zap.Logger
	Now       func() time.Time
}

// New returns a Refresher that has loaded the cache file, if there is one.
func New(cfg Config) (*Refresher, error) {
	url, client := cfg.URL, cfg.Client
	if url == "" {
		url = StatusURL
	}
	if client == nil {
		client = http.DefaultClient
	}
	r := &Refresher{}
	r.feed = feed.Feed{
		Name: "Android attestation revocation list",
		Fetch: func(ctx context.Context) ([]byte, time.Duration, error) {
			return feed.Get(ctx, client, url, maxListBytes)
		},
		CacheFile: cfg.CacheFile,
		Logger:    cfg.Logger,
		Now:       cfg.Now,
		Install:   r.install,
	}
	if r.feed.Logger == nil {
		r.feed.Logger = zap.NewNop()
	}
	if r.feed.Now == nil {
		r.feed.Now = time.Now
	}
	if err := r.feed.LoadCache(); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *Refresher) install(body []byte, _ time.Time) error {
	l, err := Parse(body)
	if err != nil {
		return err
	}
	r.list.Store(l)
	return nil
}

// Run refreshes the list until ctx is done.
func (r *Refresher) Run(ctx context.Context) { r.feed.Run(ctx) }

// RefreshOnce fetches the list once and returns when to fetch next.
func (r *Refresher) RefreshOnce(ctx context.Context) time.Duration { return r.feed.RefreshOnce(ctx) }

// Revoked reports whether the certificate with this serial (lowercase hex,
// no leading zeros) is revoked or suspended.
func (r *Refresher) Revoked(serialHex string) (bool, error) {
	l := r.list.Load()
	if l == nil {
		return false, ErrNoList
	}
	_, revoked := l.Entries[serialHex]
	return revoked, nil
}

// Status reports the list's freshness.
func (r *Refresher) Status() Status { return r.feed.Status() }

// Stale reports whether there is no list, or the list is older than
// StaleAfter. A stale list is still used: it is the best knowledge there
// is, and the operator is alerted to fix the fetch.
func (r *Refresher) Stale() bool {
	s := r.feed.Status()
	return s.FetchedAt.IsZero() || s.Age(r.feed.Now()) > StaleAfter
}
