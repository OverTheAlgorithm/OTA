package level

import (
	"context"

	"github.com/google/uuid"
)

type Repository interface {
	// GetUserCoins returns current coins for a user. Returns Lv1/0 default if no record exists.
	GetUserCoins(ctx context.Context, userID string) (UserCoins, error)
	// EarnCoin attempts to award coins for viewing a context item within a collection run.
	// Returns (false, 0, nil) if already earned (duplicate), (true, newTotal, nil) on success.
	EarnCoin(ctx context.Context, userID string, runID uuid.UUID, contextItemID uuid.UUID, coins int) (earned bool, newTotal int, err error)
	// DecayCoins subtracts 1 coin from all users (minimum 0) in batches of batchSize.
	// Returns the total number of users updated.
	DecayCoins(ctx context.Context, batchSize int) (int, error)
	// SetCoins directly overwrites a user's coins and recalculates level. For testing only.
	SetCoins(ctx context.Context, userID string, coins int) error
}
