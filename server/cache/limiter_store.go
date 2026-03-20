package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	limiter "github.com/ulule/limiter/v3"
)

// RedisLimiterStore implements limiter.Store using go-redis v9.
// Uses Redis INCR with PEXPIRE for fixed-window rate limiting.
// State survives server restarts as long as Redis persists (RDB/AOF).
type RedisLimiterStore struct {
	client *redis.Client
	prefix string
}

// NewRedisLimiterStore creates a Redis-backed limiter store.
// Pings Redis to confirm connectivity before returning.
func NewRedisLimiterStore(cfg RedisConfig, prefix string) (*RedisLimiterStore, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("redis ping: %w", err)
	}
	return &RedisLimiterStore{client: client, prefix: prefix}, nil
}

// incrAndExpire atomically increments a key by N and sets PEXPIRE if no TTL
// exists yet. Returns {current_count, pttl_ms}.
var incrAndExpire = redis.NewScript(`
local key = KEYS[1]
local ttlMs = tonumber(ARGV[1])
local n = tonumber(ARGV[2])
local current = redis.call('INCRBY', key, n)
local pttl = redis.call('PTTL', key)
if pttl == -1 then
    redis.call('PEXPIRE', key, ttlMs)
    pttl = ttlMs
end
return {current, pttl}
`)

func (s *RedisLimiterStore) prefixed(key string) string {
	return s.prefix + key
}

func (s *RedisLimiterStore) buildContext(count, pttlMs int64, rate limiter.Rate) limiter.Context {
	remaining := rate.Limit - count
	if remaining < 0 {
		remaining = 0
	}
	resetAt := time.Now().Add(time.Duration(pttlMs) * time.Millisecond).Unix()
	return limiter.Context{
		Limit:     rate.Limit,
		Remaining: remaining,
		Reset:     resetAt,
		Reached:   count > rate.Limit,
	}
}

func (s *RedisLimiterStore) increment(ctx context.Context, key string, count int64, rate limiter.Rate) (limiter.Context, error) {
	k := s.prefixed(key)
	ttlMs := rate.Period.Milliseconds()

	result, err := incrAndExpire.Run(ctx, s.client, []string{k}, ttlMs, count).Int64Slice()
	if err != nil {
		return limiter.Context{}, err
	}

	return s.buildContext(result[0], result[1], rate), nil
}

// Get increments the counter by 1 and returns the rate limit context.
func (s *RedisLimiterStore) Get(ctx context.Context, key string, rate limiter.Rate) (limiter.Context, error) {
	return s.increment(ctx, key, 1, rate)
}

// Peek returns the current counter state without incrementing.
func (s *RedisLimiterStore) Peek(ctx context.Context, key string, rate limiter.Rate) (limiter.Context, error) {
	k := s.prefixed(key)
	val, err := s.client.Get(ctx, k).Int64()
	if err == redis.Nil {
		return limiter.Context{
			Limit:     rate.Limit,
			Remaining: rate.Limit,
			Reset:     time.Now().Add(rate.Period).Unix(),
			Reached:   false,
		}, nil
	}
	if err != nil {
		return limiter.Context{}, err
	}

	pttl, err := s.client.PTTL(ctx, k).Result()
	if err != nil || pttl < 0 {
		pttl = rate.Period
	}

	return s.buildContext(val, pttl.Milliseconds(), rate), nil
}

// Reset deletes the counter key, effectively resetting the rate limit.
func (s *RedisLimiterStore) Reset(ctx context.Context, key string, rate limiter.Rate) (limiter.Context, error) {
	s.client.Del(ctx, s.prefixed(key))
	return limiter.Context{
		Limit:     rate.Limit,
		Remaining: rate.Limit,
		Reset:     time.Now().Add(rate.Period).Unix(),
		Reached:   false,
	}, nil
}

// Increment adds count to the counter and returns the rate limit context.
func (s *RedisLimiterStore) Increment(ctx context.Context, key string, count int64, rate limiter.Rate) (limiter.Context, error) {
	return s.increment(ctx, key, count, rate)
}

// Close releases the underlying Redis connection.
func (s *RedisLimiterStore) Close() error {
	return s.client.Close()
}
