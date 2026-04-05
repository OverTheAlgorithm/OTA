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
	mu      sync.RWMutex
	data    map[string]entry
	stopCh  chan struct{}
	stopped bool
}

// NewInProcess returns a ready-to-use InProcessCache with a background sweeper
// that removes expired entries every 60 seconds.
func NewInProcess() *InProcessCache {
	c := &InProcessCache{
		data:   make(map[string]entry),
		stopCh: make(chan struct{}),
	}
	go c.sweep()
	return c
}

// sweep removes expired entries every 60 seconds until Close is called.
func (c *InProcessCache) sweep() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			now := time.Now()
			c.mu.Lock()
			for k, e := range c.data {
				if now.After(e.expiresAt) {
					delete(c.data, k)
				}
			}
			c.mu.Unlock()
		case <-c.stopCh:
			return
		}
	}
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
func (c *InProcessCache) Set(k string, v any, ttl time.Duration) error {
	c.mu.Lock()
	c.data[k] = entry{value: v, expiresAt: time.Now().Add(ttl)}
	c.mu.Unlock()
	return nil
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

// Close stops the background sweeper goroutine. Safe to call multiple times.
func (c *InProcessCache) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.stopped {
		c.stopped = true
		close(c.stopCh)
	}
	return nil
}
