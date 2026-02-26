package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"ota/domain/level"
)

type LevelRepository struct {
	pool *pgxpool.Pool
}

func NewLevelRepository(pool *pgxpool.Pool) *LevelRepository {
	return &LevelRepository{pool: pool}
}

func (r *LevelRepository) GetUserPoints(ctx context.Context, userID string) (level.UserPoints, error) {
	var up level.UserPoints
	err := r.pool.QueryRow(ctx,
		`SELECT user_id, level, points, created_at, updated_at FROM user_points WHERE user_id = $1`,
		userID,
	).Scan(&up.UserID, &up.Level, &up.Points, &up.CreatedAt, &up.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return level.UserPoints{UserID: userID, Level: 1, Points: 0}, nil
	}
	if err != nil {
		return level.UserPoints{}, fmt.Errorf("get user points: %w", err)
	}
	return up, nil
}

func (r *LevelRepository) EarnPoint(ctx context.Context, userID string, runID, contextItemID uuid.UUID, points int) (bool, int, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, 0, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// 1. Insert point_log — UNIQUE(user_id, run_id, context_item_id) prevents duplicates within same run
	_, err = tx.Exec(ctx,
		`INSERT INTO point_logs (user_id, run_id, context_item_id, points_earned) VALUES ($1, $2, $3, $4)`,
		userID, runID, contextItemID, points,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return false, 0, nil // already earned this topic in this run
		}
		return false, 0, fmt.Errorf("insert point log: %w", err)
	}

	// 2. Upsert user_points and add earned points
	var newTotal int
	err = tx.QueryRow(ctx, `
		INSERT INTO user_points (user_id, level, points, updated_at)
		VALUES ($1, 1, $2, NOW())
		ON CONFLICT (user_id) DO UPDATE SET
			points = user_points.points + $2,
			updated_at = NOW()
		RETURNING points
	`, userID, points).Scan(&newTotal)
	if err != nil {
		return false, 0, fmt.Errorf("upsert user points: %w", err)
	}

	// 3. Recalculate level
	newLevel := level.CalcLevel(newTotal)
	_, err = tx.Exec(ctx,
		`UPDATE user_points SET level = $1 WHERE user_id = $2`,
		newLevel, userID,
	)
	if err != nil {
		return false, 0, fmt.Errorf("update level: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return false, 0, fmt.Errorf("commit: %w", err)
	}
	return true, newTotal, nil
}

func (r *LevelRepository) GetLastEarnedAt(ctx context.Context, userID string) (time.Time, bool, error) {
	var t time.Time
	err := r.pool.QueryRow(ctx,
		`SELECT created_at FROM point_logs WHERE user_id = $1 ORDER BY created_at DESC LIMIT 1`,
		userID,
	).Scan(&t)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, fmt.Errorf("get last earned at: %w", err)
	}
	return t, true, nil
}

func (r *LevelRepository) GetLastEarnedAtBatch(ctx context.Context, userIDs []string) (map[string]time.Time, error) {
	if len(userIDs) == 0 {
		return make(map[string]time.Time), nil
	}
	rows, err := r.pool.Query(ctx,
		`SELECT user_id, MAX(created_at) FROM point_logs WHERE user_id = ANY($1) GROUP BY user_id`,
		userIDs,
	)
	if err != nil {
		return nil, fmt.Errorf("get last earned at batch: %w", err)
	}
	defer rows.Close()

	result := make(map[string]time.Time, len(userIDs))
	for rows.Next() {
		var uid string
		var t time.Time
		if err := rows.Scan(&uid, &t); err != nil {
			return nil, fmt.Errorf("scan last earned at: %w", err)
		}
		result[uid] = t
	}
	return result, rows.Err()
}

// DecayPoints subtracts 1 point from all users (minimum 0) using keyset pagination.
// Level is recalculated after each batch.
// Thresholds must match level.Thresholds: [0, 15, 45, 90, 165].
func (r *LevelRepository) DecayPoints(ctx context.Context, batchSize int) (int, error) {
	total := 0
	cursor := "" // empty = start from beginning

	for {
		ids, err := r.fetchDecayBatch(ctx, cursor, batchSize)
		if err != nil {
			return total, err
		}
		if len(ids) == 0 {
			break
		}

		if _, err = r.pool.Exec(ctx,
			`UPDATE user_points SET points = GREATEST(0, points - 1), updated_at = NOW() WHERE user_id = ANY($1)`,
			ids,
		); err != nil {
			return total, fmt.Errorf("decay batch: %w", err)
		}

		// Recalculate levels after decay (thresholds: Lv1:0, Lv2:15, Lv3:45, Lv4:90, Lv5:165)
		if _, err = r.pool.Exec(ctx, `
			UPDATE user_points SET level = CASE
				WHEN points >= 165 THEN 5
				WHEN points >= 90  THEN 4
				WHEN points >= 45  THEN 3
				WHEN points >= 15  THEN 2
				ELSE 1
			END
			WHERE user_id = ANY($1)
		`, ids); err != nil {
			return total, fmt.Errorf("recalculate levels after decay: %w", err)
		}

		total += len(ids)
		cursor = ids[len(ids)-1]
		if len(ids) < batchSize {
			break
		}
	}

	return total, nil
}

func (r *LevelRepository) fetchDecayBatch(ctx context.Context, cursor string, batchSize int) ([]string, error) {
	var rows pgx.Rows
	var err error
	if cursor == "" {
		rows, err = r.pool.Query(ctx,
			`SELECT user_id FROM user_points WHERE points > 0 ORDER BY user_id LIMIT $1`,
			batchSize,
		)
	} else {
		rows, err = r.pool.Query(ctx,
			`SELECT user_id FROM user_points WHERE points > 0 AND user_id > $1 ORDER BY user_id LIMIT $2`,
			cursor, batchSize,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("fetch decay batch: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if scanErr := rows.Scan(&id); scanErr != nil {
			return nil, fmt.Errorf("scan user id: %w", scanErr)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *LevelRepository) SetPoints(ctx context.Context, userID string, points int) error {
	newLevel := level.CalcLevel(points)
	_, err := r.pool.Exec(ctx, `
		INSERT INTO user_points (user_id, level, points, updated_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (user_id) DO UPDATE SET
			points     = $3,
			level      = $2,
			updated_at = NOW()
	`, userID, newLevel, points)
	if err != nil {
		return fmt.Errorf("set points: %w", err)
	}
	return nil
}

// CreateMockOTAItem inserts a fake collection_run and a context_item for
// testing level progression. Returns the context_item UUID.
func (r *LevelRepository) CreateMockOTAItem(ctx context.Context) (uuid.UUID, error) {
	runID := uuid.New()
	itemID := uuid.New()

	_, err := r.pool.Exec(ctx, `
		INSERT INTO collection_runs (id, started_at, completed_at, status)
		VALUES ($1, NOW(), NOW(), 'success')
	`, runID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("create mock run: %w", err)
	}

	_, err = r.pool.Exec(ctx, `
		INSERT INTO context_items
			(id, collection_run_id, category, brain_category, rank, topic, summary, detail, details, buzz_score, sources)
		VALUES
			($1, $2, 'top', 'over_the_algorithm', 1,
			 '[테스트] Over the Algorithm 샘플 토픽',
			 '알고리즘 너머의 세상을 경험하고 있나요? 이 토픽을 읽으면 레벨 포인트가 적립돼요.',
			 '이것은 Over the Algorithm 기능 테스트용 샘플 게시글입니다. 실제 서비스에서는 AI가 분류한 진짜 맥락이 여기에 담겨요.',
			 '[]', 50, '[]')
	`, itemID, runID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("create mock item: %w", err)
	}

	return itemID, nil
}
