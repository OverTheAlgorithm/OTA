package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"ota/domain/quiz"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// QuizRepository is the PostgreSQL implementation of quiz.Repository.
type QuizRepository struct {
	pool *pgxpool.Pool
}

// NewQuizRepository creates a new QuizRepository.
func NewQuizRepository(pool *pgxpool.Pool) *QuizRepository {
	return &QuizRepository{pool: pool}
}

// SaveQuiz inserts a single quiz record.
func (r *QuizRepository) SaveQuiz(ctx context.Context, q quiz.Quiz) error {
	optionsJSON, err := json.Marshal(q.Options)
	if err != nil {
		return fmt.Errorf("save quiz: marshal options: %w", err)
	}
	_, err = r.pool.Exec(ctx,
		`INSERT INTO quizzes (id, context_item_id, question, options, correct_index)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (context_item_id) DO NOTHING`,
		q.ID, q.ContextItemID, q.Question, optionsJSON, q.CorrectIndex,
	)
	if err != nil {
		return fmt.Errorf("save quiz: %w", err)
	}
	return nil
}

// SaveQuizBatch inserts multiple quiz records in a single operation.
// Individual failures (e.g. duplicate context_item_id) are silently skipped.
func (r *QuizRepository) SaveQuizBatch(ctx context.Context, quizzes []quiz.Quiz) error {
	if len(quizzes) == 0 {
		return nil
	}
	for _, q := range quizzes {
		if err := r.SaveQuiz(ctx, q); err != nil {
			return fmt.Errorf("save quiz batch: item %s: %w", q.ContextItemID, err)
		}
	}
	return nil
}

// GetByContextItemID returns the quiz for a given article, or nil if none exists.
func (r *QuizRepository) GetByContextItemID(ctx context.Context, contextItemID uuid.UUID) (*quiz.Quiz, error) {
	var q quiz.Quiz
	var optionsJSON []byte
	err := r.pool.QueryRow(ctx,
		`SELECT id, context_item_id, question, options, correct_index, created_at
		 FROM quizzes WHERE context_item_id = $1`,
		contextItemID,
	).Scan(&q.ID, &q.ContextItemID, &q.Question, &optionsJSON, &q.CorrectIndex, &q.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get quiz by context item id: %w", err)
	}
	if err := json.Unmarshal(optionsJSON, &q.Options); err != nil {
		return nil, fmt.Errorf("get quiz by context item id: unmarshal options: %w", err)
	}
	return &q, nil
}

// HasAttempted reports whether the user has already submitted an answer for this quiz.
func (r *QuizRepository) HasAttempted(ctx context.Context, userID string, quizID uuid.UUID) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM quiz_results WHERE user_id = $1 AND quiz_id = $2)`,
		userID, quizID,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("has attempted: %w", err)
	}
	return exists, nil
}

// GetQuizExistenceMap returns a map of context_item_id -> true for all items that have a quiz.
func (r *QuizRepository) GetQuizExistenceMap(ctx context.Context, itemIDs []uuid.UUID) (map[uuid.UUID]bool, error) {
	if len(itemIDs) == 0 {
		return map[uuid.UUID]bool{}, nil
	}
	rows, err := r.pool.Query(ctx,
		`SELECT context_item_id FROM quizzes WHERE context_item_id = ANY($1)`,
		itemIDs,
	)
	if err != nil {
		return nil, fmt.Errorf("get quiz existence map: %w", err)
	}
	defer rows.Close()

	result := make(map[uuid.UUID]bool, len(itemIDs))
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("get quiz existence map: scan: %w", err)
		}
		result[id] = true
	}
	return result, rows.Err()
}

// GetQuizCompletionMap returns a map of context_item_id -> true for items where the user has completed the quiz.
func (r *QuizRepository) GetQuizCompletionMap(ctx context.Context, userID string, itemIDs []uuid.UUID) (map[uuid.UUID]bool, error) {
	if len(itemIDs) == 0 {
		return map[uuid.UUID]bool{}, nil
	}
	rows, err := r.pool.Query(ctx,
		`SELECT context_item_id FROM quiz_results WHERE user_id = $1 AND context_item_id = ANY($2)`,
		userID, itemIDs,
	)
	if err != nil {
		return nil, fmt.Errorf("get quiz completion map: %w", err)
	}
	defer rows.Close()

	result := make(map[uuid.UUID]bool, len(itemIDs))
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("get quiz completion map: scan: %w", err)
		}
		result[id] = true
	}
	return result, rows.Err()
}

// SaveResultAndAwardCoins records the quiz attempt and, if correct, awards coins in a single transaction.
// Transaction steps:
//  1. INSERT quiz_results (UNIQUE(user_id, quiz_id) prevents duplicate attempts)
//  2. If correct: INSERT coin_events (type='quiz_bonus', memo=topicName)
//  3. If correct: UPSERT user_points capped at coinCap
//
// Returns the user's new total coin balance.
func (r *QuizRepository) SaveResultAndAwardCoins(
	ctx context.Context,
	userID string,
	quizID, contextItemID uuid.UUID,
	answeredIndex int,
	isCorrect bool,
	coins, coinCap int,
	topicName string,
) (int, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("save result and award coins: begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// 1. Insert quiz result — UNIQUE(user_id, quiz_id) prevents duplicate attempts.
	_, err = tx.Exec(ctx,
		`INSERT INTO quiz_results (user_id, quiz_id, context_item_id, answered_index, is_correct, coins_earned)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		userID, quizID, contextItemID, answeredIndex, isCorrect, coins,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return 0, fmt.Errorf("save result and award coins: already attempted")
		}
		return 0, fmt.Errorf("save result and award coins: insert quiz result: %w", err)
	}

	// Fetch current balance to return as newTotal even when wrong answer.
	var currentTotal int
	err = tx.QueryRow(ctx,
		`SELECT COALESCE((SELECT points FROM user_points WHERE user_id = $1), 0)`,
		userID,
	).Scan(&currentTotal)
	if err != nil {
		return 0, fmt.Errorf("save result and award coins: read current coins: %w", err)
	}

	newTotal := currentTotal
	if isCorrect && coins > 0 {
		// 2. Record coin event (type='quiz_bonus') — NOT counted in GetTodayEarnedCoins.
		_, err = tx.Exec(ctx,
			`INSERT INTO coin_events (user_id, amount, type, memo) VALUES ($1, $2, $3, $4)`,
			userID, coins, "quiz_bonus", topicName,
		)
		if err != nil {
			return 0, fmt.Errorf("save result and award coins: insert coin event: %w", err)
		}

		// 3. Upsert user_points capped at coinCap.
		effectiveCap := coinCap
		if effectiveCap <= 0 {
			effectiveCap = int(^uint(0) >> 1) // max int — no cap
		}
		err = tx.QueryRow(ctx, `
			INSERT INTO user_points (user_id, points, updated_at)
			VALUES ($1, LEAST($2::int, $3::int), NOW())
			ON CONFLICT (user_id) DO UPDATE SET
				points     = LEAST(user_points.points + $2::int, $3::int),
				updated_at = NOW()
			RETURNING points
		`, userID, coins, effectiveCap).Scan(&newTotal)
		if err != nil {
			return 0, fmt.Errorf("save result and award coins: upsert user points: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("save result and award coins: commit: %w", err)
	}
	return newTotal, nil
}
