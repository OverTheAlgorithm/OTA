package comment

import (
	"context"

	"github.com/google/uuid"
)

// ReactionCounts is the like/dislike aggregate for one comment.
type ReactionCounts struct {
	Likes    int
	Dislikes int
}

// ReactionApplyResult is returned by ReactionStore.Apply so callers can
// echo the new state back to clients in one request without an extra read.
type ReactionApplyResult struct {
	Previous Reaction
	Current  Reaction
	Counts   ReactionCounts
}

// ReactionStore owns the source-of-truth state for likes and dislikes.
// The intended implementation is Redis-backed (with periodic write-back
// to the DB) but the interface is satisfied by an in-memory implementation
// for tests.
type ReactionStore interface {
	// Apply atomically transitions the user's reaction on the comment to
	// target and returns the resulting counts. Swapping from like to
	// dislike (or vice-versa) is one atomic step. target=ReactionNone
	// clears any existing reaction.
	Apply(ctx context.Context, commentID, userID uuid.UUID, target Reaction) (ReactionApplyResult, error)

	// Counts returns the cached counts for one comment. Implementations
	// that cache miss should hydrate from the DB transparently via the
	// provided ColdLoader callback.
	Counts(ctx context.Context, commentID uuid.UUID) (ReactionCounts, error)

	// BatchCounts loads counts for many comments in one round-trip.
	BatchCounts(ctx context.Context, commentIDs []uuid.UUID) (map[uuid.UUID]ReactionCounts, error)

	// UserReaction returns the user's current reaction on the comment.
	UserReaction(ctx context.Context, commentID, userID uuid.UUID) (Reaction, error)

	// BatchUserReactions returns the user's reactions for a set of
	// comments in one round-trip. Missing entries mean ReactionNone.
	BatchUserReactions(ctx context.Context, userID uuid.UUID, commentIDs []uuid.UUID) (map[uuid.UUID]Reaction, error)

	// Hydrate seeds cache state for one comment from a cold source. It is
	// safe to call multiple times; the implementation must not overwrite
	// state that is newer in the cache.
	Hydrate(ctx context.Context, commentID uuid.UUID, counts ReactionCounts, userReactions []ReactionRow) error

	// DrainDirty returns up to limit comment IDs that have been modified
	// since the last drain and removes them from the dirty set. The
	// scheduler uses this to drive write-back to the DB. Returned IDs
	// remain available in the cache so subsequent reads stay fast.
	DrainDirty(ctx context.Context, limit int) ([]uuid.UUID, error)

	// MarkClean re-adds an ID to the dirty set if its write-back failed,
	// so the next scheduler tick retries.
	MarkDirty(ctx context.Context, commentID uuid.UUID) error
}
