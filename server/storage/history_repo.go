package storage

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"ota/domain/collector"
)

type HistoryRepository struct {
	pool *pgxpool.Pool
}

func NewHistoryRepository(pool *pgxpool.Pool) *HistoryRepository {
	return &HistoryRepository{pool: pool}
}

func (r *HistoryRepository) GetHistoryForUser(ctx context.Context, userID string) ([]collector.HistoryEntry, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT
			dl.created_at,
			ci.id::text,
			ci.category,
			COALESCE(ci.brain_category, ''),
			ci.rank,
			ci.topic,
			ci.summary,
			COALESCE(ci.detail, ''),
			COALESCE(ci.details, '[]'),
			COALESCE(ci.buzz_score, 0)
		FROM delivery_logs dl
		JOIN collection_runs cr ON dl.run_id = cr.id
		JOIN context_items ci   ON ci.collection_run_id = cr.id
		WHERE dl.user_id = $1
		  AND dl.status  = 'sent'
		ORDER BY dl.created_at DESC, ci.rank ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entryMap := make(map[string]*collector.HistoryEntry)
	var order []string

	for rows.Next() {
		var deliveredAt time.Time
		var item collector.HistoryItem
		var detailsJSON []byte
		if err := rows.Scan(&deliveredAt, &item.ID, &item.Category, &item.BrainCategory, &item.Rank, &item.Topic, &item.Summary, &item.Detail, &detailsJSON, &item.BuzzScore); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(detailsJSON, &item.Details)
		date := deliveredAt.UTC().Format("2006-01-02")
		if _, ok := entryMap[date]; !ok {
			entryMap[date] = &collector.HistoryEntry{
				Date:        date,
				DeliveredAt: deliveredAt,
				Items:       []collector.HistoryItem{},
			}
			order = append(order, date)
		}
		entryMap[date].Items = append(entryMap[date].Items, item)
	}

	result := make([]collector.HistoryEntry, 0, len(order))
	for _, date := range order {
		result = append(result, *entryMap[date])
	}
	return result, nil
}

// GetContextItemByID returns the detail for a single topic by its UUID.
// Returns nil, nil if the item does not exist.
func (r *HistoryRepository) GetContextItemByID(ctx context.Context, id uuid.UUID) (*collector.TopicDetail, error) {
	var item collector.TopicDetail
	var detailsJSON []byte
	err := r.pool.QueryRow(ctx, `
		SELECT id, topic, COALESCE(detail, ''), COALESCE(details, '[]'), COALESCE(buzz_score, 0), COALESCE(sources, '[]'), created_at
		FROM context_items
		WHERE id = $1
	`, id).Scan(&item.ID, &item.Topic, &item.Detail, &detailsJSON, &item.BuzzScore, &item.Sources, &item.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	_ = json.Unmarshal(detailsJSON, &item.Details)
	return &item, nil
}
