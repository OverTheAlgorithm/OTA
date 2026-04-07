package push

import "context"

// Repository defines persistence operations for push tokens.
type Repository interface {
	// Save upserts a push token. On conflict (same token), updates platform and user_id.
	Save(ctx context.Context, token PushToken) error
	// UnlinkUser sets user_id to NULL for a token owned by the given user.
	UnlinkUser(ctx context.Context, userID, token string) error
	// DeleteByTokens removes push tokens matching any of the given token strings.
	DeleteByTokens(ctx context.Context, tokens []string) error
	// GetByUserID returns all push tokens linked to a user.
	GetByUserID(ctx context.Context, userID string) ([]PushToken, error)
	// GetAllActive returns all push tokens (including anonymous).
	GetAllActive(ctx context.Context) ([]PushToken, error)
}
