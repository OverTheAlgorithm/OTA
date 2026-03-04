package cache

import (
	"time"

	"github.com/maypok86/otter"
)

// OtterCache wraps maypok86/otter with per-entry variable TTL.
// It satisfies the Cache interface.
type OtterCache struct {
	inner otter.CacheWithVariableTTL[string, any]
}

// New creates a ready-to-use OtterCache.
// capacity is the maximum number of items the cache can hold.
func New(capacity int) (*OtterCache, error) {
	c, err := otter.MustBuilder[string, any](capacity).
		WithVariableTTL().
		Build()
	if err != nil {
		return nil, err
	}
	return &OtterCache{inner: c}, nil
}

// Get returns the value and whether it was found.
func (o *OtterCache) Get(k string) (any, bool) {
	return o.inner.Get(k)
}

// Set stores a value with the given TTL.
func (o *OtterCache) Set(k string, v any, ttl time.Duration) {
	o.inner.Set(k, v, ttl)
}

// Delete removes an entry from the cache.
func (o *OtterCache) Delete(k string) {
	o.inner.Delete(k)
}

// Has reports whether a live entry exists for the given key.
func (o *OtterCache) Has(k string) bool {
	_, ok := o.inner.Get(k)
	return ok
}

// Close releases internal resources.
func (o *OtterCache) Close() {
	o.inner.Close()
}
