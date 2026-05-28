package comment

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestTargetType_Valid(t *testing.T) {
	cases := []struct {
		in   TargetType
		want bool
	}{
		{TargetTopic, true},
		{TargetEditorPick, true},
		{TargetType(""), false},
		{TargetType("post"), false},
		{TargetType("Topic"), false}, // case-sensitive
	}
	for _, c := range cases {
		if got := c.in.Valid(); got != c.want {
			t.Errorf("TargetType(%q).Valid() = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestSortOrder_Valid(t *testing.T) {
	cases := []struct {
		in   SortOrder
		want bool
	}{
		{SortPopular, true},
		{SortRecent, true},
		{SortOrder(""), false},
		{SortOrder("oldest"), false},
	}
	for _, c := range cases {
		if got := c.in.Valid(); got != c.want {
			t.Errorf("SortOrder(%q).Valid() = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestReaction_Valid(t *testing.T) {
	cases := []struct {
		in   Reaction
		want bool
	}{
		{ReactionLike, true},
		{ReactionDislike, true},
		{ReactionNone, true},
		{Reaction(2), false},
		{Reaction(-2), false},
	}
	for _, c := range cases {
		if got := c.in.Valid(); got != c.want {
			t.Errorf("Reaction(%d).Valid() = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestNormalizeContent(t *testing.T) {
	cases := map[string]string{
		"   hello   ":      "hello",
		"\nhi\n":           "hi",
		"line1\r\nline2":   "line1\nline2",
		"":                 "",
		" \t \n ":          "",
		"with  internal":   "with  internal", // preserve interior spacing
	}
	for in, want := range cases {
		if got := NormalizeContent(in); got != want {
			t.Errorf("NormalizeContent(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestValidateContent_Length(t *testing.T) {
	if err := ValidateContent(""); !errors.Is(err, ErrInvalidContent) {
		t.Errorf("empty content err = %v, want ErrInvalidContent", err)
	}
	if err := ValidateContent("a"); err != nil {
		t.Errorf("single char err = %v, want nil", err)
	}
	if err := ValidateContent(strings.Repeat("a", MaxContentLen)); err != nil {
		t.Errorf("max length err = %v, want nil", err)
	}
	if err := ValidateContent(strings.Repeat("a", MaxContentLen+1)); !errors.Is(err, ErrInvalidContent) {
		t.Errorf("over-max err = %v, want ErrInvalidContent", err)
	}
}

func TestResolveReplyDepth_NilParent(t *testing.T) {
	if _, _, err := ResolveReplyDepth(0, nil, nil); !errors.Is(err, ErrInvalidParent) {
		t.Errorf("nil parent err = %v, want ErrInvalidParent", err)
	}
}

func TestResolveReplyDepth_RootParent(t *testing.T) {
	parent := uuid.New()
	depth, eff, err := ResolveReplyDepth(0, &parent, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if depth != 1 {
		t.Errorf("depth = %d, want 1", depth)
	}
	if eff == nil || *eff != parent {
		t.Errorf("effective parent = %v, want %v", eff, parent)
	}
}

func TestResolveReplyDepth_ReplyParent_AttachesToGrandparent(t *testing.T) {
	parent := uuid.New()
	grandparent := uuid.New()
	depth, eff, err := ResolveReplyDepth(MaxDepth, &parent, &grandparent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if depth != MaxDepth {
		t.Errorf("depth = %d, want %d", depth, MaxDepth)
	}
	if eff == nil || *eff != grandparent {
		t.Errorf("effective parent = %v, want grandparent %v", eff, grandparent)
	}
}

func TestResolveReplyDepth_ReplyParentMissingGrandparent_Rejects(t *testing.T) {
	parent := uuid.New()
	if _, _, err := ResolveReplyDepth(MaxDepth, &parent, nil); !errors.Is(err, ErrInvalidParent) {
		t.Errorf("missing grandparent err = %v, want ErrInvalidParent", err)
	}
}

func TestResolveReplyDepth_InvalidDepth(t *testing.T) {
	parent := uuid.New()
	for _, badDepth := range []int{-1, 2, 5} {
		if _, _, err := ResolveReplyDepth(badDepth, &parent, nil); !errors.Is(err, ErrInvalidParent) {
			t.Errorf("depth %d err = %v, want ErrInvalidParent", badDepth, err)
		}
	}
}

func TestClampLimit(t *testing.T) {
	cases := map[int]int{
		0:               DefaultPageSize,
		-5:              DefaultPageSize,
		5:               5,
		DefaultPageSize: DefaultPageSize,
		MaxPageSize:     MaxPageSize,
		MaxPageSize + 1: MaxPageSize,
		1000:            MaxPageSize,
	}
	for in, want := range cases {
		if got := ClampLimit(in); got != want {
			t.Errorf("ClampLimit(%d) = %d, want %d", in, got, want)
		}
	}
}

func TestComment_IsDeleted(t *testing.T) {
	c := Comment{}
	if c.IsDeleted() {
		t.Error("zero-value Comment.IsDeleted() = true, want false")
	}
	now := time.Now()
	c.DeletedAt = &now
	if !c.IsDeleted() {
		t.Error("deleted Comment.IsDeleted() = false, want true")
	}
}

func TestComment_AuthorDisplayName(t *testing.T) {
	cases := []struct {
		nickname string
		penName  string
		want     string
	}{
		{"alice", "", "alice"},
		{"alice", "writer-a", "writer-a"},
		{"alice", "   ", "alice"}, // whitespace pen_name falls back
		{"", "writer-a", "writer-a"},
	}
	for _, c := range cases {
		got := Comment{AuthorNickname: c.nickname, AuthorPenName: c.penName}.AuthorDisplayName()
		if got != c.want {
			t.Errorf("AuthorDisplayName(nickname=%q, pen=%q) = %q, want %q",
				c.nickname, c.penName, got, c.want)
		}
	}
}
