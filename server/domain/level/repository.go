package level

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Repository interface {
	// GetUserPoints returns current points for a user. Returns Lv1/0pt default if no record exists.
	GetUserPoints(ctx context.Context, userID string) (UserPoints, error)
	// EarnPoint attempts to award points for viewing a context item within a collection run.
	// Returns (false, 0, nil) if already earned (duplicate), (true, newTotal, nil) on success.
	EarnPoint(ctx context.Context, userID string, runID uuid.UUID, contextItemID uuid.UUID, points int) (earned bool, newTotal int, err error)
	// GetLastEarnedAt returns the time of the user's most recent point earn.
	// Returns (zero, false, nil) if the user has never earned points.
	GetLastEarnedAt(ctx context.Context, userID string) (time.Time, bool, error)
	// GetLastEarnedAtBatch returns the most recent earn time for multiple users at once.
	// Users with no earn history are omitted from the result map.
	GetLastEarnedAtBatch(ctx context.Context, userIDs []string) (map[string]time.Time, error)
	// DecayPoints subtracts 1 point from all users (minimum 0) in batches of batchSize.
	// Returns the total number of users updated.
	DecayPoints(ctx context.Context, batchSize int) (int, error)
	// SetPoints directly overwrites a user's points and recalculates level. For testing only.
	SetPoints(ctx context.Context, userID string, points int) error
}
