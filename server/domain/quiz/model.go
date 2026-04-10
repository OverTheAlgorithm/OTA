package quiz

import (
	"time"

	"github.com/google/uuid"
)

// Quiz holds the full quiz data including the correct answer (server-side only).
type Quiz struct {
	ID            uuid.UUID
	ContextItemID uuid.UUID
	Question      string
	Options       []string
	CorrectIndex  int
	CreatedAt     time.Time
}

// QuizForUser is the quiz data sent to the frontend (correct answer omitted).
// PastAttempt is non-nil when the user has already submitted an answer; it lets the
// frontend hydrate a static "already completed" card without exposing the correct index.
type QuizForUser struct {
	ID            uuid.UUID    `json:"id"`
	ContextItemID uuid.UUID    `json:"context_item_id"`
	Question      string       `json:"question"`
	Options       []string     `json:"options"`
	PastAttempt   *PastAttempt `json:"past_attempt,omitempty"`
}

// PastAttempt summarises a user's previous quiz submission for hydration.
// NOTE: The correct answer index is intentionally absent — only the user's chosen
// option and whether it was correct. This preserves the no-reveal contract.
type PastAttempt struct {
	SelectedIndex int       `json:"selected_index"`
	IsCorrect     bool      `json:"is_correct"`
	CoinsEarned   int       `json:"coins_earned"`
	AttemptedAt   time.Time `json:"attempted_at"`
}

// SubmitResult is the response after a user submits a quiz answer.
// NOTE: CorrectIndex is intentionally absent — wrong answers do NOT reveal the correct answer.
type SubmitResult struct {
	Correct     bool `json:"correct"`
	CoinsEarned int  `json:"coins_earned"`
	TotalCoins  int  `json:"total_coins"`
}
