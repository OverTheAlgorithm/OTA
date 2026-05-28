package comment

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// TargetValidator checks that a target_id refers to a real, visible record
// (a published context_item or published editor_post). Each domain
// provides one implementation; the service composes them so handlers do
// not have to reach across packages.
type TargetValidator interface {
	Exists(ctx context.Context, id uuid.UUID) (bool, error)
}

// Service holds the domain logic for comments. It is intentionally a thin
// orchestration layer: validation, depth resolution, and lexorank are pure
// helpers in this package; persistence lives in storage; reactions live in
// ReactionStore.
type Service struct {
	repo            Repository
	reactions       ReactionStore
	targets         map[TargetType]TargetValidator
	maxContentBytes int
}

// NewService constructs a Service.
func NewService(repo Repository, reactions ReactionStore, targets map[TargetType]TargetValidator) *Service {
	return &Service{
		repo:            repo,
		reactions:       reactions,
		targets:         targets,
		maxContentBytes: MaxContentLen,
	}
}

// Create writes a new comment authored by userID. When parentID is nil the
// comment is a depth-0 root; otherwise it is a depth-1 reply (replies to a
// reply collapse to siblings per ResolveReplyDepth).
func (s *Service) Create(ctx context.Context, userID uuid.UUID, target TargetType, targetID uuid.UUID, parentID *uuid.UUID, content string) (Comment, error) {
	if userID == uuid.Nil {
		return Comment{}, ErrAnonymousForbidden
	}
	if !target.Valid() {
		return Comment{}, ErrInvalidTarget
	}
	content = NormalizeContent(content)
	if err := ValidateContent(content); err != nil {
		return Comment{}, err
	}
	if err := s.assertTargetExists(ctx, target, targetID); err != nil {
		return Comment{}, err
	}

	if parentID == nil {
		return s.createRoot(ctx, userID, target, targetID, content)
	}
	return s.createReply(ctx, userID, target, targetID, *parentID, content)
}

func (s *Service) createRoot(ctx context.Context, userID uuid.UUID, target TargetType, targetID uuid.UUID, content string) (Comment, error) {
	id := uuid.New()
	row := Comment{
		ID:         id,
		TargetType: target,
		TargetID:   targetID,
		UserID:     userID,
		GroupID:    id,
		Depth:      0,
		RankKey:    First(),
		Content:    content,
	}
	saved, err := s.repo.InsertRoot(ctx, row)
	if err != nil {
		return Comment{}, fmt.Errorf("comment service create root: %w", err)
	}
	return saved, nil
}

func (s *Service) createReply(ctx context.Context, userID uuid.UUID, target TargetType, targetID uuid.UUID, parentID uuid.UUID, content string) (Comment, error) {
	parent, err := s.repo.GetByID(ctx, parentID)
	if err != nil {
		return Comment{}, fmt.Errorf("comment service load parent: %w", err)
	}
	if parent.TargetType != target || parent.TargetID != targetID {
		return Comment{}, ErrInvalidParent
	}
	if parent.IsDeleted() {
		// Allow replies under a deleted parent — common UX. The parent
		// row still exists, just with a tombstone marker.
	}
	depth, effectiveParent, err := ResolveReplyDepth(parent.Depth, &parent.ID, parent.ParentID)
	if err != nil {
		return Comment{}, err
	}
	lastRank, err := s.repo.LastReplyRankKey(ctx, parent.GroupID)
	if err != nil {
		return Comment{}, fmt.Errorf("comment service load last rank: %w", err)
	}
	rank, err := After(lastRank)
	if err != nil {
		return Comment{}, fmt.Errorf("comment service compute rank: %w", err)
	}
	row := Comment{
		ID:         uuid.New(),
		TargetType: target,
		TargetID:   targetID,
		UserID:     userID,
		GroupID:    parent.GroupID,
		ParentID:   effectiveParent,
		Depth:      depth,
		RankKey:    rank,
		Content:    content,
	}
	saved, err := s.repo.InsertReply(ctx, row)
	if err != nil {
		return Comment{}, fmt.Errorf("comment service create reply: %w", err)
	}
	return saved, nil
}

// Update overwrites a comment's content. The caller must be the author.
func (s *Service) Update(ctx context.Context, userID, commentID uuid.UUID, content string) (Comment, error) {
	if userID == uuid.Nil {
		return Comment{}, ErrAnonymousForbidden
	}
	content = NormalizeContent(content)
	if err := ValidateContent(content); err != nil {
		return Comment{}, err
	}
	existing, err := s.repo.GetByID(ctx, commentID)
	if err != nil {
		return Comment{}, err
	}
	if existing.UserID != userID {
		return Comment{}, ErrNotOwner
	}
	if existing.IsDeleted() {
		return Comment{}, ErrAlreadyDeleted
	}
	if err := s.repo.UpdateContent(ctx, commentID, content); err != nil {
		return Comment{}, fmt.Errorf("comment service update: %w", err)
	}
	return s.repo.GetByID(ctx, commentID)
}

// Delete soft-deletes a comment. The caller must be the author. Admins
// pass through a separate handler path that bypasses ownership checks.
func (s *Service) Delete(ctx context.Context, userID, commentID uuid.UUID) error {
	if userID == uuid.Nil {
		return ErrAnonymousForbidden
	}
	existing, err := s.repo.GetByID(ctx, commentID)
	if err != nil {
		return err
	}
	if existing.UserID != userID {
		return ErrNotOwner
	}
	if existing.IsDeleted() {
		return ErrAlreadyDeleted
	}
	if err := s.repo.SoftDelete(ctx, commentID); err != nil {
		return fmt.Errorf("comment service delete: %w", err)
	}
	return nil
}

// AdminDelete soft-deletes any comment without an ownership check.
func (s *Service) AdminDelete(ctx context.Context, commentID uuid.UUID) error {
	existing, err := s.repo.GetByID(ctx, commentID)
	if err != nil {
		return err
	}
	if existing.IsDeleted() {
		return ErrAlreadyDeleted
	}
	if err := s.repo.SoftDelete(ctx, commentID); err != nil {
		return fmt.Errorf("comment service admin delete: %w", err)
	}
	return nil
}

// React applies a like/dislike/clear transition. Like→dislike and
// dislike→like swaps are atomic. Calling with the user's existing reaction
// is a no-op that still returns the current counts.
func (s *Service) React(ctx context.Context, userID, commentID uuid.UUID, target Reaction) (ReactionApplyResult, error) {
	if userID == uuid.Nil {
		return ReactionApplyResult{}, ErrAnonymousForbidden
	}
	if !target.Valid() {
		return ReactionApplyResult{}, ErrInvalidReaction
	}
	existing, err := s.repo.GetByID(ctx, commentID)
	if err != nil {
		return ReactionApplyResult{}, err
	}
	if existing.IsDeleted() {
		return ReactionApplyResult{}, ErrAlreadyDeleted
	}
	return s.reactions.Apply(ctx, commentID, userID, target)
}

// ListRoots returns a page of root comments for a target.
func (s *Service) ListRoots(ctx context.Context, target TargetType, targetID uuid.UUID, sort SortOrder, cursor string, limit int) (RootPage, error) {
	if !target.Valid() {
		return RootPage{}, ErrInvalidTarget
	}
	if !sort.Valid() {
		sort = SortPopular
	}
	limit = ClampLimit(limit)
	return s.repo.ListRoots(ctx, target, targetID, sort, cursor, limit)
}

// ListReplies returns a page of replies under a group.
func (s *Service) ListReplies(ctx context.Context, groupID uuid.UUID, cursor string, limit int) (ReplyPage, error) {
	if groupID == uuid.Nil {
		return ReplyPage{}, ErrCommentNotFound
	}
	limit = ClampLimit(limit)
	return s.repo.ListReplies(ctx, groupID, cursor, limit)
}

// GetByID returns one comment with author fields hydrated.
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (Comment, error) {
	return s.repo.GetByID(ctx, id)
}

// ReplyCounts proxies to repo.CountReplies for handler convenience.
func (s *Service) ReplyCounts(ctx context.Context, groupIDs []uuid.UUID) (map[uuid.UUID]int, error) {
	if len(groupIDs) == 0 {
		return map[uuid.UUID]int{}, nil
	}
	return s.repo.CountReplies(ctx, groupIDs)
}

// BatchCounts proxies to ReactionStore.BatchCounts.
func (s *Service) BatchCounts(ctx context.Context, commentIDs []uuid.UUID) (map[uuid.UUID]ReactionCounts, error) {
	if len(commentIDs) == 0 {
		return map[uuid.UUID]ReactionCounts{}, nil
	}
	return s.reactions.BatchCounts(ctx, commentIDs)
}

// BatchUserReactions proxies to ReactionStore.BatchUserReactions.
func (s *Service) BatchUserReactions(ctx context.Context, userID uuid.UUID, commentIDs []uuid.UUID) (map[uuid.UUID]Reaction, error) {
	if userID == uuid.Nil || len(commentIDs) == 0 {
		return map[uuid.UUID]Reaction{}, nil
	}
	return s.reactions.BatchUserReactions(ctx, userID, commentIDs)
}

func (s *Service) assertTargetExists(ctx context.Context, target TargetType, targetID uuid.UUID) error {
	if s.targets == nil {
		return nil
	}
	validator, ok := s.targets[target]
	if !ok {
		return nil
	}
	exists, err := validator.Exists(ctx, targetID)
	if err != nil {
		return fmt.Errorf("comment service validate target: %w", err)
	}
	if !exists {
		return ErrTargetNotExist
	}
	return nil
}
