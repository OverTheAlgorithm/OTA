package user

import (
	"context"
	"time"
)

// RoleChangeLog records a single role transition for audit purposes. ActorID
// is the admin who made the change (nullable when the platform itself triggers
// it, e.g. a future automatic demotion job).
type RoleChangeLog struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	BeforeRole string    `json:"before_role"`
	AfterRole  string    `json:"after_role"`
	ActorID    *string   `json:"actor_id,omitempty"`
	Memo       string    `json:"memo"`
	CreatedAt  time.Time `json:"created_at"`
}

// RoleChangeRepository persists and queries the role change audit log.
type RoleChangeRepository interface {
	Log(ctx context.Context, entry RoleChangeLog) (RoleChangeLog, error)
	ListByUser(ctx context.Context, userID string, limit, offset int) ([]RoleChangeLog, error)
}
