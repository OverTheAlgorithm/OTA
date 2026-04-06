package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"ota/domain/collector"
)

var kstLocation = time.FixedZone("KST", 9*60*60)

type HistoryRepository struct {
	pool *pgxpool.Pool
}

func NewHistoryRepository(pool *pgxpool.Pool) *HistoryRepository {
	return &HistoryRepository{pool: pool}
}

func (r *HistoryRepository) GetHistoryForUser(ctx context.Context, userID string, limit, offset int) ([]collector.HistoryEntry, bool, error) {
	// Fetch limit+1 distinct dates to determine hasMore.
	rows, err := r.pool.Query(ctx, `
		WITH target_dates AS (
			SELECT DISTINCT DATE(dl.created_at AT TIME ZONE 'Asia/Seoul') AS d
			FROM delivery_logs dl
			WHERE dl.user_id = $1 AND dl.status = 'sent'
			ORDER BY d DESC
			LIMIT $2 OFFSET $3
		)
		SELECT
			dl.created_at,
			ci.id::text,
			ci.category,
			COALESCE(ci.priority, 'none'),
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
		  AND DATE(dl.created_at AT TIME ZONE 'Asia/Seoul') IN (SELECT d FROM target_dates)
		ORDER BY dl.created_at DESC, ci.rank ASC
	`, userID, limit+1, offset)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	entryMap := make(map[string]*collector.HistoryEntry)
	var order []string

	for rows.Next() {
		var deliveredAt time.Time
		var item collector.HistoryItem
		var detailsJSON []byte
		if err := rows.Scan(&deliveredAt, &item.ID, &item.Category, &item.Priority, &item.BrainCategory, &item.Rank, &item.Topic, &item.Summary, &item.Detail, &detailsJSON, &item.BuzzScore); err != nil {
			return nil, false, err
		}
		item.Details = collector.UnmarshalDetails(detailsJSON)
		date := deliveredAt.In(kstLocation).Format("2006-01-02")
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
	if err := rows.Err(); err != nil {
		return nil, false, err
	}

	hasMore := len(order) > limit
	if hasMore {
		order = order[:limit]
	}

	result := make([]collector.HistoryEntry, 0, len(order))
	for _, date := range order {
		result = append(result, *entryMap[date])
	}
	return result, hasMore, nil
}

// GetContextItemByID returns the detail for a single topic by its UUID.
// Returns nil, nil if the item does not exist.
func (r *HistoryRepository) GetContextItemByID(ctx context.Context, id uuid.UUID) (*collector.TopicDetail, error) {
	var item collector.TopicDetail
	var detailsJSON []byte
	var imagePath *string
	var quizID *uuid.UUID
	err := r.pool.QueryRow(ctx, `
		SELECT ci.id, ci.collection_run_id, ci.category, COALESCE(ci.priority, 'none'), ci.topic,
		       COALESCE(ci.detail, ''), COALESCE(ci.details, '[]'), COALESCE(ci.buzz_score, 0),
		       COALESCE(ci.sources, '[]'), COALESCE(ci.brain_category, ''), ci.created_at, ci.image_path,
		       q.id AS quiz_id
		FROM context_items ci
		LEFT JOIN quizzes q ON q.context_item_id = ci.id
		WHERE ci.id = $1
	`, id).Scan(&item.ID, &item.RunID, &item.Category, &item.Priority, &item.Topic, &item.Detail, &detailsJSON, &item.BuzzScore, &item.Sources, &item.BrainCategory, &item.CreatedAt, &imagePath, &quizID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	item.Details = collector.UnmarshalDetails(detailsJSON)
	if imagePath != nil {
		url := "/api/v1/images/" + *imagePath
		item.ImageURL = &url
	}
	item.HasQuiz = quizID != nil
	return &item, nil
}

// GetRecentTopics returns up to `count` random topics from the latest successful collection run.
func (r *HistoryRepository) GetRecentTopics(ctx context.Context, count int) ([]collector.TopicPreview, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT ci.id, ci.topic, ci.summary, ci.image_path
		FROM context_items ci
		WHERE ci.collection_run_id = (
			SELECT id FROM collection_runs
			WHERE status = 'success'
			ORDER BY started_at DESC
			LIMIT 1
		)
		ORDER BY RANDOM()
		LIMIT $1
	`, count)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []collector.TopicPreview
	for rows.Next() {
		var item collector.TopicPreview
		var imagePath *string
		if err := rows.Scan(&item.ID, &item.Topic, &item.Summary, &imagePath); err != nil {
			return nil, err
		}
		if imagePath != nil {
			url := "/api/v1/images/" + *imagePath
			item.ImageURL = &url
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// GetLatestRunTopics returns all topics from the latest successful collection run.
func (r *HistoryRepository) GetLatestRunTopics(ctx context.Context) ([]collector.TopicPreview, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT ci.id, ci.topic, ci.summary, ci.image_path,
			ci.collection_run_id, ci.category, COALESCE(ci.brain_category, ''), COALESCE(ci.priority, 'none'), ci.created_at,
			(q.id IS NOT NULL) AS has_quiz
		FROM context_items ci
		LEFT JOIN quizzes q ON q.context_item_id = ci.id
		WHERE ci.collection_run_id = (
			SELECT id FROM collection_runs
			WHERE status = 'success'
			ORDER BY started_at DESC
			LIMIT 1
		)
		ORDER BY ci.priority DESC, ci.rank ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []collector.TopicPreview
	for rows.Next() {
		var item collector.TopicPreview
		var imagePath *string
		if err := rows.Scan(&item.ID, &item.Topic, &item.Summary, &imagePath,
			&item.RunID, &item.Category, &item.BrainCategory, &item.Priority, &item.CreatedAt, &item.HasQuiz); err != nil {
			return nil, err
		}
		if imagePath != nil {
			url := "/api/v1/images/" + *imagePath
			item.ImageURL = &url
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// IsRunCreatedToday returns true if the run was started today (KST).
func (r *HistoryRepository) IsRunCreatedToday(ctx context.Context, runID uuid.UUID) (bool, error) {
	query := `
		SELECT DATE(started_at AT TIME ZONE 'Asia/Seoul') = DATE(NOW() AT TIME ZONE 'Asia/Seoul')
		FROM collection_runs
		WHERE id = $1
	`
	var isToday bool
	err := r.pool.QueryRow(ctx, query, runID).Scan(&isToday)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return isToday, nil
}

// GetAllTopics returns paginated topics across all successful collection runs.
// Supports filtering by "category" or "brain_category".
func (r *HistoryRepository) GetAllTopics(ctx context.Context, filterType, filterValue string, limit, offset int) ([]collector.TopicPreview, bool, error) {
	// Build query dynamically based on filter type.
	baseQuery := `
		SELECT ci.id, ci.topic, ci.summary, ci.image_path,
			ci.collection_run_id, ci.category, COALESCE(ci.brain_category, ''), COALESCE(ci.priority, 'none'), ci.created_at,
			(q.id IS NOT NULL) AS has_quiz
		FROM context_items ci
		JOIN collection_runs cr ON cr.id = ci.collection_run_id AND cr.status = 'success'
		LEFT JOIN quizzes q ON q.context_item_id = ci.id
		WHERE 1=1`

	var args []interface{}
	argIdx := 1

	switch filterType {
	case "category":
		baseQuery += fmt.Sprintf(` AND ci.category = $%d`, argIdx)
		args = append(args, filterValue)
		argIdx++
	case "brain_category":
		baseQuery += fmt.Sprintf(` AND ci.brain_category = $%d`, argIdx)
		args = append(args, filterValue)
		argIdx++
	}

	baseQuery += fmt.Sprintf(` ORDER BY ci.created_at DESC, ci.rank ASC LIMIT $%d OFFSET $%d`, argIdx, argIdx+1)
	args = append(args, limit+1, offset)

	rows, err := r.pool.Query(ctx, baseQuery, args...)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	var items []collector.TopicPreview
	for rows.Next() {
		var item collector.TopicPreview
		var imagePath *string
		if err := rows.Scan(&item.ID, &item.Topic, &item.Summary, &imagePath,
			&item.RunID, &item.Category, &item.BrainCategory, &item.Priority, &item.CreatedAt, &item.HasQuiz); err != nil {
			return nil, false, err
		}
		if imagePath != nil {
			url := "/api/v1/images/" + *imagePath
			item.ImageURL = &url
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}

	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	return items, hasMore, nil
}

// GetItemCategoryMap returns lightweight metadata for a batch of item IDs.
func (r *HistoryRepository) GetItemCategoryMap(ctx context.Context, itemIDs []uuid.UUID) (map[uuid.UUID]collector.ItemMeta, error) {
	if len(itemIDs) == 0 {
		return map[uuid.UUID]collector.ItemMeta{}, nil
	}

	rows, err := r.pool.Query(ctx, `
		SELECT id, collection_run_id, category, COALESCE(priority, 'none')
		FROM context_items
		WHERE id = ANY($1)
	`, itemIDs)
	if err != nil {
		return nil, fmt.Errorf("get item category map: %w", err)
	}
	defer rows.Close()

	result := make(map[uuid.UUID]collector.ItemMeta, len(itemIDs))
	for rows.Next() {
		var id uuid.UUID
		var meta collector.ItemMeta
		if err := rows.Scan(&id, &meta.RunID, &meta.Category, &meta.Priority); err != nil {
			return nil, err
		}
		result[id] = meta
	}
	return result, rows.Err()
}
