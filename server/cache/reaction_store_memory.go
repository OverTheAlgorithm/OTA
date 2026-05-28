package cache

import (
	"context"
	"sync"

	"github.com/google/uuid"

	"ota/domain/comment"
)

// MemoryReactionStore is an in-memory comment.ReactionStore for tests.
// It mirrors the semantics of RedisReactionStore exactly so service-level
// tests can run without Redis. Production code uses RedisReactionStore.
type MemoryReactionStore struct {
	mu        sync.Mutex
	counts    map[uuid.UUID]comment.ReactionCounts
	reactions map[uuid.UUID]map[uuid.UUID]comment.Reaction
	dirty     map[uuid.UUID]struct{}
}

// NewMemoryReactionStore constructs an empty store.
func NewMemoryReactionStore() *MemoryReactionStore {
	return &MemoryReactionStore{
		counts:    make(map[uuid.UUID]comment.ReactionCounts),
		reactions: make(map[uuid.UUID]map[uuid.UUID]comment.Reaction),
		dirty:     make(map[uuid.UUID]struct{}),
	}
}

// Apply mirrors RedisReactionStore.Apply.
func (s *MemoryReactionStore) Apply(_ context.Context, commentID, userID uuid.UUID, target comment.Reaction) (comment.ReactionApplyResult, error) {
	if !target.Valid() {
		return comment.ReactionApplyResult{}, comment.ErrInvalidReaction
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	users := s.reactions[commentID]
	if users == nil {
		users = make(map[uuid.UUID]comment.Reaction)
		s.reactions[commentID] = users
	}
	prev := users[userID]
	counts := s.counts[commentID]

	if prev == target {
		return comment.ReactionApplyResult{Previous: prev, Current: target, Counts: counts}, nil
	}

	switch prev {
	case comment.ReactionLike:
		counts.Likes--
	case comment.ReactionDislike:
		counts.Dislikes--
	}
	switch target {
	case comment.ReactionLike:
		counts.Likes++
	case comment.ReactionDislike:
		counts.Dislikes++
	}
	if counts.Likes < 0 {
		counts.Likes = 0
	}
	if counts.Dislikes < 0 {
		counts.Dislikes = 0
	}

	if target == comment.ReactionNone {
		delete(users, userID)
	} else {
		users[userID] = target
	}
	s.counts[commentID] = counts
	s.dirty[commentID] = struct{}{}

	return comment.ReactionApplyResult{Previous: prev, Current: target, Counts: counts}, nil
}

// Counts mirrors RedisReactionStore.Counts.
func (s *MemoryReactionStore) Counts(_ context.Context, commentID uuid.UUID) (comment.ReactionCounts, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.counts[commentID], nil
}

// BatchCounts mirrors RedisReactionStore.BatchCounts.
func (s *MemoryReactionStore) BatchCounts(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]comment.ReactionCounts, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[uuid.UUID]comment.ReactionCounts, len(ids))
	for _, id := range ids {
		out[id] = s.counts[id]
	}
	return out, nil
}

// UserReaction mirrors RedisReactionStore.UserReaction.
func (s *MemoryReactionStore) UserReaction(_ context.Context, commentID, userID uuid.UUID) (comment.Reaction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if users, ok := s.reactions[commentID]; ok {
		return users[userID], nil
	}
	return comment.ReactionNone, nil
}

// BatchUserReactions mirrors RedisReactionStore.BatchUserReactions.
func (s *MemoryReactionStore) BatchUserReactions(_ context.Context, userID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]comment.Reaction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[uuid.UUID]comment.Reaction, len(ids))
	for _, id := range ids {
		if users, ok := s.reactions[id]; ok {
			out[id] = users[userID]
		}
	}
	return out, nil
}

// Hydrate mirrors RedisReactionStore.Hydrate.
func (s *MemoryReactionStore) Hydrate(_ context.Context, commentID uuid.UUID, counts comment.ReactionCounts, rows []comment.ReactionRow) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.counts[commentID] = counts
	users := s.reactions[commentID]
	if users == nil {
		users = make(map[uuid.UUID]comment.Reaction)
		s.reactions[commentID] = users
	}
	for _, r := range rows {
		users[r.UserID] = r.Reaction
	}
	return nil
}

// DrainDirty mirrors RedisReactionStore.DrainDirty.
func (s *MemoryReactionStore) DrainDirty(_ context.Context, limit int) ([]uuid.UUID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]uuid.UUID, 0, len(s.dirty))
	for id := range s.dirty {
		if limit > 0 && len(out) >= limit {
			break
		}
		out = append(out, id)
		delete(s.dirty, id)
	}
	return out, nil
}

// MarkDirty mirrors RedisReactionStore.MarkDirty.
func (s *MemoryReactionStore) MarkDirty(_ context.Context, commentID uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dirty[commentID] = struct{}{}
	return nil
}

// ReactionsHashAll mirrors RedisReactionStore.ReactionsHashAll. Provided so
// the flusher's reconcile pass works against the memory store too.
func (s *MemoryReactionStore) ReactionsHashAll(_ context.Context, commentID uuid.UUID) ([]comment.ReactionRow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	users := s.reactions[commentID]
	out := make([]comment.ReactionRow, 0, len(users))
	for uid, r := range users {
		if r == comment.ReactionNone {
			continue
		}
		out = append(out, comment.ReactionRow{UserID: uid, Reaction: r})
	}
	return out, nil
}
