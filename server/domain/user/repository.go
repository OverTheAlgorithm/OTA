package user

import "context"

type Repository interface {
	UpsertByKakaoID(ctx context.Context, kakaoID int64, email, nickname, profileImage string) (User, error)
	FindByID(ctx context.Context, id string) (User, error)
	FindByKakaoID(ctx context.Context, kakaoID int64) (User, bool, error)
	FindByEmail(ctx context.Context, email string) (User, error)
	UpdateEmail(ctx context.Context, userID string, email string) error
	UpdateRole(ctx context.Context, userID, newRole string) error
	// UpdatePenName persists a normalised pen name (or empty string to clear it).
	// Returns ErrPenNameTaken if another user already holds the same name (case
	// insensitive).
	UpdatePenName(ctx context.Context, userID, penName string) error
	// UpdateNickname overwrites the nickname and advances nickname_state to
	// 'custom'. Callers must pre-normalise via NormaliseNickname.
	UpdateNickname(ctx context.Context, userID, nickname string) error
	// AcknowledgeNicknameWarning advances nickname_state from 'default' to
	// 'acknowledged'. A no-op for any other state, so the call is idempotent.
	AcknowledgeNicknameWarning(ctx context.Context, userID string) error
	DeleteByID(ctx context.Context, userID string) error
}
