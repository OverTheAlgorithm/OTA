package cache

import "time"

// Cache is a generic in-process key-value store with per-entry TTL support.
// All method names are exported so the interface can be used across packages.
type Cache interface {
	Get(k string) (any, bool)
	Set(k string, v any, ttl time.Duration)
	Delete(k string)
	Has(k string) bool
}
