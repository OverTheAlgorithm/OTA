package level

import (
	"context"

	"github.com/google/uuid"
)

type Repository interface {
	// GetUserPoints returns current points for a user. Returns Lv1/0pt default if no record exists.
	GetUserPoints(ctx context.Context, userID string) (UserPoints, error)
	// EarnPoint attempts to award 1 point for viewing a context item.
	// Returns (false, 0, nil) if already earned (duplicate), (true, newTotal, nil) on success.
	EarnPoint(ctx context.Context, userID string, contextItemID uuid.UUID) (earned bool, newTotal int, err error)
	// GetBrainCategory returns the brain_category of a context_item by its ID.
	GetBrainCategory(ctx context.Context, contextItemID uuid.UUID) (string, error)
	// SetPoints directly overwrites a user's points and recalculates level. For testing only.
	SetPoints(ctx context.Context, userID string, points int) error
}
