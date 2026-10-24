package auth

import (
	"sync"
	"time"
)

// policyCache holds what each namespace's setting on the session policy row
// was when this gateway last read it, for devicePolicyStaleness. Every
// account-level refresh asks for the device and sign-in policies, and every
// pub/sub read by an end user asks for the end-user topics, so each is read
// through one of these rather than from the registry.
type policyCache[T any] struct {
	mu      sync.Mutex
	entries map[string]cachedPolicy[T]
}

type cachedPolicy[T any] struct {
	value  T
	readAt time.Time
}

func (c *policyCache[T]) get(namespace string, now time.Time) (T, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[namespace]
	if !ok || now.Sub(e.readAt) >= devicePolicyStaleness {
		var zero T
		return zero, false
	}
	return e.value, true
}

func (c *policyCache[T]) put(namespace string, value T, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil {
		c.entries = map[string]cachedPolicy[T]{}
	}
	c.entries[namespace] = cachedPolicy[T]{value: value, readAt: now}
}
