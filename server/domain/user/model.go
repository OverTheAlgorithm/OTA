package user

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Role constants used throughout the codebase. The string values match the
// CHECK constraint on users.role.
const (
	RoleUser   = "user"
	RoleEditor = "editor"
	RoleAdmin  = "admin"
)

// Pen name limits. The DB CHECK constraint mirrors these bounds.
const (
	MinPenNameLen = 2
	MaxPenNameLen = 32
)

var (
	ErrPenNameTooShort = errors.New("필명은 2자 이상이어야 합니다")
	ErrPenNameTooLong  = errors.New("필명은 32자 이내여야 합니다")
	ErrPenNameTaken    = errors.New("이미 사용 중인 필명입니다")
)

// NormalisePenName trims surrounding whitespace and validates the length. An
// empty (or whitespace-only) input is allowed and means "clear the pen name" —
// callers persist it as NULL so display logic falls back to nickname.
func NormalisePenName(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}
	n := len([]rune(trimmed))
	if n < MinPenNameLen {
		return "", fmt.Errorf("%w (현재 %d자)", ErrPenNameTooShort, n)
	}
	if n > MaxPenNameLen {
		return "", fmt.Errorf("%w (현재 %d자)", ErrPenNameTooLong, n)
	}
	return trimmed, nil
}

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
	PenName       string    `json:"pen_name,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
