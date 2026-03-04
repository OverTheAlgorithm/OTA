package level

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

type mockRepo struct {
	coins       UserCoins
	earnResult  bool
	earnTotal   int
	earnErr     error
	todayEarned int
}

func (m *mockRepo) GetUserCoins(_ context.Context, _ string) (UserCoins, error) {
	return m.coins, nil
}

func (m *mockRepo) EarnCoin(_ context.Context, _ string, _, _ uuid.UUID, _ int) (bool, int, error) {
	return m.earnResult, m.earnTotal, m.earnErr
}

func (m *mockRepo) SetCoins(_ context.Context, _ string, _ int) error {
	return nil
}

func (m *mockRepo) GetTodayEarnedCoins(_ context.Context, _ string) (int, error) {
	return m.todayEarned, nil
}

func (m *mockRepo) HasEarned(_ context.Context, _ string, _, _ uuid.UUID) (bool, error) {
	return false, nil
}

func TestService_GetLevel(t *testing.T) {
	// 100코인 → Lv2 with thresholds [0, 50, 200, 500, 1000]
	svc := NewService(&mockRepo{coins: UserCoins{Coins: 100}}, 0)
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
	if info.CoinsToNext != 150 {
		t.Errorf("CoinsToNext = %d, want 150 (200-50)", info.CoinsToNext)
	}
}

func TestService_EarnCoin_Success_LevelUp(t *testing.T) {
	// Lv1→Lv2: before=49, after=50
	svc := NewService(&mockRepo{
		coins:      UserCoins{Coins: 49},
		earnResult: true,
		earnTotal:  50,
	}, 0)
	res, err := svc.EarnCoin(context.Background(), "user1", uuid.New(), uuid.New(), false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Earned {
		t.Error("expected earned = true")
	}
	if res.Reason != ReasonEarned {
		t.Errorf("Reason = %q, want %q", res.Reason, ReasonEarned)
	}
	if !res.LeveledUp {
		t.Error("expected leveled_up = true (Lv1 -> Lv2)")
	}
	if res.Level != 2 {
		t.Errorf("Level = %d, want 2", res.Level)
	}
}

func TestService_EarnCoin_Success_NoLevelUp(t *testing.T) {
	// Within Lv2 range (50-199)
	svc := NewService(&mockRepo{
		coins:      UserCoins{Coins: 100},
		earnResult: true,
		earnTotal:  101,
	}, 0)
	res, err := svc.EarnCoin(context.Background(), "user1", uuid.New(), uuid.New(), false)
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

func TestService_EarnCoin_Duplicate(t *testing.T) {
	svc := NewService(&mockRepo{
		coins:      UserCoins{Coins: 20},
		earnResult: false,
		earnTotal:  0,
	}, 0)
	res, err := svc.EarnCoin(context.Background(), "user1", uuid.New(), uuid.New(), true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Earned {
		t.Error("expected earned = false for duplicate")
	}
	if res.Reason != ReasonDuplicate {
		t.Errorf("Reason = %q, want %q", res.Reason, ReasonDuplicate)
	}
	if res.LeveledUp {
		t.Error("expected leveled_up = false for duplicate")
	}
}

// TestEarnCoin_AllLevelTransitions checks every level-up boundary:
// Thresholds: [0, 50, 200, 500, 1000]
func TestEarnCoin_AllLevelTransitions(t *testing.T) {
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
				coins:      UserCoins{Coins: tt.before},
				earnResult: true,
				earnTotal:  tt.after,
			}, 0)
			res, err := svc.EarnCoin(context.Background(), "user1", uuid.New(), uuid.New(), false)
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
			if res.TotalCoins != tt.after {
				t.Errorf("TotalCoins = %d, want %d", res.TotalCoins, tt.after)
			}
		})
	}
}

// TestEarnCoin_AtMaxLevel verifies that earning at Lv5 records the coin
// but does not set LeveledUp (already max).
func TestEarnCoin_AtMaxLevel(t *testing.T) {
	svc := NewService(&mockRepo{
		coins:      UserCoins{Coins: 1050}, // already Lv5
		earnResult: true,
		earnTotal:  1051,
	}, 0)
	res, err := svc.EarnCoin(context.Background(), "user1", uuid.New(), uuid.New(), true)
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
	if res.CoinsToNext != 0 {
		t.Errorf("CoinsToNext = %d, want 0 at max level", res.CoinsToNext)
	}
}

// TestEarnCoin_ProgressCalc verifies CurrentProgress and CoinsToNext
// are calculated correctly within a level.
func TestEarnCoin_ProgressCalc(t *testing.T) {
	// Lv2: 50~199코인. At 100코인 → progress=50 (100-50), needed=150 (200-50)
	svc := NewService(&mockRepo{
		coins:      UserCoins{Coins: 99},
		earnResult: true,
		earnTotal:  100,
	}, 0)
	res, err := svc.EarnCoin(context.Background(), "user1", uuid.New(), uuid.New(), true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Level != 2 {
		t.Errorf("Level = %d, want 2", res.Level)
	}
	if res.CurrentProgress != 50 {
		t.Errorf("CurrentProgress = %d, want 50 (100-50)", res.CurrentProgress)
	}
	if res.CoinsToNext != 150 {
		t.Errorf("CoinsToNext = %d, want 150 (200-50)", res.CoinsToNext)
	}
}

// TestEarnCoin_DailyLimit_Blocked: 일일 한도 도달 시 코인 적립 차단
func TestEarnCoin_DailyLimit_Blocked(t *testing.T) {
	svc := NewService(&mockRepo{
		coins:       UserCoins{Coins: 50},
		todayEarned: 10, // already earned 10 today
		earnResult:  true,
		earnTotal:   55,
	}, 10) // limit = 10
	res, err := svc.EarnCoin(context.Background(), "user1", uuid.New(), uuid.New(), true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Earned {
		t.Error("expected earned = false when daily limit reached")
	}
	if res.Reason != ReasonDailyLimit {
		t.Errorf("Reason = %q, want %q", res.Reason, ReasonDailyLimit)
	}
	// Level info should still be returned
	if res.Level != 2 {
		t.Errorf("Level = %d, want 2 (50코인 = Lv2)", res.Level)
	}
}

// TestEarnCoin_DailyLimit_UnderLimit: 한도 미달 시 정상 적립
func TestEarnCoin_DailyLimit_UnderLimit(t *testing.T) {
	svc := NewService(&mockRepo{
		coins:       UserCoins{Coins: 50},
		todayEarned: 5, // earned 5 today, limit is 10 → still allowed
		earnResult:  true,
		earnTotal:   55,
	}, 10)
	res, err := svc.EarnCoin(context.Background(), "user1", uuid.New(), uuid.New(), true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Earned {
		t.Error("expected earned = true when under daily limit")
	}
	if res.Reason != ReasonEarned {
		t.Errorf("Reason = %q, want %q", res.Reason, ReasonEarned)
	}
}

// TestEarnCoin_DailyLimit_Zero_Unlimited: limit=0이면 무제한
func TestEarnCoin_DailyLimit_Zero_Unlimited(t *testing.T) {
	svc := NewService(&mockRepo{
		coins:       UserCoins{Coins: 50},
		todayEarned: 9999, // huge amount but limit=0 → unlimited
		earnResult:  true,
		earnTotal:   55,
	}, 0)
	res, err := svc.EarnCoin(context.Background(), "user1", uuid.New(), uuid.New(), true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Earned {
		t.Error("expected earned = true when limit is 0 (unlimited)")
	}
}
