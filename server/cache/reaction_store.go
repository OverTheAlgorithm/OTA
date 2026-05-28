package cache

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"ota/domain/comment"
)

// Redis key layout (prefixed per RedisReactionStore.prefix):
//
//   counts:{commentID}      Hash {likes, dislikes}
//   reactions:{commentID}   Hash {userID -> "1" or "-1"}
//   dirty                   Set of commentIDs needing write-back
//
// Counts and reactions are kept in lockstep by the apply Lua script,
// which runs every reaction transition. dirty is appended to on every
// modification so the scheduler knows which comments to flush.

// RedisReactionStore implements comment.ReactionStore against Redis.
type RedisReactionStore struct {
	client *redis.Client
	prefix string
}

// NewRedisReactionStore returns a Redis-backed reaction store. prefix is
// prepended to all keys (e.g. "comment:") so multiple feature areas can
// share one Redis instance.
func NewRedisReactionStore(client *redis.Client, prefix string) *RedisReactionStore {
	return &RedisReactionStore{client: client, prefix: prefix}
}

// NewRedisReactionStoreFromConfig connects to Redis using the standard
// config struct and returns a reaction store. Pings the server to confirm
// connectivity before returning.
func NewRedisReactionStoreFromConfig(cfg RedisConfig, prefix string) (*RedisReactionStore, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("redis ping for reaction store: %w", err)
	}
	return &RedisReactionStore{client: client, prefix: prefix}, nil
}

// Close releases the underlying Redis client.
func (s *RedisReactionStore) Close() error {
	return s.client.Close()
}

func (s *RedisReactionStore) countsKey(id uuid.UUID) string {
	return s.prefix + "counts:" + id.String()
}

func (s *RedisReactionStore) reactionsKey(id uuid.UUID) string {
	return s.prefix + "reactions:" + id.String()
}

func (s *RedisReactionStore) dirtyKey() string {
	return s.prefix + "dirty"
}

// applyScript atomically transitions a user's reaction on one comment.
//
// KEYS: [countsKey, reactionsKey, dirtyKey]
// ARGV: [userID, targetReaction]   ("1", "-1", or "0" for clear)
//
// Returns: {previous_reaction, current_reaction, likes, dislikes}
// previous_reaction is the value before the call (-1/0/1).
// current_reaction is the value after the call (-1/0/1).
//
// Same-value calls are no-ops (previous == current, counts unchanged).
// Swapping like→dislike decrements likes by 1 and increments dislikes by 1
// in a single round-trip.
var applyScript = redis.NewScript(`
local counts = KEYS[1]
local reactions = KEYS[2]
local dirty = KEYS[3]
local user = ARGV[1]
local target = tonumber(ARGV[2])

local prevStr = redis.call('HGET', reactions, user)
local prev = 0
if prevStr then
    prev = tonumber(prevStr)
end

if prev == target then
    local l = tonumber(redis.call('HGET', counts, 'likes') or '0')
    local d = tonumber(redis.call('HGET', counts, 'dislikes') or '0')
    return {prev, target, l, d}
end

local likes = tonumber(redis.call('HGET', counts, 'likes') or '0')
local dislikes = tonumber(redis.call('HGET', counts, 'dislikes') or '0')

if prev == 1 then
    likes = likes - 1
elseif prev == -1 then
    dislikes = dislikes - 1
end

if target == 1 then
    likes = likes + 1
elseif target == -1 then
    dislikes = dislikes + 1
end

if likes < 0 then likes = 0 end
if dislikes < 0 then dislikes = 0 end

if target == 0 then
    redis.call('HDEL', reactions, user)
else
    redis.call('HSET', reactions, user, tostring(target))
end

redis.call('HSET', counts, 'likes', tostring(likes))
redis.call('HSET', counts, 'dislikes', tostring(dislikes))
redis.call('SADD', dirty, ARGV[3])

return {prev, target, likes, dislikes}
`)

// Apply runs the Lua transition script.
func (s *RedisReactionStore) Apply(ctx context.Context, commentID, userID uuid.UUID, target comment.Reaction) (comment.ReactionApplyResult, error) {
	res, err := applyScript.Run(ctx, s.client,
		[]string{s.countsKey(commentID), s.reactionsKey(commentID), s.dirtyKey()},
		userID.String(), int(target), commentID.String(),
	).Int64Slice()
	if err != nil {
		return comment.ReactionApplyResult{}, fmt.Errorf("reaction apply: %w", err)
	}
	if len(res) != 4 {
		return comment.ReactionApplyResult{}, fmt.Errorf("reaction apply: unexpected result length %d", len(res))
	}
	return comment.ReactionApplyResult{
		Previous: comment.Reaction(res[0]),
		Current:  comment.Reaction(res[1]),
		Counts: comment.ReactionCounts{
			Likes:    int(res[2]),
			Dislikes: int(res[3]),
		},
	}, nil
}

// Counts returns the cached counts for one comment. Missing keys return
// zero counts; callers are expected to hydrate from the DB on cache miss
// before relying on this value.
func (s *RedisReactionStore) Counts(ctx context.Context, commentID uuid.UUID) (comment.ReactionCounts, error) {
	res, err := s.client.HMGet(ctx, s.countsKey(commentID), "likes", "dislikes").Result()
	if err != nil {
		return comment.ReactionCounts{}, fmt.Errorf("counts: %w", err)
	}
	return parseCountsFields(res), nil
}

// BatchCounts loads counts for many comments in one pipelined batch.
func (s *RedisReactionStore) BatchCounts(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]comment.ReactionCounts, error) {
	if len(ids) == 0 {
		return map[uuid.UUID]comment.ReactionCounts{}, nil
	}
	pipe := s.client.Pipeline()
	cmds := make([]*redis.SliceCmd, len(ids))
	for i, id := range ids {
		cmds[i] = pipe.HMGet(ctx, s.countsKey(id), "likes", "dislikes")
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return nil, fmt.Errorf("batch counts: %w", err)
	}
	out := make(map[uuid.UUID]comment.ReactionCounts, len(ids))
	for i, id := range ids {
		out[id] = parseCountsFields(cmds[i].Val())
	}
	return out, nil
}

// UserReaction returns the user's reaction on one comment.
func (s *RedisReactionStore) UserReaction(ctx context.Context, commentID, userID uuid.UUID) (comment.Reaction, error) {
	v, err := s.client.HGet(ctx, s.reactionsKey(commentID), userID.String()).Result()
	if errors.Is(err, redis.Nil) {
		return comment.ReactionNone, nil
	}
	if err != nil {
		return comment.ReactionNone, fmt.Errorf("user reaction: %w", err)
	}
	return parseReactionString(v), nil
}

// BatchUserReactions returns the user's reactions across many comments.
func (s *RedisReactionStore) BatchUserReactions(ctx context.Context, userID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]comment.Reaction, error) {
	if len(ids) == 0 {
		return map[uuid.UUID]comment.Reaction{}, nil
	}
	pipe := s.client.Pipeline()
	cmds := make([]*redis.StringCmd, len(ids))
	for i, id := range ids {
		cmds[i] = pipe.HGet(ctx, s.reactionsKey(id), userID.String())
	}
	if _, err := pipe.Exec(ctx); err != nil && !errors.Is(err, redis.Nil) {
		return nil, fmt.Errorf("batch user reactions: %w", err)
	}
	out := make(map[uuid.UUID]comment.Reaction, len(ids))
	for i, id := range ids {
		v, err := cmds[i].Result()
		if errors.Is(err, redis.Nil) || v == "" {
			out[id] = comment.ReactionNone
			continue
		}
		out[id] = parseReactionString(v)
	}
	return out, nil
}

// Hydrate seeds counts and user reactions for one comment. Used on cold
// cache reads so the next Apply call has accurate state.
func (s *RedisReactionStore) Hydrate(ctx context.Context, commentID uuid.UUID, counts comment.ReactionCounts, reactions []comment.ReactionRow) error {
	pipe := s.client.Pipeline()
	pipe.HSet(ctx, s.countsKey(commentID),
		"likes", fmt.Sprintf("%d", counts.Likes),
		"dislikes", fmt.Sprintf("%d", counts.Dislikes),
	)
	if len(reactions) > 0 {
		args := make([]any, 0, len(reactions)*2)
		for _, r := range reactions {
			args = append(args, r.UserID.String(), fmt.Sprintf("%d", int(r.Reaction)))
		}
		pipe.HSet(ctx, s.reactionsKey(commentID), args...)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("hydrate: %w", err)
	}
	return nil
}

// DrainDirty returns up to limit pending comment IDs and removes them from
// the dirty set. The scheduler calls this periodically to drive write-back.
func (s *RedisReactionStore) DrainDirty(ctx context.Context, limit int) ([]uuid.UUID, error) {
	if limit <= 0 {
		return nil, nil
	}
	// SPOP atomically removes up to `limit` random members. Atomic with the
	// rest of Apply (which uses SADD), so we cannot lose updates: any
	// reaction applied after this SPOP will re-add the ID to dirty.
	ids, err := s.client.SPopN(ctx, s.dirtyKey(), int64(limit)).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, fmt.Errorf("drain dirty: %w", err)
	}
	out := make([]uuid.UUID, 0, len(ids))
	for _, s := range ids {
		id, err := uuid.Parse(s)
		if err != nil {
			continue
		}
		out = append(out, id)
	}
	return out, nil
}

// MarkDirty re-marks a comment ID for retry.
func (s *RedisReactionStore) MarkDirty(ctx context.Context, commentID uuid.UUID) error {
	return s.client.SAdd(ctx, s.dirtyKey(), commentID.String()).Err()
}

// ReactionsHashAll returns all (user, reaction) pairs from the reactions
// hash for one comment. Used by the flusher to reconcile the DB.
func (s *RedisReactionStore) ReactionsHashAll(ctx context.Context, commentID uuid.UUID) ([]comment.ReactionRow, error) {
	raw, err := s.client.HGetAll(ctx, s.reactionsKey(commentID)).Result()
	if err != nil {
		return nil, fmt.Errorf("reactions hash: %w", err)
	}
	out := make([]comment.ReactionRow, 0, len(raw))
	for u, v := range raw {
		uid, err := uuid.Parse(u)
		if err != nil {
			continue
		}
		r := parseReactionString(v)
		if r == comment.ReactionNone {
			continue
		}
		out = append(out, comment.ReactionRow{UserID: uid, Reaction: r})
	}
	return out, nil
}

func parseCountsFields(fields []any) comment.ReactionCounts {
	likes := parseIntField(fields, 0)
	dislikes := parseIntField(fields, 1)
	return comment.ReactionCounts{Likes: likes, Dislikes: dislikes}
}

func parseIntField(fields []any, idx int) int {
	if idx >= len(fields) || fields[idx] == nil {
		return 0
	}
	s, ok := fields[idx].(string)
	if !ok {
		return 0
	}
	n := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '-' && i == 0 {
			continue
		}
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	if len(s) > 0 && s[0] == '-' {
		n = -n
	}
	return n
}

func parseReactionString(s string) comment.Reaction {
	switch s {
	case "1":
		return comment.ReactionLike
	case "-1":
		return comment.ReactionDislike
	default:
		return comment.ReactionNone
	}
}
