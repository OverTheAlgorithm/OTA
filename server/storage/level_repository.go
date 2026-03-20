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

// SetCoins atomically overwrites a user's coin balance and records the delta
// as a coin_event for audit purposes. The actorID identifies who triggered
// the change (empty string = system). Both writes happen in one transaction.
func (r *LevelRepository) SetCoins(ctx context.Context, userID string, coins int, actorID string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("set coins begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// 1. Read current balance so we can compute the delta for the audit log.
	var current int
	err = tx.QueryRow(ctx,
		`SELECT COALESCE((SELECT points FROM user_points WHERE user_id = $1), 0)`,
		userID,
	).Scan(&current)
	if err != nil {
		return fmt.Errorf("set coins read current: %w", err)
	}

	// 2. Overwrite the balance.
	_, err = tx.Exec(ctx, `
		INSERT INTO user_points (user_id, points, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (user_id) DO UPDATE SET
			points     = $2,
			updated_at = NOW()
	`, userID, coins)
	if err != nil {
		return fmt.Errorf("set coins upsert: %w", err)
	}

	// 3. Record the delta as an audit event (skip if no change).
	delta := coins - current
	if delta != 0 {
		var actor interface{}
		if actorID != "" {
			actor = actorID
		}
		_, err = tx.Exec(ctx,
			`INSERT INTO coin_events (user_id, amount, type, memo, actor_id) VALUES ($1, $2, $3, $4, $5)`,
			userID, delta, "admin_set", fmt.Sprintf("set to %d (was %d)", coins, current), actor,
		)
		if err != nil {
			return fmt.Errorf("set coins audit event: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("set coins commit: %w", err)
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

// DeductCoins atomically subtracts coins from a user's balance.
// Returns an error if the user has insufficient funds.
func (r *LevelRepository) DeductCoins(ctx context.Context, userID string, amount int) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE user_points SET points = points - $2, updated_at = NOW()
		WHERE user_id = $1 AND points >= $2
	`, userID, amount)
	if err != nil {
		return fmt.Errorf("deduct coins: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("insufficient coins")
	}
	return nil
}

// RestoreCoins adds coins back to a user's balance.
func (r *LevelRepository) RestoreCoins(ctx context.Context, userID string, amount int) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE user_points SET points = points + $2, updated_at = NOW()
		WHERE user_id = $1
	`, userID, amount)
	if err != nil {
		return fmt.Errorf("restore coins: %w", err)
	}
	return nil
}

// AddPoints adds coins to a user's balance, creating the row if it doesn't exist.
func (r *LevelRepository) AddPoints(ctx context.Context, userID string, amount int) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO user_points (user_id, points, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (user_id) DO UPDATE SET
			points     = user_points.points + $2,
			updated_at = NOW()
	`, userID, amount)
	if err != nil {
		return fmt.Errorf("add points: %w", err)
	}
	return nil
}

// InsertCoinEvent logs a non-topic coin event.
// actorID is the admin/user who triggered it; empty string stores NULL.
func (r *LevelRepository) InsertCoinEvent(ctx context.Context, userID string, amount int, eventType, memo, actorID string) error {
	var actor interface{}
	if actorID != "" {
		actor = actorID
	}
	_, err := r.pool.Exec(ctx,
		`INSERT INTO coin_events (user_id, amount, type, memo, actor_id) VALUES ($1, $2, $3, $4, $5)`,
		userID, amount, eventType, memo, actor,
	)
	if err != nil {
		return fmt.Errorf("insert coin event: %w", err)
	}
	return nil
}

// GetCoinHistory returns a unified paginated timeline of all coin balance changes.
// The query delegates to the coin_history view (migration 000024).
func (r *LevelRepository) GetCoinHistory(ctx context.Context, userID string, limit, offset int) ([]level.CoinTransaction, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, amount, type, description, link_id, created_at
		FROM coin_history
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("get coin history: %w", err)
	}
	defer rows.Close()

	var txns []level.CoinTransaction
	for rows.Next() {
		var t level.CoinTransaction
		if err := rows.Scan(&t.ID, &t.Amount, &t.Type, &t.Description, &t.LinkID, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan coin history: %w", err)
		}
		txns = append(txns, t)
	}
	return txns, nil
}

// GetEarnedItemIDs returns the subset of itemIDs that the user has already earned coins for.
func (r *LevelRepository) GetEarnedItemIDs(ctx context.Context, userID string, itemIDs []uuid.UUID) ([]uuid.UUID, error) {
	if len(itemIDs) == 0 {
		return nil, nil
	}
	rows, err := r.pool.Query(ctx,
		`SELECT DISTINCT context_item_id FROM coin_logs WHERE user_id = $1 AND context_item_id = ANY($2)`,
		userID, itemIDs,
	)
	if err != nil {
		return nil, fmt.Errorf("get earned item ids: %w", err)
	}
	defer rows.Close()

	var earned []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		earned = append(earned, id)
	}
	return earned, rows.Err()
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
