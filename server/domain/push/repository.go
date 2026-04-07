package push

import "context"

// Repository defines persistence operations for push tokens.
type Repository interface {
	Save(ctx context.Context, token PushToken) error
	Delete(ctx context.Context, userID, token string) error
	GetByUserID(ctx context.Context, userID string) ([]PushToken, error)
	GetAllActive(ctx context.Context) ([]PushToken, error)
}
