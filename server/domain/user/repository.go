package user

import "context"

type Repository interface {
	UpsertByKakaoID(ctx context.Context, kakaoID int64, email, nickname, profileImage string) (User, error)
	FindByID(ctx context.Context, id string) (User, error)
}
