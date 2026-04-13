package poll

import (
	"context"

	"github.com/google/uuid"
)

// Repository defines data access for polls and votes.
type Repository interface {
	SavePollBatch(ctx context.Context, polls []Poll) error
	GetByContextItemID(ctx context.Context, contextItemID uuid.UUID) (*Poll, error)
	// CountRawTallies returns only option_index rows that have votes. Service pads zeros.
	CountRawTallies(ctx context.Context, pollID uuid.UUID) ([]VoteTally, error)
	GetUserVoteIndex(ctx context.Context, userID string, pollID uuid.UUID) (*int, error)
	// InsertVote records a single vote. Returns ErrAlreadyVoted if the user has already voted.
	InsertVote(ctx context.Context, userID string, pollID uuid.UUID, optionIndex int) error

	// Admin operations.
	UpdatePollAndMaybeResetVotes(ctx context.Context, pollID uuid.UUID, question string, options []string, resetVotes bool) error
	DeleteByContextItemID(ctx context.Context, contextItemID uuid.UUID) error
}
