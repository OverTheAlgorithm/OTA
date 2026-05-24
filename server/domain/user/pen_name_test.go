package user

import (
	"errors"
	"testing"
)

func TestNormalisePenName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr error
	}{
		{"empty input clears", "", "", nil},
		{"plain spaces clear", "    ", "", nil},
		{"tabs and newlines clear", "  \n\t \r ", "", nil},
		{"single char too short", "a", "", ErrPenNameTooShort},
		{"single hangul too short", "가", "", ErrPenNameTooShort},
		{"two chars accepted", "가나", "가나", nil},
		{"trims surrounding whitespace", "  필명  ", "필명", nil},
		{"trims newlines and tabs", "\n\t에디터\r ", "에디터", nil},
		{"max length accepted", "가나다라마바사아자차카타파하12345678901234567890", "", ErrPenNameTooLong},
		{"32 char limit", "12345678901234567890123456789012", "12345678901234567890123456789012", nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NormalisePenName(tc.input)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
