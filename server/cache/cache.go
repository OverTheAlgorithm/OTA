package cache

import "time"

// Cache is a generic key-value store with per-entry TTL support.
// Implementations include RedisCache (production) and InProcessCache (dev/test).
type Cache interface {
	Get(k string) (any, bool)
	Set(k string, v any, ttl time.Duration) error
	Delete(k string)
	Has(k string) bool
	Close() error
}
