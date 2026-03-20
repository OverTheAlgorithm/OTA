package collector

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type HistoryItem struct {
	ID            string       `json:"id"`
	Category      string       `json:"category"`
	Priority      string       `json:"priority"`
	BrainCategory string       `json:"brain_category"`
	Rank          int          `json:"rank"`
	Topic         string       `json:"topic"`
	Summary       string       `json:"summary"`
	Detail        string       `json:"detail"`
	Details       []DetailItem `json:"details"`
	BuzzScore     int          `json:"buzz_score"`
}

type HistoryEntry struct {
	Date        string        `json:"date"`
	DeliveredAt time.Time     `json:"delivered_at"`
	Items       []HistoryItem `json:"items"`
}

// TopicPreview is a lightweight projection for the landing page and all-news page.
type TopicPreview struct {
	ID            uuid.UUID `json:"id"`
	Topic         string    `json:"topic"`
	Summary       string    `json:"summary"`
	ImageURL      *string   `json:"image_url"`
	RunID         uuid.UUID `json:"run_id,omitempty"`
	Category      string    `json:"category,omitempty"`
	BrainCategory string    `json:"brain_category,omitempty"`
	Priority      string    `json:"priority,omitempty"`
	CreatedAt     time.Time `json:"created_at,omitempty"`
}

// TopicDetail holds the full detail for a single context item, served on the public detail page.
type TopicDetail struct {
	ID            uuid.UUID    `json:"id"`
	RunID         uuid.UUID    `json:"run_id"`
	Category      string       `json:"category"`
	Priority      string       `json:"priority"`
	Topic         string       `json:"topic"`
	Detail        string       `json:"detail"`
	Details       []DetailItem `json:"details"`
	BuzzScore     int          `json:"buzz_score"`
	Sources       []string     `json:"sources"`
	BrainCategory string       `json:"brain_category"`
	CreatedAt     time.Time    `json:"created_at"`
	ImageURL      *string      `json:"image_url"`
}

// ItemMeta holds lightweight metadata for batch earn status lookups.
type ItemMeta struct {
	RunID    uuid.UUID
	Category string
	Priority string
}

type HistoryRepository interface {
	// GetHistoryForUser returns paginated history entries (date-based).
	// Returns entries, hasMore, error.
	GetHistoryForUser(ctx context.Context, userID string, limit, offset int) ([]HistoryEntry, bool, error)
	// GetContextItemByID returns the detail for a single topic. Returns nil, nil if not found.
	GetContextItemByID(ctx context.Context, id uuid.UUID) (*TopicDetail, error)
	// IsRunCreatedToday checks if the collection run was started today (in KST).
	IsRunCreatedToday(ctx context.Context, runID uuid.UUID) (bool, error)
	// GetRecentTopics returns up to `count` random topics from the latest collection run.
	GetRecentTopics(ctx context.Context, count int) ([]TopicPreview, error)
	// GetLatestRunTopics returns all topics from the latest successful collection run.
	GetLatestRunTopics(ctx context.Context) ([]TopicPreview, error)
	// GetAllTopics returns paginated topics with optional filter.
	// filterType: "category" | "brain_category" | "" (all)
	// Returns topics, hasMore, error.
	GetAllTopics(ctx context.Context, filterType, filterValue string, limit, offset int) ([]TopicPreview, bool, error)
	// GetItemCategoryMap returns lightweight metadata for a batch of item IDs.
	GetItemCategoryMap(ctx context.Context, itemIDs []uuid.UUID) (map[uuid.UUID]ItemMeta, error)
}
