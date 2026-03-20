package level

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

// testCfg uses coinCap=5000, coinsPerLevel=1000 → thresholds [0,1000,2000,3000,4000], maxLevel=5
var testCfg = NewLevelConfig(5000, 1000)

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

func (m *mockRepo) SetCoins(_ context.Context, _ string, _ int, _ string) error {
	return nil
}

func (m *mockRepo) GetTodayEarnedCoins(_ context.Context, _ string) (int, error) {
	return m.todayEarned, nil
}

func (m *mockRepo) HasEarned(_ context.Context, _ string, _, _ uuid.UUID) (bool, error) {
	return false, nil
}

func (m *mockRepo) DeductCoins(_ context.Context, _ string, _ int) error {
	return nil
}

func (m *mockRepo) RestoreCoins(_ context.Context, _ string, _ int) error {
	return nil
}

func (m *mockRepo) InsertCoinEvent(_ context.Context, _ string, _ int, _, _, _ string) error {
	return nil
}

func (m *mockRepo) GetCoinHistory(_ context.Context, _ string, _, _ int) ([]CoinTransaction, error) {
	return nil, nil
}

func (m *mockRepo) GetEarnedItemIDs(_ context.Context, _ string, _ []uuid.UUID) ([]uuid.UUID, error) {
	return nil, nil
}

func TestService_GetLevel(t *testing.T) {
	// 1500 coins → Lv2 (threshold 1000), base=10, extra=10 → dailyLimit = 10 + 2*10 = 30
	svc := NewService(&mockRepo{coins: UserCoins{Coins: 1500}}, testCfg, 10, 10)
	info, err := svc.GetLevel(context.Background(), "user1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Level != 2 {
		t.Errorf("Level = %d, want 2", info.Level)
	}
	if info.TotalCoins != 1500 {
		t.Errorf("TotalCoins = %d, want 1500", info.TotalCoins)
	}
	if info.DailyLimit != 30 {
		t.Errorf("DailyLimit = %d, want 30", info.DailyLimit)
	}
	if info.CoinCap != 5000 {
		t.Errorf("CoinCap = %d, want 5000", info.CoinCap)
	}
}

func TestService_EarnCoin_Duplicate(t *testing.T) {
	svc := NewService(&mockRepo{
		coins:      UserCoins{Coins: 500},
		earnResult: false,
		earnTotal:  0,
	}, testCfg, 0, 0)
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
}

func TestEarnCoin_AllLevelTransitions(t *testing.T) {
	transitions := []struct {
		name      string
		before    int
		after     int
		wantLevel int
		wantLevUp bool
	}{
		{"Lv1→Lv2", 999, 1000, 2, true},
		{"Lv2→Lv3", 1999, 2000, 3, true},
		{"Lv3→Lv4", 2999, 3000, 4, true},
		{"Lv4→Lv5", 3999, 4000, 5, true},
		{"Lv2 no up", 1500, 1501, 2, false},
		{"Lv3 no up", 2500, 2501, 3, false},
	}

	for _, tt := range transitions {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewService(&mockRepo{
				coins:      UserCoins{Coins: tt.before},
				earnResult: true,
				earnTotal:  tt.after,
			}, testCfg, 0, 0)
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

func TestEarnCoin_AtMaxLevel(t *testing.T) {
	svc := NewService(&mockRepo{
		coins:      UserCoins{Coins: 4500},
		earnResult: true,
		earnTotal:  4501,
	}, testCfg, 0, 0)
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
	if res.Level != testCfg.MaxLevel {
		t.Errorf("Level = %d, want %d", res.Level, testCfg.MaxLevel)
	}
}

func TestEarnCoin_DailyLimit(t *testing.T) {
	// Lv2 user (1000 coins), base=10, extra=5 → limit = 10 + 2*5 = 20
	cases := []struct {
		name        string
		todayEarned int
		base, extra int
		wantEarned  bool
		wantReason  string
	}{
		{"at limit → blocked", 20, 10, 5, false, ReasonDailyLimit},
		{"under limit → allowed", 5, 10, 5, true, ReasonEarned},
		{"base=0 → unlimited", 9999, 0, 10, true, ReasonEarned},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewService(&mockRepo{
				coins:       UserCoins{Coins: 1000},
				todayEarned: tt.todayEarned,
				earnResult:  true,
				earnTotal:   1005,
			}, testCfg, tt.base, tt.extra)
			res, err := svc.EarnCoin(context.Background(), "user1", uuid.New(), uuid.New(), true)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if res.Earned != tt.wantEarned {
				t.Errorf("Earned = %v, want %v", res.Earned, tt.wantEarned)
			}
			if res.Reason != tt.wantReason {
				t.Errorf("Reason = %q, want %q", res.Reason, tt.wantReason)
			}
		})
	}
}
