package editor

import (
	"context"
	"fmt"
	"strings"
	"time"

	"ota/domain/user"
)

// Service holds the business logic for editor posts. It is responsible for
// sanitisation, validation, and ownership checks. Persistence is delegated to
// Repository, which keeps the service trivially testable.
type Service struct {
	repo Repository
	now  func() time.Time
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo, now: time.Now}
}

// withNow lets tests inject a deterministic clock.
func (s *Service) withNow(fn func() time.Time) *Service {
	s.now = fn
	return s
}

// CreateParams is the user-controlled subset of fields used on create.
type CreateParams struct {
	AuthorID    string
	Title       string
	ContentHTML string
	Status      string // draft or published
}

// UpdateParams is what the editor can change on an existing post. Title and
// ContentHTML are required even on draft saves so the row always has a sane state.
type UpdateParams struct {
	Title       string
	ContentHTML string
	Status      string
}

// Create persists a new post for AuthorID.
func (s *Service) Create(ctx context.Context, p CreateParams) (Post, error) {
	if err := validateStatus(p.Status); err != nil {
		return Post{}, err
	}
	title, err := normaliseTitle(p.Title)
	if err != nil {
		return Post{}, err
	}
	cleanHTML, err := normaliseHTML(p.ContentHTML)
	if err != nil {
		return Post{}, err
	}

	post := Post{
		AuthorID:    p.AuthorID,
		Title:       title,
		ContentHTML: cleanHTML,
		ContentText: Excerpt(cleanHTML, MaxContentLen),
		Status:      p.Status,
	}
	if url := FirstImageURL(cleanHTML); url != "" {
		post.FirstImageURL = &url
	}
	if p.Status == StatusPublished {
		now := s.now()
		post.PublishedAt = &now
	}

	return s.repo.Create(ctx, post)
}

// Update mutates an existing post. callerID and callerRole are used for the
// authorisation check; the post's stored author is left intact even when an
// admin edits it.
func (s *Service) Update(ctx context.Context, id, callerID, callerRole string, p UpdateParams) (Post, error) {
	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return Post{}, err
	}
	if !canModify(callerID, callerRole, existing.AuthorID) {
		return Post{}, ErrNotAuthorized
	}

	if err := validateStatus(p.Status); err != nil {
		return Post{}, err
	}
	title, err := normaliseTitle(p.Title)
	if err != nil {
		return Post{}, err
	}
	cleanHTML, err := normaliseHTML(p.ContentHTML)
	if err != nil {
		return Post{}, err
	}

	existing.Title = title
	existing.ContentHTML = cleanHTML
	existing.ContentText = Excerpt(cleanHTML, MaxContentLen)
	if url := FirstImageURL(cleanHTML); url != "" {
		existing.FirstImageURL = &url
	} else {
		existing.FirstImageURL = nil
	}
	existing.Status = p.Status

	switch {
	case p.Status == StatusPublished && existing.PublishedAt == nil:
		now := s.now()
		existing.PublishedAt = &now
	case p.Status == StatusDraft:
		existing.PublishedAt = nil
	}

	return s.repo.Update(ctx, existing)
}

// Delete removes a post after an ownership check.
func (s *Service) Delete(ctx context.Context, id, callerID, callerRole string) error {
	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if !canModify(callerID, callerRole, existing.AuthorID) {
		return ErrNotAuthorized
	}
	return s.repo.Delete(ctx, id)
}

// GetForEdit returns a post for the editor UI, enforcing ownership.
func (s *Service) GetForEdit(ctx context.Context, id, callerID, callerRole string) (Post, error) {
	p, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return Post{}, err
	}
	if !canModify(callerID, callerRole, p.AuthorID) {
		return Post{}, ErrNotAuthorized
	}
	return p, nil
}

// ListForCaller returns the caller's own posts. Admins see every post.
func (s *Service) ListForCaller(ctx context.Context, callerID, callerRole string) ([]Post, error) {
	if user.HasRoleAtLeast(callerRole, user.RoleAdmin) {
		return s.repo.ListAllForAdmin(ctx)
	}
	return s.repo.ListByAuthor(ctx, callerID)
}

// ─── Internal helpers ───────────────────────────────────────────────────────

func validateStatus(s string) error {
	if s != StatusDraft && s != StatusPublished {
		return ErrInvalidStatus
	}
	return nil
}

func normaliseTitle(title string) (string, error) {
	t := strings.TrimSpace(title)
	if t == "" {
		return "", ErrTitleRequired
	}
	if len([]rune(t)) > MaxTitleLen {
		return "", fmt.Errorf("%w (최대 %d자)", ErrTitleTooLong, MaxTitleLen)
	}
	return t, nil
}

func normaliseHTML(html string) (string, error) {
	if len(html) > MaxContentLen {
		return "", fmt.Errorf("%w (최대 %dKB)", ErrContentTooLong, MaxContentLen/1024)
	}
	cleaned := Sanitize(html)
	if strings.TrimSpace(Excerpt(cleaned, 1)) == "" {
		return "", ErrContentEmpty
	}
	return cleaned, nil
}

// canModify returns true when callerID owns the post or has admin privileges.
func canModify(callerID, callerRole, authorID string) bool {
	if callerID == authorID {
		return true
	}
	return user.HasRoleAtLeast(callerRole, user.RoleAdmin)
}
