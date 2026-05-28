package comment

import (
	"context"

	"github.com/google/uuid"
)

// Repository persists comment rows. It does not own reaction state — that
// lives behind ReactionStore and is only flushed back through ApplyCounters
// and UpsertReactions during scheduled write-back passes.
type Repository interface {
	// InsertRoot creates a new depth-0 comment. The group_id must equal
	// the comment's own id (callers commonly let the storage layer
	// generate the UUID; passing one in is supported for tests).
	InsertRoot(ctx context.Context, c Comment) (Comment, error)

	// InsertReply creates a depth-1 comment. group_id and parent_id must
	// already point at existing rows; depth is forced to 1.
	InsertReply(ctx context.Context, c Comment) (Comment, error)

	// GetByID fetches a single comment with author fields hydrated. It
	// returns ErrCommentNotFound if no row matches.
	GetByID(ctx context.Context, id uuid.UUID) (Comment, error)

	// ListRoots returns up to limit depth-0 comments for a target, ordered
	// by the requested sort. cursor is the encoded keyset cursor from the
	// previous page (empty for the first page).
	ListRoots(ctx context.Context, target TargetType, targetID uuid.UUID, sort SortOrder, cursor string, limit int) (RootPage, error)

	// ListReplies returns up to limit depth-1 comments for a group,
	// ordered by rank_key ascending. cursor is the last rank_key seen
	// (empty for the first page).
	ListReplies(ctx context.Context, groupID uuid.UUID, cursor string, limit int) (ReplyPage, error)

	// LastReplyRankKey returns the largest rank_key currently stored in
	// the group. Empty when the group has no replies yet.
	LastReplyRankKey(ctx context.Context, groupID uuid.UUID) (string, error)

	// CountReplies returns the number of (non-deleted) replies per group.
	// Missing keys in the result mean zero replies.
	CountReplies(ctx context.Context, groupIDs []uuid.UUID) (map[uuid.UUID]int, error)

	// UpdateContent overwrites a comment's content. The caller must have
	// already verified ownership via GetByID.
	UpdateContent(ctx context.Context, id uuid.UUID, newContent string) error

	// SoftDelete marks the row deleted_at = NOW(). The row is preserved
	// so the reply tree stays intact and rank ordering is undisturbed.
	SoftDelete(ctx context.Context, id uuid.UUID) error

	// ApplyCounters overwrites likes_count and dislikes_count from the
	// Redis cache during write-back.
	ApplyCounters(ctx context.Context, id uuid.UUID, likes, dislikes int) error

	// UpsertReactions replaces all reaction rows for one comment with the
	// supplied set. Reactions in DB but not in the set are deleted, so
	// the flusher can use this to reconcile Redis truth.
	UpsertReactions(ctx context.Context, commentID uuid.UUID, reactions []ReactionRow) error
}

// RootPage is the paginated response for ListRoots.
type RootPage struct {
	Items      []Comment
	NextCursor string // empty when no more pages
}

// ReplyPage is the paginated response for ListReplies.
type ReplyPage struct {
	Items      []Comment
	NextCursor string // empty when no more pages
}

// ReactionRow is the storage-layer reaction record.
type ReactionRow struct {
	UserID   uuid.UUID
	Reaction Reaction
}
