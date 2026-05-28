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

// Nickname limits. Same bounds as pen names — short names are easier to
// impersonate, long ones break list layouts.
const (
	MinNicknameLen = 2
	MaxNicknameLen = 32
)

// NicknameState values mirror the DB CHECK constraint. The state machine
// is forward-only: default → acknowledged → custom, or default → custom.
const (
	NicknameStateDefault      = "default"
	NicknameStateAcknowledged = "acknowledged"
	NicknameStateCustom       = "custom"
)

var (
	ErrPenNameTooShort  = errors.New("필명은 2자 이상이어야 합니다")
	ErrPenNameTooLong   = errors.New("필명은 32자 이내여야 합니다")
	ErrPenNameTaken     = errors.New("이미 사용 중인 필명입니다")
	ErrNicknameTooShort = errors.New("닉네임은 2자 이상이어야 합니다")
	ErrNicknameTooLong  = errors.New("닉네임은 32자 이내여야 합니다")
	ErrNicknameEmpty    = errors.New("닉네임을 입력해주세요")
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

// NormaliseNickname trims surrounding whitespace and validates the length.
// Unlike pen names, the empty case is not allowed — every user must have a
// displayable nickname (it falls back to Kakao on signup but must not be
// cleared afterwards).
func NormaliseNickname(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", ErrNicknameEmpty
	}
	n := len([]rune(trimmed))
	if n < MinNicknameLen {
		return "", fmt.Errorf("%w (현재 %d자)", ErrNicknameTooShort, n)
	}
	if n > MaxNicknameLen {
		return "", fmt.Errorf("%w (현재 %d자)", ErrNicknameTooLong, n)
	}
	return trimmed, nil
}

// IsValidNicknameState reports whether v is one of the recognized
// nickname state values.
func IsValidNicknameState(v string) bool {
	switch v {
	case NicknameStateDefault, NicknameStateAcknowledged, NicknameStateCustom:
		return true
	default:
		return false
	}
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
	NicknameState string    `json:"nickname_state"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// NeedsNicknameWarning reports whether the comment-composer first-time
// warning modal should be shown for this user. Only the `default` state
// triggers it — both `acknowledged` (user dismissed) and `custom` (user
// chose) suppress the modal.
func (u User) NeedsNicknameWarning() bool {
	return u.NicknameState == NicknameStateDefault
}
