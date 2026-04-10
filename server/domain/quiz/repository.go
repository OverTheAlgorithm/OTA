package quiz

import (
	"context"

	"github.com/google/uuid"
)

// Repository defines data access operations for quiz data.
type Repository interface {
	// SaveQuiz persists a single quiz record.
	SaveQuiz(ctx context.Context, quiz Quiz) error
	// SaveQuizBatch persists multiple quiz records in one operation.
	SaveQuizBatch(ctx context.Context, quizzes []Quiz) error
	// GetByContextItemID returns the quiz for a given article, or nil if none exists.
	GetByContextItemID(ctx context.Context, contextItemID uuid.UUID) (*Quiz, error)
	// GetUserAttempt returns the user's past attempt for the given quiz, or nil if none.
	// Used by GetQuizForUser to hydrate "already completed" cards.
	GetUserAttempt(ctx context.Context, userID string, quizID uuid.UUID) (*PastAttempt, error)
	// SaveResultAndAwardCoins executes in a SINGLE DB TRANSACTION:
	//   1. INSERT quiz_results (UNIQUE(user_id, quiz_id) prevents duplicates)
	//   2. If correct: INSERT coin_events (type='quiz_bonus', memo=topic name)
	//   3. If correct: UPDATE user_points (capped at coinCap)
	// Returns the new total coin balance.
	SaveResultAndAwardCoins(ctx context.Context, userID string, quizID, contextItemID uuid.UUID, answeredIndex int, isCorrect bool, coins, coinCap int, topicName string) (newTotal int, err error)
}
