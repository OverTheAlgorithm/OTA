package user

import "context"

type Repository interface {
	UpsertByKakaoID(ctx context.Context, kakaoID int64, email, nickname, profileImage string) (User, error)
	FindByID(ctx context.Context, id string) (User, error)
	FindByKakaoID(ctx context.Context, kakaoID int64) (User, bool, error)
	FindByEmail(ctx context.Context, email string) (User, error)
	UpdateEmail(ctx context.Context, userID string, email string) error
	DeleteByID(ctx context.Context, userID string) error
}
