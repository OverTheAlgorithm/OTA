package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	redisStatePrefix = "oauth_state:"
	redisStateTTL    = 10 * time.Minute
)

// StateStorer is the interface satisfied by both the in-memory StateStore
// and the Redis-backed RedisStateStore.
type StateStorer interface {
	Generate() (string, error)
	Validate(state string) bool
}

// RedisStateStore stores OAuth CSRF state tokens in Redis.
// Each token is stored with a TTL and consumed (deleted) on first successful
// validation, preventing replay attacks across multiple server instances.
type RedisStateStore struct {
	client *redis.Client
	ctx    context.Context
}

// NewRedisStateStore creates a RedisStateStore connected to the given server.
// Pings the server to confirm connectivity before returning.
func NewRedisStateStore(addr, password string, db int) (*RedisStateStore, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}
	return &RedisStateStore{client: client, ctx: ctx}, nil
}

// Generate creates a random 16-byte hex state token and stores it in Redis
// with a 10-minute TTL.
func (s *RedisStateStore) Generate() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	state := hex.EncodeToString(b)
	key := redisStatePrefix + state
	if err := s.client.Set(s.ctx, key, "1", redisStateTTL).Err(); err != nil {
		return "", fmt.Errorf("store oauth state: %w", err)
	}
	return state, nil
}

// Validate checks that the state token exists in Redis and atomically deletes
// it (one-time use). Returns false if the token is missing or expired.
func (s *RedisStateStore) Validate(state string) bool {
	key := redisStatePrefix + state
	n, err := s.client.Del(s.ctx, key).Result()
	return err == nil && n > 0
}

// Close releases the underlying Redis connection.
func (s *RedisStateStore) Close() error {
	return s.client.Close()
}
