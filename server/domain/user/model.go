package user

import "time"

// Role constants used throughout the codebase. The string values match the
// CHECK constraint on users.role.
const (
	RoleUser   = "user"
	RoleEditor = "editor"
	RoleAdmin  = "admin"
)

// roleRank gives an ordered precedence to the named roles. Unknown values rank
// 0 (same as RoleUser) so an unexpected value never accidentally grants
// elevated permissions.
var roleRank = map[string]int{
	RoleUser:   0,
	RoleEditor: 1,
	RoleAdmin:  2,
}

// HasRoleAtLeast reports whether `role` is equal to or higher than `min` in the
// role hierarchy (user < editor < admin). Empty strings or unknown values are
// treated as the lowest tier.
func HasRoleAtLeast(role, min string) bool {
	return roleRank[role] >= roleRank[min]
}

// IsValidRole reports whether v is one of the known role identifiers.
func IsValidRole(v string) bool {
	_, ok := roleRank[v]
	return ok
}

type User struct {
	ID            string    `json:"id"`
	KakaoID       int64     `json:"kakao_id"`
	Email         string    `json:"email,omitempty"`
	EmailVerified bool      `json:"email_verified"`
	Nickname      string    `json:"nickname,omitempty"`
	ProfileImage  string    `json:"profile_image,omitempty"`
	Role          string    `json:"role"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
