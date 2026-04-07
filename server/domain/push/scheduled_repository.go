package push

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ScheduledRepository defines persistence operations for scheduled push notifications.
type ScheduledRepository interface {
	Create(ctx context.Context, p ScheduledPush) (ScheduledPush, error)
	GetByID(ctx context.Context, id uuid.UUID) (ScheduledPush, error)
	Update(ctx context.Context, p ScheduledPush) error
	List(ctx context.Context, status *string) ([]ScheduledPush, error)
	ListPending(ctx context.Context) ([]ScheduledPush, error)
	// CAS operations: all use WHERE status='pending' and return (updated bool, error).
	// Returns (true, nil) if 1 row updated, (false, nil) if 0 rows (already processed).
	MarkSent(ctx context.Context, id uuid.UUID, sentAt time.Time) (bool, error)
	MarkFailed(ctx context.Context, id uuid.UUID, errMsg string) (bool, error)
	MarkCancelled(ctx context.Context, id uuid.UUID) (bool, error)
}
