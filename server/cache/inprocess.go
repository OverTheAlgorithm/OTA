package cache

import (
	"sync"
	"time"
)

// entry holds a cached value and its expiry time.
type entry struct {
	value     any
	expiresAt time.Time
}

// InProcessCache is a simple thread-safe in-memory cache for use in tests
// and local development when Redis is not available.
type InProcessCache struct {
	mu   sync.RWMutex
	data map[string]entry
}

// NewInProcess returns a ready-to-use InProcessCache.
func NewInProcess() *InProcessCache {
	return &InProcessCache{data: make(map[string]entry)}
}

// Get returns the value and whether it was found and not expired.
func (c *InProcessCache) Get(k string) (any, bool) {
	c.mu.RLock()
	e, ok := c.data[k]
	c.mu.RUnlock()
	if !ok || time.Now().After(e.expiresAt) {
		return nil, false
	}
	return e.value, true
}

// Set stores a value with the given TTL.
func (c *InProcessCache) Set(k string, v any, ttl time.Duration) {
	c.mu.Lock()
	c.data[k] = entry{value: v, expiresAt: time.Now().Add(ttl)}
	c.mu.Unlock()
}

// Delete removes a key.
func (c *InProcessCache) Delete(k string) {
	c.mu.Lock()
	delete(c.data, k)
	c.mu.Unlock()
}

// Has reports whether a live (non-expired) key exists.
func (c *InProcessCache) Has(k string) bool {
	_, ok := c.Get(k)
	return ok
}

// Close is a no-op for in-process caches; satisfies the Cache interface.
func (c *InProcessCache) Close() error { return nil }
