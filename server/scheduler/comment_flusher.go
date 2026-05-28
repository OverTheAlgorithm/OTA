package scheduler

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"

	"ota/domain/comment"
)

// reactionFullStore is the subset of cache.ReactionStore the flusher needs.
// We declare it locally so the scheduler does not have to import the cache
// package, keeping dependency direction one-way.
type reactionFullStore interface {
	comment.ReactionStore
	// ReactionsHashAll lists all (user, reaction) pairs for one comment.
	// Implementations: cache.RedisReactionStore, cache.MemoryReactionStore.
	ReactionsHashAll(ctx context.Context, commentID uuid.UUID) ([]comment.ReactionRow, error)
}

// CommentFlusherConfig tunes the flusher.
type CommentFlusherConfig struct {
	// Interval between scheduled flushes. Defaults to 10s.
	Interval time.Duration
	// Maximum dirty IDs drained per tick. Defaults to 256.
	BatchSize int
}

// CommentFlusher periodically copies cached reaction state into Postgres.
type CommentFlusher struct {
	store reactionFullStore
	repo  comment.Repository
	cfg   CommentFlusherConfig

	stopOnce sync.Once
	stopCh   chan struct{}
}

// NewCommentFlusher constructs a flusher.
func NewCommentFlusher(store reactionFullStore, repo comment.Repository, cfg CommentFlusherConfig) *CommentFlusher {
	if cfg.Interval <= 0 {
		cfg.Interval = 10 * time.Second
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 256
	}
	return &CommentFlusher{
		store:  store,
		repo:   repo,
		cfg:    cfg,
		stopCh: make(chan struct{}),
	}
}

// Start runs a bootstrap flush and then a ticker until ctx is cancelled or
// Stop is called.
func (f *CommentFlusher) Start(ctx context.Context) {
	go func() {
		// Bootstrap: any IDs still in dirty after a crash get flushed
		// immediately on startup.
		f.runOnce(ctx, "bootstrap")

		ticker := time.NewTicker(f.cfg.Interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-f.stopCh:
				return
			case <-ticker.C:
				f.runOnce(ctx, "tick")
			}
		}
	}()
}

// Stop signals the goroutine to exit. Safe to call multiple times.
func (f *CommentFlusher) Stop() {
	f.stopOnce.Do(func() { close(f.stopCh) })
}

// runOnce drains one batch and writes it back to the DB. Errors are
// logged; dirty IDs that fail are re-marked so the next tick retries.
func (f *CommentFlusher) runOnce(ctx context.Context, label string) {
	ids, err := f.store.DrainDirty(ctx, f.cfg.BatchSize)
	if err != nil {
		slog.Error("comment flusher drain", "label", label, "error", err)
		return
	}
	if len(ids) == 0 {
		return
	}
	slog.Debug("comment flusher draining", "label", label, "count", len(ids))

	flushed := 0
	for _, id := range ids {
		if err := f.flushOne(ctx, id); err != nil {
			slog.Error("comment flusher flush one", "id", id, "error", err)
			// Re-mark for retry on next tick.
			if reMarkErr := f.store.MarkDirty(ctx, id); reMarkErr != nil {
				slog.Error("comment flusher re-mark", "id", id, "error", reMarkErr)
			}
			continue
		}
		flushed++
	}
	if flushed > 0 {
		slog.Info("comment flusher synced", "label", label, "count", flushed)
	}
}

// flushOne copies one comment's cached state to the DB.
func (f *CommentFlusher) flushOne(ctx context.Context, id uuid.UUID) error {
	counts, err := f.store.Counts(ctx, id)
	if err != nil {
		return err
	}
	reactions, err := f.store.ReactionsHashAll(ctx, id)
	if err != nil {
		return err
	}
	if err := f.repo.UpsertReactions(ctx, id, reactions); err != nil {
		if errors.Is(err, context.Canceled) {
			return err
		}
		return err
	}
	return f.repo.ApplyCounters(ctx, id, counts.Likes, counts.Dislikes)
}

// FlushNow drains and writes one batch synchronously. Useful for tests.
func (f *CommentFlusher) FlushNow(ctx context.Context) error {
	f.runOnce(ctx, "manual")
	return nil
}
