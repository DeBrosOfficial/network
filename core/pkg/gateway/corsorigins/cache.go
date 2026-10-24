package corsorigins

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ReloadInterval is how often a gateway rereads every namespace's list, and so
// how long a change made through another gateway takes to reach this one. The
// gateway that recorded a change applies it at once.
const ReloadInterval = 30 * time.Second

// reloadTimeout bounds one read of the table.
const reloadTimeout = 5 * time.Second

// Cache is every namespace's list, held in memory and reread on a timer.
//
// The CORS decision is made on every request, before authentication, from the
// Host the request names, so it must cost no registry read: a per-request or
// per-namespace lookup would let anyone turn requests with made-up ns-<name>
// hosts into registry queries. The whole table is read instead, once per
// ReloadInterval per gateway whatever the traffic: at most
// MaxOriginsPerNamespace short strings for each namespace that set any.
//
// Until the first read succeeds nothing beyond the built-in rule is admitted:
// an origin is never let in on a guess.
type Cache struct {
	store  *Store
	logger *zap.Logger

	mu          sync.RWMutex
	byNamespace map[string]map[string]bool
}

// NewCache builds an empty cache over store.
func NewCache(store *Store, logger *zap.Logger) *Cache {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Cache{store: store, logger: logger, byNamespace: map[string]map[string]bool{}}
}

// Allows reports whether namespace lists origin. origin is compared in its
// canonical form.
func (c *Cache) Allows(namespace, origin string) bool {
	if c == nil {
		return false
	}
	canonical, err := Canonical(origin)
	if err != nil {
		return false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.byNamespace[normalizedNamespace(namespace)][canonical]
}

// Put applies a list this gateway just recorded, ahead of the next reload.
func (c *Cache) Put(namespace string, origins []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(origins) == 0 {
		delete(c.byNamespace, normalizedNamespace(namespace))
		return
	}
	c.byNamespace[normalizedNamespace(namespace)] = setOf(origins)
}

// Reload rereads the table. A failed read keeps the previous lists: they were
// set by the namespaces' owners and a registry blip does not unset them.
func (c *Cache) Reload(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, reloadTimeout)
	defer cancel()
	all, err := c.store.All(ctx)
	if err != nil {
		return err
	}
	next := make(map[string]map[string]bool, len(all))
	for ns, origins := range all {
		next[ns] = setOf(origins)
	}
	c.mu.Lock()
	c.byNamespace = next
	c.mu.Unlock()
	return nil
}

// Start reloads now and then every ReloadInterval until ctx is done.
func (c *Cache) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(ReloadInterval)
		defer ticker.Stop()
		for {
			if err := c.Reload(ctx); err != nil {
				c.logger.Warn("could not reload the namespaces' allowed origins; the previous lists stay in force", zap.Error(err))
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

func setOf(origins []string) map[string]bool {
	set := make(map[string]bool, len(origins))
	for _, o := range origins {
		set[o] = true
	}
	return set
}
