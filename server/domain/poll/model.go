package poll

import (
	"time"

	"github.com/google/uuid"
)

// Poll is the full poll record.
type Poll struct {
	ID            uuid.UUID
	ContextItemID uuid.UUID
	Question      string
	Options       []string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// VoteTally is per-option count. Tallies returned from the service are padded
// (length == len(options), zeros filled) so the client can index by option_index.
type VoteTally struct {
	OptionIndex int `json:"option_index"`
	Count       int `json:"count"`
}

// PollForUser is the poll payload sent to the frontend.
// UserVoteIndex is nil for non-logged-in viewers and logged-in users who have not voted yet.
type PollForUser struct {
	ID            uuid.UUID   `json:"id"`
	ContextItemID uuid.UUID   `json:"context_item_id"`
	Question      string      `json:"question"`
	Options       []string    `json:"options"`
	Tallies       []VoteTally `json:"tallies"`
	TotalVotes    int         `json:"total_votes"`
	UserVoteIndex *int        `json:"user_vote_index"`
}
