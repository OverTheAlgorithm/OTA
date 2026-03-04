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
	// SetCoins directly overwrites a user's coins and recalculates level. For testing only.
	SetCoins(ctx context.Context, userID string, coins int) error
	// GetTodayEarnedCoins returns the total coins a user has earned today (KST).
	GetTodayEarnedCoins(ctx context.Context, userID string) (int, error)
	// HasEarned reports whether the user has already earned a coin for the given run+item combination.
	HasEarned(ctx context.Context, userID string, runID, contextItemID uuid.UUID) (bool, error)
}
