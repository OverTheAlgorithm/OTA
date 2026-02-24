package level

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

type mockRepo struct {
	points        UserPoints
	brainCategory string
	earnResult    bool
	earnTotal     int
	earnErr       error
	bcErr         error
}

func (m *mockRepo) GetUserPoints(_ context.Context, _ string) (UserPoints, error) {
	return m.points, nil
}

func (m *mockRepo) EarnPoint(_ context.Context, _ string, _ uuid.UUID) (bool, int, error) {
	return m.earnResult, m.earnTotal, m.earnErr
}

func (m *mockRepo) GetBrainCategory(_ context.Context, _ uuid.UUID) (string, error) {
	return m.brainCategory, m.bcErr
}

func (m *mockRepo) SetPoints(_ context.Context, _ string, _ int) error {
	return nil
}

func TestService_GetLevel(t *testing.T) {
	svc := NewService(&mockRepo{points: UserPoints{Points: 22}})
	info, err := svc.GetLevel(context.Background(), "user1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Level != 3 {
		t.Errorf("Level = %d, want 3", info.Level)
	}
	if info.CurrentProgress != 7 {
		t.Errorf("CurrentProgress = %d, want 7", info.CurrentProgress)
	}
}

func TestService_EarnPoint_Success_LevelUp(t *testing.T) {
	svc := NewService(&mockRepo{
		points:        UserPoints{Points: 4}, // Lv1, 1 more = 5 = Lv2
		brainCategory: BrainCategoryKey,
		earnResult:    true,
		earnTotal:     5,
	})
	res, err := svc.EarnPoint(context.Background(), "user1", uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Earned {
		t.Error("expected earned = true")
	}
	if !res.LeveledUp {
		t.Error("expected leveled_up = true (Lv1 -> Lv2)")
	}
	if res.Level != 2 {
		t.Errorf("Level = %d, want 2", res.Level)
	}
}

func TestService_EarnPoint_Success_NoLevelUp(t *testing.T) {
	svc := NewService(&mockRepo{
		points:        UserPoints{Points: 6}, // Lv2
		brainCategory: BrainCategoryKey,
		earnResult:    true,
		earnTotal:     7,
	})
	res, err := svc.EarnPoint(context.Background(), "user1", uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Earned {
		t.Error("expected earned = true")
	}
	if res.LeveledUp {
		t.Error("expected leveled_up = false")
	}
}

func TestService_EarnPoint_WrongCategory(t *testing.T) {
	svc := NewService(&mockRepo{brainCategory: "entertainment"})
	res, err := svc.EarnPoint(context.Background(), "user1", uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Earned {
		t.Error("expected earned = false for wrong category")
	}
}

func TestService_EarnPoint_Duplicate(t *testing.T) {
	svc := NewService(&mockRepo{
		points:        UserPoints{Points: 10},
		brainCategory: BrainCategoryKey,
		earnResult:    false,
		earnTotal:     0,
	})
	res, err := svc.EarnPoint(context.Background(), "user1", uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Earned {
		t.Error("expected earned = false for duplicate")
	}
	if res.LeveledUp {
		t.Error("expected leveled_up = false for duplicate")
	}
}

func TestService_EarnPoint_BrainCategoryError(t *testing.T) {
	svc := NewService(&mockRepo{bcErr: errors.New("db error")})
	_, err := svc.EarnPoint(context.Background(), "user1", uuid.New())
	if err == nil {
		t.Error("expected error when brain category lookup fails")
	}
}

// TestEarnPoint_AllLevelTransitions checks every level-up boundary:
// Lv1→Lv2 at 5pt, Lv2→Lv3 at 15pt, Lv3→Lv4 at 30pt, Lv4→Lv5 at 55pt
func TestEarnPoint_AllLevelTransitions(t *testing.T) {
	transitions := []struct {
		name       string
		before     int // points before earning
		after      int // points after earning
		wantLevel  int
		wantLevUp  bool
	}{
		{"Lv1→Lv2", 4, 5, 2, true},
		{"Lv2→Lv3", 14, 15, 3, true},
		{"Lv3→Lv4", 29, 30, 4, true},
		{"Lv4→Lv5", 54, 55, 5, true},
		{"Lv2 no up", 6, 7, 2, false},
		{"Lv3 no up", 20, 21, 3, false},
	}

	for _, tt := range transitions {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewService(&mockRepo{
				points:        UserPoints{Points: tt.before},
				brainCategory: BrainCategoryKey,
				earnResult:    true,
				earnTotal:     tt.after,
			})
			res, err := svc.EarnPoint(context.Background(), "user1", uuid.New())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !res.Earned {
				t.Error("expected earned = true")
			}
			if res.Level != tt.wantLevel {
				t.Errorf("Level = %d, want %d", res.Level, tt.wantLevel)
			}
			if res.LeveledUp != tt.wantLevUp {
				t.Errorf("LeveledUp = %v, want %v", res.LeveledUp, tt.wantLevUp)
			}
			if res.TotalPoints != tt.after {
				t.Errorf("TotalPoints = %d, want %d", res.TotalPoints, tt.after)
			}
		})
	}
}

// TestEarnPoint_AtMaxLevel verifies that earning at Lv5 records the point
// but does not set LeveledUp (already max).
func TestEarnPoint_AtMaxLevel(t *testing.T) {
	svc := NewService(&mockRepo{
		points:        UserPoints{Points: 60}, // already Lv5
		brainCategory: BrainCategoryKey,
		earnResult:    true,
		earnTotal:     61,
	})
	res, err := svc.EarnPoint(context.Background(), "user1", uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Earned {
		t.Error("expected earned = true even at max level")
	}
	if res.LeveledUp {
		t.Error("expected leveled_up = false at max level")
	}
	if res.Level != MaxLevel {
		t.Errorf("Level = %d, want %d", res.Level, MaxLevel)
	}
	if res.PointsToNext != 0 {
		t.Errorf("PointsToNext = %d, want 0 at max level", res.PointsToNext)
	}
}

// TestEarnPoint_ProgressCalc verifies CurrentProgress and PointsToNext
// are calculated correctly within a level.
func TestEarnPoint_ProgressCalc(t *testing.T) {
	// Lv2: 5~14pt, range=10. At 8pt → progress=3, needed=10
	svc := NewService(&mockRepo{
		points:        UserPoints{Points: 7},
		brainCategory: BrainCategoryKey,
		earnResult:    true,
		earnTotal:     8,
	})
	res, err := svc.EarnPoint(context.Background(), "user1", uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Level != 2 {
		t.Errorf("Level = %d, want 2", res.Level)
	}
	if res.CurrentProgress != 3 {
		t.Errorf("CurrentProgress = %d, want 3 (8-5)", res.CurrentProgress)
	}
	if res.PointsToNext != 10 {
		t.Errorf("PointsToNext = %d, want 10 (15-5)", res.PointsToNext)
	}
}
