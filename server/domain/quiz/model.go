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
type QuizForUser struct {
	ID            uuid.UUID `json:"id"`
	ContextItemID uuid.UUID `json:"context_item_id"`
	Question      string    `json:"question"`
	Options       []string  `json:"options"`
}

// SubmitResult is the response after a user submits a quiz answer.
// NOTE: CorrectIndex is intentionally absent — wrong answers do NOT reveal the correct answer.
type SubmitResult struct {
	Correct     bool `json:"correct"`
	CoinsEarned int  `json:"coins_earned"`
	TotalCoins  int  `json:"total_coins"`
}
