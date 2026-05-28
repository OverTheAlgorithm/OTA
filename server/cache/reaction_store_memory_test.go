package cache

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"ota/domain/comment"
)

func TestMemoryReactionStore_LikeFromNone(t *testing.T) {
	s := NewMemoryReactionStore()
	cID, uID := uuid.New(), uuid.New()
	res, err := s.Apply(context.Background(), cID, uID, comment.ReactionLike)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	assertResult(t, res, comment.ReactionNone, comment.ReactionLike, 1, 0)
}

func TestMemoryReactionStore_UnlikeBacksOff(t *testing.T) {
	s := NewMemoryReactionStore()
	cID, uID := uuid.New(), uuid.New()
	_, _ = s.Apply(context.Background(), cID, uID, comment.ReactionLike)
	res, _ := s.Apply(context.Background(), cID, uID, comment.ReactionNone)
	assertResult(t, res, comment.ReactionLike, comment.ReactionNone, 0, 0)
}

func TestMemoryReactionStore_DislikeFromNone(t *testing.T) {
	s := NewMemoryReactionStore()
	cID, uID := uuid.New(), uuid.New()
	res, _ := s.Apply(context.Background(), cID, uID, comment.ReactionDislike)
	assertResult(t, res, comment.ReactionNone, comment.ReactionDislike, 0, 1)
}

func TestMemoryReactionStore_LikeToDislikeSwap(t *testing.T) {
	s := NewMemoryReactionStore()
	cID, uID := uuid.New(), uuid.New()
	_, _ = s.Apply(context.Background(), cID, uID, comment.ReactionLike)
	res, _ := s.Apply(context.Background(), cID, uID, comment.ReactionDislike)
	// Swap should decrement likes and increment dislikes in one atomic call.
	assertResult(t, res, comment.ReactionLike, comment.ReactionDislike, 0, 1)
}

func TestMemoryReactionStore_DislikeToLikeSwap(t *testing.T) {
	s := NewMemoryReactionStore()
	cID, uID := uuid.New(), uuid.New()
	_, _ = s.Apply(context.Background(), cID, uID, comment.ReactionDislike)
	res, _ := s.Apply(context.Background(), cID, uID, comment.ReactionLike)
	assertResult(t, res, comment.ReactionDislike, comment.ReactionLike, 1, 0)
}

func TestMemoryReactionStore_RepeatedSameValueIsNoOp(t *testing.T) {
	s := NewMemoryReactionStore()
	cID, uID := uuid.New(), uuid.New()
	_, _ = s.Apply(context.Background(), cID, uID, comment.ReactionLike)
	res, _ := s.Apply(context.Background(), cID, uID, comment.ReactionLike)
	// Repeated like must NOT double-count.
	assertResult(t, res, comment.ReactionLike, comment.ReactionLike, 1, 0)
}

func TestMemoryReactionStore_MultipleUsers_SumCorrectly(t *testing.T) {
	s := NewMemoryReactionStore()
	cID := uuid.New()
	u1, u2, u3 := uuid.New(), uuid.New(), uuid.New()
	_, _ = s.Apply(context.Background(), cID, u1, comment.ReactionLike)
	_, _ = s.Apply(context.Background(), cID, u2, comment.ReactionLike)
	_, _ = s.Apply(context.Background(), cID, u3, comment.ReactionDislike)
	counts, _ := s.Counts(context.Background(), cID)
	if counts.Likes != 2 || counts.Dislikes != 1 {
		t.Errorf("counts = %+v, want {Likes:2 Dislikes:1}", counts)
	}
}

func TestMemoryReactionStore_DrainDirtyRemovesIDs(t *testing.T) {
	s := NewMemoryReactionStore()
	cID := uuid.New()
	_, _ = s.Apply(context.Background(), cID, uuid.New(), comment.ReactionLike)

	drained, _ := s.DrainDirty(context.Background(), 10)
	if len(drained) != 1 || drained[0] != cID {
		t.Errorf("drained = %v, want [%v]", drained, cID)
	}
	// Second drain should be empty until another modification.
	drained2, _ := s.DrainDirty(context.Background(), 10)
	if len(drained2) != 0 {
		t.Errorf("second drain = %v, want []", drained2)
	}
}

func TestMemoryReactionStore_DrainDirtyHonoursLimit(t *testing.T) {
	s := NewMemoryReactionStore()
	for i := 0; i < 5; i++ {
		_, _ = s.Apply(context.Background(), uuid.New(), uuid.New(), comment.ReactionLike)
	}
	first, _ := s.DrainDirty(context.Background(), 3)
	second, _ := s.DrainDirty(context.Background(), 3)
	if len(first)+len(second) != 5 {
		t.Errorf("total drained = %d, want 5 (first=%d second=%d)",
			len(first)+len(second), len(first), len(second))
	}
	if len(first) > 3 {
		t.Errorf("first drain returned %d, exceeds limit 3", len(first))
	}
}

func TestMemoryReactionStore_HydrateSeedsCounts(t *testing.T) {
	s := NewMemoryReactionStore()
	cID := uuid.New()
	uID := uuid.New()
	_ = s.Hydrate(context.Background(), cID, comment.ReactionCounts{Likes: 5, Dislikes: 2}, []comment.ReactionRow{
		{UserID: uID, Reaction: comment.ReactionLike},
	})
	counts, _ := s.Counts(context.Background(), cID)
	if counts.Likes != 5 || counts.Dislikes != 2 {
		t.Errorf("hydrated counts = %+v, want {Likes:5 Dislikes:2}", counts)
	}
	r, _ := s.UserReaction(context.Background(), cID, uID)
	if r != comment.ReactionLike {
		t.Errorf("hydrated reaction = %v, want Like", r)
	}
}

func TestMemoryReactionStore_BatchCountsAndReactions(t *testing.T) {
	s := NewMemoryReactionStore()
	c1, c2 := uuid.New(), uuid.New()
	u1 := uuid.New()
	_, _ = s.Apply(context.Background(), c1, u1, comment.ReactionLike)
	_, _ = s.Apply(context.Background(), c2, u1, comment.ReactionDislike)

	counts, _ := s.BatchCounts(context.Background(), []uuid.UUID{c1, c2})
	if counts[c1].Likes != 1 || counts[c2].Dislikes != 1 {
		t.Errorf("batch counts wrong: %+v", counts)
	}

	reactions, _ := s.BatchUserReactions(context.Background(), u1, []uuid.UUID{c1, c2})
	if reactions[c1] != comment.ReactionLike || reactions[c2] != comment.ReactionDislike {
		t.Errorf("batch reactions wrong: %+v", reactions)
	}
}

func TestMemoryReactionStore_InvalidReactionRejected(t *testing.T) {
	s := NewMemoryReactionStore()
	_, err := s.Apply(context.Background(), uuid.New(), uuid.New(), comment.Reaction(5))
	if err == nil {
		t.Error("expected error for invalid reaction value")
	}
}

func assertResult(t *testing.T, res comment.ReactionApplyResult, wantPrev, wantCur comment.Reaction, wantLikes, wantDislikes int) {
	t.Helper()
	if res.Previous != wantPrev {
		t.Errorf("Previous = %d, want %d", res.Previous, wantPrev)
	}
	if res.Current != wantCur {
		t.Errorf("Current = %d, want %d", res.Current, wantCur)
	}
	if res.Counts.Likes != wantLikes {
		t.Errorf("Likes = %d, want %d", res.Counts.Likes, wantLikes)
	}
	if res.Counts.Dislikes != wantDislikes {
		t.Errorf("Dislikes = %d, want %d", res.Counts.Dislikes, wantDislikes)
	}
}
