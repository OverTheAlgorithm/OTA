package editor

import (
	"context"
	"errors"
	"time"
)

// Status constants for editor_posts.status. The DB has a CHECK constraint that
// must stay in sync with these values.
const (
	StatusDraft     = "draft"
	StatusPublished = "published"
)

// Limits enforced before persisting.
const (
	MaxTitleLen   = 200
	MaxContentLen = 100 * 1024 // 100 KB
	ExcerptLen    = 280        // characters in card preview
)

var (
	ErrTitleRequired = errors.New("제목은 필수입니다")
	ErrTitleTooLong  = errors.New("제목이 너무 깁니다")
	ErrContentEmpty  = errors.New("본문이 비어 있습니다")
	ErrContentTooLong = errors.New("본문이 너무 깁니다")
	ErrInvalidStatus = errors.New("status는 draft 또는 published여야 합니다")
	ErrNotAuthorized = errors.New("이 글에 대한 권한이 없습니다")
	ErrPostNotFound  = errors.New("글을 찾을 수 없습니다")
)

// Post represents a single editor-authored article.
type Post struct {
	ID            string     `json:"id"`
	AuthorID      string     `json:"author_id"`
	Title         string     `json:"title"`
	ContentHTML   string     `json:"content_html"`
	ContentText   string     `json:"content_text"`
	FirstImageURL *string    `json:"first_image_url,omitempty"`
	Status        string     `json:"status"`
	PublishedAt   *time.Time `json:"published_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// IsPublished is a convenience for handlers that need to filter drafts.
func (p Post) IsPublished() bool { return p.Status == StatusPublished }

// PublicCard is the shape returned by the listing endpoint: lightweight,
// excerpt only.
type PublicCard struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	Excerpt       string    `json:"excerpt"`
	FirstImageURL *string   `json:"first_image_url,omitempty"`
	AuthorID      string    `json:"author_id"`
	AuthorName    string    `json:"author_name,omitempty"`
	PublishedAt   time.Time `json:"published_at"`
}

// PublicPost is the full payload for the detail page (includes sanitised HTML).
type PublicPost struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	ContentHTML   string    `json:"content_html"`
	FirstImageURL *string   `json:"first_image_url,omitempty"`
	AuthorID      string    `json:"author_id"`
	AuthorName    string    `json:"author_name,omitempty"`
	PublishedAt   time.Time `json:"published_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// Repository defines persistence for editor posts.
type Repository interface {
	Create(ctx context.Context, p Post) (Post, error)
	Update(ctx context.Context, p Post) (Post, error)
	Delete(ctx context.Context, id string) error
	FindByID(ctx context.Context, id string) (Post, error)
	// FindDraftByAuthor returns the single draft for an author, or
	// ErrPostNotFound if none exists. A partial unique index in the DB
	// guarantees at most one row matches.
	FindDraftByAuthor(ctx context.Context, authorID string) (Post, error)
	ListByAuthor(ctx context.Context, authorID string) ([]Post, error)
	ListAllForAdmin(ctx context.Context) ([]Post, error)
	ListPublishedCards(ctx context.Context, limit, offset int) ([]PublicCard, error)
	GetPublishedByID(ctx context.Context, id string) (PublicPost, error)
	CountPublished(ctx context.Context) (int, error)
}
