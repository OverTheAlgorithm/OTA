package level

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

type mockRepo struct {
	points     UserPoints
	earnResult bool
	earnTotal  int
	earnErr    error
}

func (m *mockRepo) GetUserPoints(_ context.Context, _ string) (UserPoints, error) {
	return m.points, nil
}

func (m *mockRepo) EarnPoint(_ context.Context, _ string, _, _ uuid.UUID, _ int) (bool, int, error) {
	return m.earnResult, m.earnTotal, m.earnErr
}

func (m *mockRepo) DecayPoints(_ context.Context, _ int) (int, error) {
	return 0, nil
}

func (m *mockRepo) SetPoints(_ context.Context, _ string, _ int) error {
	return nil
}

func TestService_GetLevel(t *testing.T) {
	// 100pt → Lv2 with thresholds [0, 50, 200, 500, 1000]
	svc := NewService(&mockRepo{points: UserPoints{Points: 100}})
	info, err := svc.GetLevel(context.Background(), "user1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Level != 2 {
		t.Errorf("Level = %d, want 2", info.Level)
	}
	if info.CurrentProgress != 50 {
		t.Errorf("CurrentProgress = %d, want 50 (100-50)", info.CurrentProgress)
	}
	if info.PointsToNext != 150 {
		t.Errorf("PointsToNext = %d, want 150 (200-50)", info.PointsToNext)
	}
}

func TestService_EarnPoint_Success_LevelUp(t *testing.T) {
	// Lv1→Lv2: before=49, after=50
	svc := NewService(&mockRepo{
		points:     UserPoints{Points: 49},
		earnResult: true,
		earnTotal:  50,
	})
	res, err := svc.EarnPoint(context.Background(), "user1", uuid.New(), uuid.New(), false)
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
	// Within Lv2 range (50-199)
	svc := NewService(&mockRepo{
		points:     UserPoints{Points: 100},
		earnResult: true,
		earnTotal:  101,
	})
	res, err := svc.EarnPoint(context.Background(), "user1", uuid.New(), uuid.New(), false)
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

func TestService_EarnPoint_Duplicate(t *testing.T) {
	svc := NewService(&mockRepo{
		points:     UserPoints{Points: 20},
		earnResult: false,
		earnTotal:  0,
	})
	res, err := svc.EarnPoint(context.Background(), "user1", uuid.New(), uuid.New(), true)
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

// TestEarnPoint_AllLevelTransitions checks every level-up boundary:
// Thresholds: [0, 50, 200, 500, 1000]
func TestEarnPoint_AllLevelTransitions(t *testing.T) {
	transitions := []struct {
		name      string
		before    int
		after     int
		wantLevel int
		wantLevUp bool
	}{
		{"Lv1→Lv2", 49, 50, 2, true},
		{"Lv2→Lv3", 199, 200, 3, true},
		{"Lv3→Lv4", 499, 500, 4, true},
		{"Lv4→Lv5", 999, 1000, 5, true},
		{"Lv2 no up", 100, 101, 2, false},
		{"Lv3 no up", 250, 251, 3, false},
	}

	for _, tt := range transitions {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewService(&mockRepo{
				points:     UserPoints{Points: tt.before},
				earnResult: true,
				earnTotal:  tt.after,
			})
			res, err := svc.EarnPoint(context.Background(), "user1", uuid.New(), uuid.New(), false)
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
		points:     UserPoints{Points: 1050}, // already Lv5
		earnResult: true,
		earnTotal:  1051,
	})
	res, err := svc.EarnPoint(context.Background(), "user1", uuid.New(), uuid.New(), true)
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
	// Lv2: 50~199pt. At 100pt → progress=50 (100-50), needed=150 (200-50)
	svc := NewService(&mockRepo{
		points:     UserPoints{Points: 99},
		earnResult: true,
		earnTotal:  100,
	})
	res, err := svc.EarnPoint(context.Background(), "user1", uuid.New(), uuid.New(), true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Level != 2 {
		t.Errorf("Level = %d, want 2", res.Level)
	}
	if res.CurrentProgress != 50 {
		t.Errorf("CurrentProgress = %d, want 50 (100-50)", res.CurrentProgress)
	}
	if res.PointsToNext != 150 {
		t.Errorf("PointsToNext = %d, want 150 (200-50)", res.PointsToNext)
	}
}
