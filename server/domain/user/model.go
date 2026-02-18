package user

import "time"

type User struct {
	ID           string    `json:"id"`
	KakaoID      int64     `json:"kakao_id"`
	Email        string    `json:"email,omitempty"`
	Nickname     string    `json:"nickname,omitempty"`
	ProfileImage string    `json:"profile_image,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
