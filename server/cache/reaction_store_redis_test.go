//go:build integration

package cache

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/testcontainers/testcontainers-go"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"

	"ota/domain/comment"
)

func setupRedisStore(t *testing.T) (*RedisReactionStore, func()) {
	t.Helper()
	ctx := context.Background()
	c, err := tcredis.Run(ctx, "redis:7-alpine")
	if err != nil {
		t.Skipf("redis container unavailable: %v", err)
	}
	endpoint, err := c.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}
	opts, err := redis.ParseURL(endpoint)
	if err != nil {
		t.Fatalf("parse redis URL: %v", err)
	}
	client := redis.NewClient(opts)
	store := NewRedisReactionStore(client, "test:comment:")

	cleanup := func() {
		_ = client.Close()
		_ = testcontainers.TerminateContainer(c)
	}
	return store, cleanup
}

func TestRedisReactionStore_LikeFromNone(t *testing.T) {
	store, done := setupRedisStore(t)
	defer done()
	cID, uID := uuid.New(), uuid.New()
	res, err := store.Apply(context.Background(), cID, uID, comment.ReactionLike)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.Previous != comment.ReactionNone || res.Current != comment.ReactionLike {
		t.Errorf("transitions wrong: %+v", res)
	}
	if res.Counts.Likes != 1 || res.Counts.Dislikes != 0 {
		t.Errorf("counts wrong: %+v", res.Counts)
	}
}

func TestRedisReactionStore_LikeToDislikeSwapAtomic(t *testing.T) {
	store, done := setupRedisStore(t)
	defer done()
	cID, uID := uuid.New(), uuid.New()
	_, _ = store.Apply(context.Background(), cID, uID, comment.ReactionLike)
	res, err := store.Apply(context.Background(), cID, uID, comment.ReactionDislike)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.Previous != comment.ReactionLike || res.Current != comment.ReactionDislike {
		t.Errorf("transitions wrong: %+v", res)
	}
	if res.Counts.Likes != 0 || res.Counts.Dislikes != 1 {
		t.Errorf("counts after swap: %+v, want {Likes:0 Dislikes:1}", res.Counts)
	}
}

func TestRedisReactionStore_RepeatIsNoOp(t *testing.T) {
	store, done := setupRedisStore(t)
	defer done()
	cID, uID := uuid.New(), uuid.New()
	_, _ = store.Apply(context.Background(), cID, uID, comment.ReactionLike)
	res, _ := store.Apply(context.Background(), cID, uID, comment.ReactionLike)
	if res.Counts.Likes != 1 {
		t.Errorf("repeated like double-counted: %+v", res.Counts)
	}
}

func TestRedisReactionStore_UnreactClears(t *testing.T) {
	store, done := setupRedisStore(t)
	defer done()
	cID, uID := uuid.New(), uuid.New()
	_, _ = store.Apply(context.Background(), cID, uID, comment.ReactionDislike)
	res, _ := store.Apply(context.Background(), cID, uID, comment.ReactionNone)
	if res.Counts.Dislikes != 0 || res.Counts.Likes != 0 {
		t.Errorf("after unreact: %+v, want zeros", res.Counts)
	}
	got, _ := store.UserReaction(context.Background(), cID, uID)
	if got != comment.ReactionNone {
		t.Errorf("user reaction after unreact = %v, want None", got)
	}
}

func TestRedisReactionStore_DrainDirty(t *testing.T) {
	store, done := setupRedisStore(t)
	defer done()
	c1, c2 := uuid.New(), uuid.New()
	_, _ = store.Apply(context.Background(), c1, uuid.New(), comment.ReactionLike)
	_, _ = store.Apply(context.Background(), c2, uuid.New(), comment.ReactionDislike)

	drained, _ := store.DrainDirty(context.Background(), 10)
	if len(drained) != 2 {
		t.Errorf("drained = %d ids, want 2", len(drained))
	}
	// Verify counts persist after drain (drain only clears the dirty set).
	counts, _ := store.Counts(context.Background(), c1)
	if counts.Likes != 1 {
		t.Errorf("counts cleared after drain: %+v", counts)
	}
}

func TestRedisReactionStore_BatchCountsAndReactions(t *testing.T) {
	store, done := setupRedisStore(t)
	defer done()
	c1, c2 := uuid.New(), uuid.New()
	u1 := uuid.New()
	_, _ = store.Apply(context.Background(), c1, u1, comment.ReactionLike)
	_, _ = store.Apply(context.Background(), c2, u1, comment.ReactionDislike)

	counts, _ := store.BatchCounts(context.Background(), []uuid.UUID{c1, c2})
	if counts[c1].Likes != 1 || counts[c2].Dislikes != 1 {
		t.Errorf("batch counts wrong: %+v", counts)
	}
	reactions, _ := store.BatchUserReactions(context.Background(), u1, []uuid.UUID{c1, c2})
	if reactions[c1] != comment.ReactionLike || reactions[c2] != comment.ReactionDislike {
		t.Errorf("batch reactions wrong: %+v", reactions)
	}
}

func TestRedisReactionStore_HydrateSeeds(t *testing.T) {
	store, done := setupRedisStore(t)
	defer done()
	cID := uuid.New()
	u1 := uuid.New()
	_ = store.Hydrate(context.Background(), cID, comment.ReactionCounts{Likes: 10, Dislikes: 3}, []comment.ReactionRow{
		{UserID: u1, Reaction: comment.ReactionLike},
	})
	counts, _ := store.Counts(context.Background(), cID)
	if counts.Likes != 10 || counts.Dislikes != 3 {
		t.Errorf("hydrated counts: %+v", counts)
	}
	r, _ := store.UserReaction(context.Background(), cID, u1)
	if r != comment.ReactionLike {
		t.Errorf("hydrated reaction = %v, want Like", r)
	}
}

func TestRedisReactionStore_ConcurrentApplyAtomic(t *testing.T) {
	// Lua atomicity: many goroutines hitting the same comment must not
	// produce counts drift.
	store, done := setupRedisStore(t)
	defer done()
	cID := uuid.New()
	const workers = 50
	ch := make(chan error, workers)
	for i := 0; i < workers; i++ {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_, err := store.Apply(ctx, cID, uuid.New(), comment.ReactionLike)
			ch <- err
		}()
	}
	for i := 0; i < workers; i++ {
		if err := <-ch; err != nil {
			t.Errorf("worker err: %v", err)
		}
	}
	counts, _ := store.Counts(context.Background(), cID)
	if counts.Likes != workers {
		t.Errorf("concurrent likes = %d, want %d (lost updates?)", counts.Likes, workers)
	}
}
