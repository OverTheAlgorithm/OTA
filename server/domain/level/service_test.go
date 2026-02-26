package level

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

type mockRepo struct {
	points        UserPoints
	earnResult    bool
	earnTotal     int
	earnErr       error
	lastEarned    time.Time
	hasLastEarned bool
	lastEarnedErr error
}

func (m *mockRepo) GetUserPoints(_ context.Context, _ string) (UserPoints, error) {
	return m.points, nil
}

func (m *mockRepo) EarnPoint(_ context.Context, _ string, _, _ uuid.UUID, _ int) (bool, int, error) {
	return m.earnResult, m.earnTotal, m.earnErr
}

func (m *mockRepo) GetLastEarnedAt(_ context.Context, _ string) (time.Time, bool, error) {
	return m.lastEarned, m.hasLastEarned, m.lastEarnedErr
}

func (m *mockRepo) GetLastEarnedAtBatch(_ context.Context, _ []string) (map[string]time.Time, error) {
	return nil, nil
}

func (m *mockRepo) DecayPoints(_ context.Context, _ int) (int, error) {
	return 0, nil
}

func (m *mockRepo) SetPoints(_ context.Context, _ string, _ int) error {
	return nil
}

func TestService_GetLevel(t *testing.T) {
	// 22pt → Lv2 with new thresholds [0, 15, 45, 90, 165]
	svc := NewService(&mockRepo{points: UserPoints{Points: 22}})
	info, err := svc.GetLevel(context.Background(), "user1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Level != 2 {
		t.Errorf("Level = %d, want 2", info.Level)
	}
	if info.CurrentProgress != 7 {
		t.Errorf("CurrentProgress = %d, want 7 (22-15)", info.CurrentProgress)
	}
}

func TestService_EarnPoint_Success_LevelUp(t *testing.T) {
	// Lv1→Lv2: before=14, after=15
	svc := NewService(&mockRepo{
		points:     UserPoints{Points: 14},
		earnResult: true,
		earnTotal:  15,
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
	// Within Lv2 range (15-44)
	svc := NewService(&mockRepo{
		points:     UserPoints{Points: 16},
		earnResult: true,
		earnTotal:  17,
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
// Thresholds: [0, 15, 45, 90, 165]
func TestEarnPoint_AllLevelTransitions(t *testing.T) {
	transitions := []struct {
		name      string
		before    int
		after     int
		wantLevel int
		wantLevUp bool
	}{
		{"Lv1→Lv2", 14, 15, 2, true},
		{"Lv2→Lv3", 44, 45, 3, true},
		{"Lv3→Lv4", 89, 90, 4, true},
		{"Lv4→Lv5", 164, 165, 5, true},
		{"Lv2 no up", 16, 17, 2, false},
		{"Lv3 no up", 50, 51, 3, false},
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
		points:     UserPoints{Points: 170}, // already Lv5
		earnResult: true,
		earnTotal:  171,
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
	// Lv2: 15~44pt. At 20pt → progress=5 (20-15), needed=30 (45-15)
	svc := NewService(&mockRepo{
		points:     UserPoints{Points: 19},
		earnResult: true,
		earnTotal:  20,
	})
	res, err := svc.EarnPoint(context.Background(), "user1", uuid.New(), uuid.New(), true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Level != 2 {
		t.Errorf("Level = %d, want 2", res.Level)
	}
	if res.CurrentProgress != 5 {
		t.Errorf("CurrentProgress = %d, want 5 (20-15)", res.CurrentProgress)
	}
	if res.PointsToNext != 30 {
		t.Errorf("PointsToNext = %d, want 30 (45-15)", res.PointsToNext)
	}
}
