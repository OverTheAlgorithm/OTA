package user

import "testing"

func TestHasRoleAtLeast(t *testing.T) {
	tests := []struct {
		role string
		min  string
		want bool
	}{
		{RoleUser, RoleUser, true},
		{RoleUser, RoleEditor, false},
		{RoleUser, RoleAdmin, false},
		{RoleEditor, RoleUser, true},
		{RoleEditor, RoleEditor, true},
		{RoleEditor, RoleAdmin, false},
		{RoleAdmin, RoleUser, true},
		{RoleAdmin, RoleEditor, true},
		{RoleAdmin, RoleAdmin, true},
		// Unknown values fall back to the lowest tier.
		{"", RoleUser, true},
		{"", RoleEditor, false},
		{"super-admin", RoleAdmin, false},
		{RoleAdmin, "unknown", true},
	}
	for _, tc := range tests {
		t.Run(tc.role+"_>=_"+tc.min, func(t *testing.T) {
			if got := HasRoleAtLeast(tc.role, tc.min); got != tc.want {
				t.Fatalf("HasRoleAtLeast(%q, %q) = %v, want %v", tc.role, tc.min, got, tc.want)
			}
		})
	}
}

func TestIsValidRole(t *testing.T) {
	cases := map[string]bool{
		RoleUser:    true,
		RoleEditor:  true,
		RoleAdmin:   true,
		"":          false,
		"superuser": false,
	}
	for in, want := range cases {
		if got := IsValidRole(in); got != want {
			t.Errorf("IsValidRole(%q) = %v, want %v", in, got, want)
		}
	}
}
