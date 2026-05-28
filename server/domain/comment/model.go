package comment

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// TargetType identifies which kind of content a comment is attached to.
// Two kinds are supported: news topics (context_items rows) and editor
// picks (editor_posts rows). The string values are persisted to the
// database, so changing them is a breaking change.
type TargetType string

const (
	TargetTopic      TargetType = "topic"
	TargetEditorPick TargetType = "editor_pick"
)

// Valid reports whether t is one of the recognized target types.
func (t TargetType) Valid() bool {
	return t == TargetTopic || t == TargetEditorPick
}

// SortOrder controls how root comments are returned. Replies are always
// sorted by rank_key ascending and ignore this value.
type SortOrder string

const (
	// SortPopular orders roots by likes_count DESC, created_at DESC.
	// Tiebreakers cascade to id DESC so cursors stay deterministic.
	SortPopular SortOrder = "popular"
	// SortRecent orders roots by created_at DESC, id DESC.
	SortRecent SortOrder = "recent"
)

// Valid reports whether s is one of the recognized sort orders.
func (s SortOrder) Valid() bool {
	return s == SortPopular || s == SortRecent
}

// Reaction represents a user's reaction state on a comment.
// 0 means no reaction; the constants below are the only non-zero values.
type Reaction int8

const (
	ReactionNone    Reaction = 0
	ReactionLike    Reaction = 1
	ReactionDislike Reaction = -1
)

// Valid reports whether r is one of the recognized reactions.
func (r Reaction) Valid() bool {
	return r == ReactionNone || r == ReactionLike || r == ReactionDislike
}

// Depth limits used for two-level threading.
const (
	MaxDepth        = 1    // root + one reply level (YouTube-style)
	MaxContentLen   = 2000 // bytes; matches DB CHECK constraint
	MinContentLen   = 1
	DefaultPageSize = 10
	MaxPageSize     = 50
)

// Sentinel errors returned by the service. Callers should compare with
// errors.Is to allow wrapping.
var (
	ErrCommentNotFound    = errors.New("comment: not found")
	ErrNotOwner           = errors.New("comment: caller is not the owner")
	ErrAlreadyDeleted     = errors.New("comment: already deleted")
	ErrInvalidTarget      = errors.New("comment: invalid target")
	ErrInvalidContent     = errors.New("comment: invalid content")
	ErrInvalidParent      = errors.New("comment: invalid parent")
	ErrInvalidReaction    = errors.New("comment: invalid reaction value")
	ErrInvalidPageSize    = errors.New("comment: invalid page size")
	ErrTargetNotExist     = errors.New("comment: target does not exist")
	ErrAnonymousForbidden = errors.New("comment: anonymous users cannot post")
)

// Comment is the canonical in-memory representation. Author fields are
// hydrated by the storage layer via a JOIN against users.
type Comment struct {
	ID            uuid.UUID
	TargetType    TargetType
	TargetID      uuid.UUID
	UserID        uuid.UUID
	GroupID       uuid.UUID
	ParentID      *uuid.UUID
	Depth         int
	RankKey       string
	Content       string
	LikesCount    int
	DislikesCount int
	EditedAt      *time.Time
	DeletedAt     *time.Time
	CreatedAt     time.Time

	// Author fields populated by storage JOINs. Empty when storage does
	// not include them (e.g. internal lookups). Comments always display
	// the nickname — pen_name is scoped to editor-pick author bylines.
	AuthorNickname     string
	AuthorProfileImage string
}

// IsDeleted reports whether the comment has been soft-deleted.
func (c Comment) IsDeleted() bool {
	return c.DeletedAt != nil
}

// AuthorDisplayName returns the user-visible name for the author. Comments
// surface the nickname regardless of any pen name the user may have set
// for their editor-pick byline.
func (c Comment) AuthorDisplayName() string {
	return strings.TrimSpace(c.AuthorNickname)
}

// NormalizeContent trims surrounding whitespace and collapses Windows-style
// newlines. It does not modify embedded whitespace beyond trimming.
func NormalizeContent(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.TrimSpace(s)
}

// ValidateContent enforces the length contract used by the DB CHECK
// constraint. Empty content after normalization is rejected.
func ValidateContent(s string) error {
	if l := len(s); l < MinContentLen || l > MaxContentLen {
		return ErrInvalidContent
	}
	return nil
}

// ResolveReplyDepth implements the two-level depth rule: a reply to a
// depth-1 comment is recorded as a sibling at depth 1 rather than going
// deeper. Returns the effective (depth, parent_id) the new comment should
// use, given the parent the user clicked "reply" on.
func ResolveReplyDepth(parentDepth int, parentID, parentParentID *uuid.UUID) (depth int, effectiveParentID *uuid.UUID, err error) {
	if parentID == nil {
		return 0, nil, ErrInvalidParent
	}
	switch parentDepth {
	case 0:
		// Root parent: new comment goes one level deeper.
		return 1, parentID, nil
	case MaxDepth:
		// Reply parent: attach to that parent's parent so the new
		// comment stays at depth 1 (the YouTube model).
		if parentParentID == nil {
			return 0, nil, ErrInvalidParent
		}
		return MaxDepth, parentParentID, nil
	default:
		return 0, nil, ErrInvalidParent
	}
}

// ClampLimit normalizes a caller-supplied page size into the supported
// range. Negative or zero inputs fall back to DefaultPageSize.
func ClampLimit(n int) int {
	if n <= 0 {
		return DefaultPageSize
	}
	if n > MaxPageSize {
		return MaxPageSize
	}
	return n
}
