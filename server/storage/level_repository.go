package storage

import (
	"context"
	"errors"
	"fmt"

	"ota/domain/level"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type LevelRepository struct {
	pool *pgxpool.Pool
}

func NewLevelRepository(pool *pgxpool.Pool) *LevelRepository {
	return &LevelRepository{pool: pool}
}

func (r *LevelRepository) GetUserCoins(ctx context.Context, userID string) (level.UserCoins, error) {
	var uc level.UserCoins
	err := r.pool.QueryRow(ctx,
		`SELECT user_id, points, created_at, updated_at FROM user_points WHERE user_id = $1`,
		userID,
	).Scan(&uc.UserID, &uc.Coins, &uc.CreatedAt, &uc.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return level.UserCoins{UserID: userID, Coins: 0}, nil
	}
	if err != nil {
		return level.UserCoins{}, fmt.Errorf("get user coins: %w", err)
	}
	return uc, nil
}

func (r *LevelRepository) EarnCoin(ctx context.Context, userID string, runID, contextItemID uuid.UUID, coins int) (bool, int, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, 0, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// 1. Insert coin_log — UNIQUE(user_id, run_id, context_item_id) prevents duplicates within same run
	_, err = tx.Exec(ctx,
		`INSERT INTO coin_logs (user_id, run_id, context_item_id, coins_earned) VALUES ($1, $2, $3, $4)`,
		userID, runID, contextItemID, coins,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return false, 0, nil // already earned this topic in this run
		}
		return false, 0, fmt.Errorf("insert coin log: %w", err)
	}

	// 2. Upsert user_points and add earned coins
	var newTotal int
	err = tx.QueryRow(ctx, `
		INSERT INTO user_points (user_id, points, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (user_id) DO UPDATE SET
			points = user_points.points + $2,
			updated_at = NOW()
		RETURNING points
	`, userID, coins).Scan(&newTotal)
	if err != nil {
		return false, 0, fmt.Errorf("upsert user coins: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return false, 0, fmt.Errorf("commit: %w", err)
	}
	return true, newTotal, nil
}

func (r *LevelRepository) SetCoins(ctx context.Context, userID string, coins int) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO user_points (user_id, points, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (user_id) DO UPDATE SET
			points     = $2,
			updated_at = NOW()
	`, userID, coins)
	if err != nil {
		return fmt.Errorf("set coins: %w", err)
	}
	return nil
}

// HasEarned reports whether the given (user, run, item) triple already exists in coin_logs.
func (r *LevelRepository) HasEarned(ctx context.Context, userID string, runID, contextItemID uuid.UUID) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM coin_logs WHERE user_id = $1 AND run_id = $2 AND context_item_id = $3)`,
		userID, runID, contextItemID,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("has earned: %w", err)
	}
	return exists, nil
}

// GetTodayEarnedCoins returns the total coins a user has earned today (KST).
func (r *LevelRepository) GetTodayEarnedCoins(ctx context.Context, userID string) (int, error) {
	var total int
	err := r.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(coins_earned), 0)
		FROM coin_logs
		WHERE user_id = $1
		  AND created_at >= DATE_TRUNC('day', NOW() AT TIME ZONE 'Asia/Seoul') AT TIME ZONE 'Asia/Seoul'
	`, userID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("get today earned coins: %w", err)
	}
	return total, nil
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
			 '알고리즘 너머의 세상을 경험하고 있나요? 이 토픽을 읽으면 레벨 코인이 적립돼요.',
			 '이것은 Over the Algorithm 기능 테스트용 샘플 게시글입니다. 실제 서비스에서는 AI가 분류한 진짜 맥락이 여기에 담겨요.',
			 '[]', 50, '[]')
	`, itemID, runID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("create mock item: %w", err)
	}

	return itemID, nil
}
