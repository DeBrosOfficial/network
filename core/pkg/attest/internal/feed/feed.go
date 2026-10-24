// Package feed keeps a document fetched over HTTPS fresh in the background:
// it refetches when the server's Cache-Control freshness runs out, keeps the
// last good copy (in memory and, optionally, in a file that survives a
// restart) when a fetch fails, and reports how old that copy is. Requests
// never wait on it.
package feed

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/Orama-Network/orama/core/pkg/durablefile"
)

// Refresh bounds: a server's freshness is honoured within them, so a
// max-age of seconds does not turn into a fetch loop and a missing one does
// not stop refreshing.
const (
	MinRefresh = time.Hour
	MaxRefresh = 24 * time.Hour
	// RetryAfterFailure is how soon a failed fetch is tried again.
	RetryAfterFailure = 15 * time.Minute
	fetchTimeout      = 30 * time.Second
	cacheFileMode     = 0o600
)

// Feed is one refreshed document. Fetch gets it and says how long it stays
// fresh; Install validates a fetched or cached body and makes it current. A
// body Install refuses is never cached.
type Feed struct {
	Name      string
	Fetch     func(ctx context.Context) (body []byte, freshFor time.Duration, err error)
	CacheFile string
	Logger    *zap.Logger
	Now       func() time.Time
	// Install receives a body and the time it was fetched.
	Install func(body []byte, fetchedAt time.Time) error

	mu          sync.RWMutex
	fetchedAt   time.Time
	lastAttempt time.Time
	lastErr     error
}

// Status is how fresh a feed's current copy is.
type Status struct {
	// FetchedAt is when the current copy was fetched; zero when there is
	// none.
	FetchedAt   time.Time
	LastAttempt time.Time
	LastError   error
}

// Age is how old the current copy is at now; it is meaningless when
// FetchedAt is zero.
func (s Status) Age(now time.Time) time.Duration { return now.Sub(s.FetchedAt) }

// LoadCache installs the cache file's copy, if there is one, dated by the
// file's modification time. A missing file is not an error.
func (f *Feed) LoadCache() error {
	if f.CacheFile == "" {
		return nil
	}
	body, err := os.ReadFile(f.CacheFile)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read the cached %s: %w", f.Name, err)
	}
	info, err := os.Stat(f.CacheFile)
	if err != nil {
		return fmt.Errorf("stat the cached %s: %w", f.Name, err)
	}
	if err := f.Install(body, info.ModTime()); err != nil {
		return fmt.Errorf("the cached %s: %w", f.Name, err)
	}
	f.mu.Lock()
	f.fetchedAt = info.ModTime()
	f.mu.Unlock()
	return nil
}

// Run fetches at once, then whenever the copy's freshness runs out, until
// ctx is done.
func (f *Feed) Run(ctx context.Context) {
	for {
		wait := f.RefreshOnce(ctx)
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
	}
}

// RefreshOnce fetches and installs the document once and returns how long
// to wait before the next fetch.
func (f *Feed) RefreshOnce(ctx context.Context) time.Duration {
	now := f.Now()
	body, freshFor, err := f.Fetch(ctx)
	if err == nil {
		err = f.Install(body, now)
	}
	if err == nil && f.CacheFile != "" {
		if werr := durablefile.Write(f.CacheFile, body, cacheFileMode); werr != nil {
			f.Logger.Warn("could not cache the fetched list; a restart will refetch it",
				zap.String("feed", f.Name), zap.Error(werr))
		}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastAttempt, f.lastErr = now, err
	if err != nil {
		f.Logger.Warn("fetching the list failed; keeping the last good copy",
			zap.String("feed", f.Name),
			zap.Time("last_good", f.fetchedAt), zap.Error(err))
		return RetryAfterFailure
	}
	f.fetchedAt = now
	return clamp(freshFor)
}

// Status reports the current copy's freshness.
func (f *Feed) Status() Status {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return Status{FetchedAt: f.fetchedAt, LastAttempt: f.lastAttempt, LastError: f.lastErr}
}

// Get fetches url and returns its body (at most maxBytes) and how long the
// server says it stays fresh (max-age less Age; 0 when unstated).
func Get(ctx context.Context, client *http.Client, url string, maxBytes int64) ([]byte, time.Duration, error) {
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, 0, fmt.Errorf("read %s: %w", url, err)
	}
	if int64(len(body)) > maxBytes {
		return nil, 0, fmt.Errorf("%s is larger than %d bytes", url, maxBytes)
	}
	return body, freshness(resp.Header), nil
}

// freshness is max-age less Age (RFC 9111 §4.2), 0 when max-age is absent.
func freshness(h http.Header) time.Duration {
	var maxAge int64 = -1
	for _, directive := range strings.Split(h.Get("Cache-Control"), ",") {
		name, value, ok := strings.Cut(strings.TrimSpace(directive), "=")
		if ok && strings.EqualFold(name, "max-age") {
			if n, err := strconv.ParseInt(strings.Trim(value, `"`), 10, 64); err == nil && n >= 0 {
				maxAge = n
			}
		}
	}
	if maxAge < 0 {
		return 0
	}
	age, err := strconv.ParseInt(h.Get("Age"), 10, 64)
	if err != nil || age < 0 {
		age = 0
	}
	return time.Duration(maxAge-age) * time.Second
}

func clamp(d time.Duration) time.Duration {
	switch {
	case d < MinRefresh:
		return MinRefresh
	case d > MaxRefresh:
		return MaxRefresh
	default:
		return d
	}
}
