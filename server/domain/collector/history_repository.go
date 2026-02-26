package collector

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type HistoryItem struct {
	ID            string       `json:"id"`
	Category      string       `json:"category"`
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

// TopicDetail holds the full detail for a single context item, served on the public detail page.
type TopicDetail struct {
	ID            uuid.UUID    `json:"id"`
	Category      string       `json:"category"`
	Topic         string       `json:"topic"`
	Detail        string       `json:"detail"`
	Details       []DetailItem `json:"details"`
	BuzzScore     int          `json:"buzz_score"`
	Sources       []string     `json:"sources"`
	BrainCategory string       `json:"brain_category"`
	CreatedAt     time.Time    `json:"created_at"`
}

type HistoryRepository interface {
	GetHistoryForUser(ctx context.Context, userID string) ([]HistoryEntry, error)
	// GetContextItemByID returns the detail for a single topic. Returns nil, nil if not found.
	GetContextItemByID(ctx context.Context, id uuid.UUID) (*TopicDetail, error)
	// IsRunCreatedToday checks if the collection run was started today (in KST).
	IsRunCreatedToday(ctx context.Context, runID uuid.UUID) (bool, error)
}
