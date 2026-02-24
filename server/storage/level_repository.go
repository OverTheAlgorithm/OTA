package storage

import (
	"context"
	"errors"
	"fmt"

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

func (r *LevelRepository) EarnPoint(ctx context.Context, userID string, contextItemID uuid.UUID) (bool, int, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, 0, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// 1. Insert point_log — UNIQUE(user_id, context_item_id) prevents duplicates
	_, err = tx.Exec(ctx,
		`INSERT INTO point_logs (user_id, context_item_id, points_earned) VALUES ($1, $2, 1)`,
		userID, contextItemID,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return false, 0, nil // already earned
		}
		return false, 0, fmt.Errorf("insert point log: %w", err)
	}

	// 2. Upsert user_points and increment total
	var newTotal int
	err = tx.QueryRow(ctx, `
		INSERT INTO user_points (user_id, level, points, updated_at)
		VALUES ($1, 1, 1, NOW())
		ON CONFLICT (user_id) DO UPDATE SET
			points = user_points.points + 1,
			updated_at = NOW()
		RETURNING points
	`, userID).Scan(&newTotal)
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

// CreateMockOTAItem inserts a fake collection_run and a context_item with
// brain_category = "over_the_algorithm" for testing level progression.
// Returns the context_item UUID that can be visited at /topic/:id.
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

func (r *LevelRepository) GetBrainCategory(ctx context.Context, contextItemID uuid.UUID) (string, error) {
	var bc string
	err := r.pool.QueryRow(ctx,
		`SELECT COALESCE(brain_category, '') FROM context_items WHERE id = $1`,
		contextItemID,
	).Scan(&bc)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("context item not found: %s", contextItemID)
	}
	if err != nil {
		return "", fmt.Errorf("get brain category: %w", err)
	}
	return bc, nil
}
