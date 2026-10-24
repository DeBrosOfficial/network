package gateway

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Orama-Network/orama/core/pkg/client"
	"github.com/Orama-Network/orama/core/pkg/gateway/routepolicy"
	"github.com/Orama-Network/orama/core/pkg/logging"
	"go.uber.org/zap"
)

// requestLogEntry holds a single request log to be batched.
type requestLogEntry struct {
	method     string
	path       string
	statusCode int
	bytesOut   int
	durationMs int64
	ip         string
	// apiKeyID is the registry id of the API key that authenticated the
	// request (trafficAttribution.keyID); 0 when no key did.
	apiKeyID int64
}

// newRequestLogEntry is the row one request leaves. A route that declares
// LogNoAddress leaves the address out: the row says a request happened, not
// who from.
func newRequestLogEntry(r *http.Request, policy routepolicy.Policy, status, bytesOut int, dur time.Duration, apiKeyID int64) requestLogEntry {
	ip := ""
	if policy.RequestLog != routepolicy.LogNoAddress {
		ip = getClientIP(r)
	}
	return requestLogEntry{
		method:     r.Method,
		path:       r.URL.Path,
		statusCode: status,
		bytesOut:   bytesOut,
		durationMs: dur.Milliseconds(),
		ip:         ip,
		apiKeyID:   apiKeyID,
	}
}

// apiKeyLastUsedResolution is how stale api_keys.last_used_at may be. A
// gateway writes a key's last_used_at at most once per this interval, so the
// registry, which every gateway of every namespace writes to, takes one write
// per key per gateway per interval however busy the key is.
const apiKeyLastUsedResolution = time.Minute

// requestLogBatcher aggregates request logs and flushes them to RQLite in bulk
// instead of issuing a DB write per request.
type requestLogBatcher struct {
	gw       *Gateway
	entries  []requestLogEntry
	mu       sync.Mutex
	interval time.Duration
	maxBatch int
	stopCh   chan struct{}
	// lastUsedWritten is when this gateway last wrote each key's
	// last_used_at. Only flush touches it, and flushes are serialized by
	// flushMu.
	lastUsedWritten map[int64]time.Time
	flushMu         sync.Mutex
}

func newRequestLogBatcher(gw *Gateway, interval time.Duration, maxBatch int) *requestLogBatcher {
	b := &requestLogBatcher{
		gw:       gw,
		entries:  make([]requestLogEntry, 0, maxBatch),
		interval: interval,
		maxBatch: maxBatch,
		stopCh:   make(chan struct{}),
	}
	go b.run()
	return b
}

// Add enqueues a log entry. If the buffer is full, it triggers an early flush.
func (b *requestLogBatcher) Add(entry requestLogEntry) {
	b.mu.Lock()
	b.entries = append(b.entries, entry)
	needsFlush := len(b.entries) >= b.maxBatch
	b.mu.Unlock()

	if needsFlush {
		go b.flush()
	}
}

// run is the background loop that flushes logs periodically.
func (b *requestLogBatcher) run() {
	ticker := time.NewTicker(b.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			b.flush()
		case <-b.stopCh:
			b.flush() // final flush on stop
			return
		}
	}
}

// buildRequestLogInsert is the batch INSERT of a flush and its arguments.
func buildRequestLogInsert(batch []requestLogEntry) (string, []interface{}) {
	var sb strings.Builder
	sb.WriteString("INSERT INTO request_logs (method, path, status_code, bytes_out, duration_ms, ip, api_key_id) VALUES ")

	args := make([]interface{}, 0, len(batch)*7)
	for i, e := range batch {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString("(?, ?, ?, ?, ?, ?, ?)")

		var apiKeyID interface{}
		if e.apiKeyID > 0 {
			apiKeyID = e.apiKeyID
		}
		args = append(args, e.method, e.path, e.statusCode, e.bytesOut, e.durationMs, e.ip, apiKeyID)
	}
	return sb.String(), args
}

// keysToTouch is the keys of batch whose last_used_at this gateway has not
// written within apiKeyLastUsedResolution of now, which it records as written
// now. It forgets keys written longer ago than that, so the map holds only
// the keys used in the last interval.
func (b *requestLogBatcher) keysToTouch(batch []requestLogEntry, now time.Time) []int64 {
	if b.lastUsedWritten == nil {
		b.lastUsedWritten = make(map[int64]time.Time)
	}
	for id, at := range b.lastUsedWritten {
		if now.Sub(at) >= apiKeyLastUsedResolution {
			delete(b.lastUsedWritten, id)
		}
	}
	var ids []int64
	for _, e := range batch {
		if e.apiKeyID <= 0 {
			continue
		}
		if _, recent := b.lastUsedWritten[e.apiKeyID]; recent {
			continue
		}
		b.lastUsedWritten[e.apiKeyID] = now
		ids = append(ids, e.apiKeyID)
	}
	return ids
}

// buildLastUsedUpdate is the UPDATE that stamps last_used_at on ids.
func buildLastUsedUpdate(ids []int64) (string, []interface{}) {
	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	return fmt.Sprintf("UPDATE api_keys SET last_used_at = CURRENT_TIMESTAMP WHERE id IN (%s)",
		strings.Join(placeholders, ",")), args
}

// flush writes all buffered log entries to RQLite in a single batch.
//
// The rows go to this gateway's own database (request_logs is a namespace
// table). last_used_at goes to the API-key registry (apiKeyDB), the only
// store that holds the keys: a namespace's database has at most a stale copy
// of api_keys that nothing reads.
func (b *requestLogBatcher) flush() {
	b.flushMu.Lock()
	defer b.flushMu.Unlock()

	b.mu.Lock()
	if len(b.entries) == 0 {
		b.mu.Unlock()
		return
	}
	batch := b.entries
	b.entries = make([]requestLogEntry, 0, b.maxBatch)
	b.mu.Unlock()

	if b.gw.client == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	internalCtx := client.WithInternalAuth(ctx)

	db := b.gw.client.Database()
	insert, args := buildRequestLogInsert(batch)
	if _, err := db.Query(internalCtx, insert, args...); err != nil && b.gw.logger != nil {
		b.gw.logger.ComponentWarn(logging.ComponentGeneral, "failed to flush request logs", zap.Error(err))
	}
	if _, err := db.Query(internalCtx,
		"DELETE FROM request_logs WHERE created_at < datetime('now', '-7 days')"); err != nil && b.gw.logger != nil {
		b.gw.logger.ComponentWarn(logging.ComponentGeneral, "failed to prune request_logs", zap.Error(err))
	}

	ids := b.keysToTouch(batch, time.Now())
	if registry := b.gw.apiKeyDB(); len(ids) > 0 && registry != nil {
		q, qargs := buildLastUsedUpdate(ids)
		if _, err := registry.Query(internalCtx, q, qargs...); err != nil && b.gw.logger != nil {
			b.gw.logger.ComponentWarn(logging.ComponentGeneral, "failed to update api key last_used_at", zap.Error(err))
		}
	}

	if b.gw.logger != nil {
		b.gw.logger.ComponentDebug(logging.ComponentGeneral, "request logs flushed",
			zap.Int("count", len(batch)),
			zap.Int("api_keys_touched", len(ids)),
		)
	}
}

// Stop signals the batcher to stop and flush remaining entries.
func (b *requestLogBatcher) Stop() {
	close(b.stopCh)
}
