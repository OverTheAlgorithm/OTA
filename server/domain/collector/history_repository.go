package collector

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type HistoryItem struct {
	Category string `json:"category"`
	Rank     int    `json:"rank"`
	Topic    string `json:"topic"`
	Summary  string `json:"summary"`
}

type HistoryEntry struct {
	Date        string        `json:"date"`
	DeliveredAt time.Time     `json:"delivered_at"`
	Items       []HistoryItem `json:"items"`
}

// TopicDetail holds the full detail for a single context item, served on the public detail page.
type TopicDetail struct {
	ID        uuid.UUID `json:"id"`
	Topic     string    `json:"topic"`
	Detail    string    `json:"detail"`
	CreatedAt time.Time `json:"created_at"`
}

type HistoryRepository interface {
	GetHistoryForUser(ctx context.Context, userID string) ([]HistoryEntry, error)
	// GetContextItemByID returns the detail for a single topic. Returns nil, nil if not found.
	GetContextItemByID(ctx context.Context, id uuid.UUID) (*TopicDetail, error)
}
