package collector

import (
	"context"
	"time"
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

type HistoryRepository interface {
	GetHistoryForUser(ctx context.Context, userID string) ([]HistoryEntry, error)
}
