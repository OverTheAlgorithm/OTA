package user

import (
	"errors"
	"strings"
	"testing"
)

func TestNormaliseNickname(t *testing.T) {
	cases := []struct {
		name      string
		input     string
		want      string
		wantErrIs error
	}{
		{"basic", "alice", "alice", nil},
		{"trims whitespace", "  alice  ", "alice", nil},
		{"empty rejected", "", "", ErrNicknameEmpty},
		{"whitespace only rejected", "   ", "", ErrNicknameEmpty},
		{"too short", "a", "", ErrNicknameTooShort},
		{"two chars accepted", "ab", "ab", nil},
		{"max length accepted", strings.Repeat("a", MaxNicknameLen), strings.Repeat("a", MaxNicknameLen), nil},
		{"too long", strings.Repeat("a", MaxNicknameLen+1), "", ErrNicknameTooLong},
		{"hangul counted as runes", "한", "", ErrNicknameTooShort},
		{"hangul ok at 2 runes", "한글", "한글", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NormaliseNickname(tc.input)
			if tc.wantErrIs == nil {
				if err != nil {
					t.Errorf("err = %v, want nil", err)
				}
				if got != tc.want {
					t.Errorf("got = %q, want %q", got, tc.want)
				}
				return
			}
			if !errors.Is(err, tc.wantErrIs) {
				t.Errorf("err = %v, want %v", err, tc.wantErrIs)
			}
		})
	}
}

func TestIsValidNicknameState(t *testing.T) {
	for _, v := range []string{NicknameStateDefault, NicknameStateAcknowledged, NicknameStateCustom} {
		if !IsValidNicknameState(v) {
			t.Errorf("IsValidNicknameState(%q) = false, want true", v)
		}
	}
	for _, v := range []string{"", "kakao_default", "invalid"} {
		if IsValidNicknameState(v) {
			t.Errorf("IsValidNicknameState(%q) = true, want false", v)
		}
	}
}

func TestUser_NeedsNicknameWarning(t *testing.T) {
	cases := map[string]bool{
		NicknameStateDefault:      true,
		NicknameStateAcknowledged: false,
		NicknameStateCustom:       false,
		"":                        false,
	}
	for state, want := range cases {
		got := User{NicknameState: state}.NeedsNicknameWarning()
		if got != want {
			t.Errorf("state=%q NeedsNicknameWarning() = %v, want %v", state, got, want)
		}
	}
}
