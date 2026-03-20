package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisCache implements Cache backed by a Redis instance.
// All keys are prefixed with the keyPrefix provided at construction to allow
// multiple logical caches to share a single Redis instance.
// Values are JSON-marshalled on write and returned as raw JSON bytes on read.
// Use GetTyped to deserialise values back to a concrete type.
type RedisCache struct {
	client    *redis.Client
	keyPrefix string
	ctx       context.Context
}

// RedisConfig holds connection parameters for a Redis instance.
type RedisConfig struct {
	Host     string
	Port     string
	Password string
	DB       int
}

// NewRedisCache creates a RedisCache connected to the given server.
// keyPrefix is prepended to every key (e.g. "earn:" or "signup:").
// Pings the server to confirm connectivity before returning.
func NewRedisCache(cfg RedisConfig, keyPrefix string) (*RedisCache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}
	return &RedisCache{client: client, keyPrefix: keyPrefix, ctx: ctx}, nil
}

func (r *RedisCache) prefixed(k string) string {
	return r.keyPrefix + k
}

// Get retrieves a value by key and returns the raw JSON bytes as any.
// Returns nil, false if the key is missing or expired.
func (r *RedisCache) Get(k string) (any, bool) {
	data, err := r.client.Get(r.ctx, r.prefixed(k)).Bytes()
	if err != nil {
		return nil, false
	}
	return data, true
}

// Set JSON-encodes v and stores it with the given TTL.
// Silently skips the write if encoding fails.
func (r *RedisCache) Set(k string, v any, ttl time.Duration) {
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	r.client.Set(r.ctx, r.prefixed(k), data, ttl)
}

// Delete removes a key from the cache.
func (r *RedisCache) Delete(k string) {
	r.client.Del(r.ctx, r.prefixed(k))
}

// Has reports whether a live (non-expired) key exists.
func (r *RedisCache) Has(k string) bool {
	n, err := r.client.Exists(r.ctx, r.prefixed(k)).Result()
	return err == nil && n > 0
}

// Close releases the underlying Redis connection.
func (r *RedisCache) Close() error {
	return r.client.Close()
}

// GetTyped retrieves a value from any Cache and deserialises it into T.
// Works transparently for both OtterCache (native type assertion) and
// RedisCache (JSON unmarshal from raw bytes).
func GetTyped[T any](c Cache, key string) (T, bool) {
	var zero T
	raw, ok := c.Get(key)
	if !ok {
		return zero, false
	}
	// OtterCache stores the value as the concrete type directly.
	if v, ok := raw.(T); ok {
		return v, true
	}
	// RedisCache returns []byte (JSON). Unmarshal into T.
	var b []byte
	switch data := raw.(type) {
	case []byte:
		b = data
	case string:
		b = []byte(data)
	default:
		// Unexpected type — try JSON round-trip via re-marshalling.
		var err error
		b, err = json.Marshal(raw)
		if err != nil {
			return zero, false
		}
	}
	var v T
	if err := json.Unmarshal(b, &v); err != nil {
		return zero, false
	}
	return v, true
}
