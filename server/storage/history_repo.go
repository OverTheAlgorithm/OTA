package storage

import (
	"context"
	"time"

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
			ci.category,
			ci.rank,
			ci.topic,
			ci.summary
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
		if err := rows.Scan(&deliveredAt, &item.Category, &item.Rank, &item.Topic, &item.Summary); err != nil {
			return nil, err
		}
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
