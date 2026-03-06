package level

import "testing"

func TestNewLevelConfig(t *testing.T) {
	lc := NewLevelConfig(5000, 1000)
	if lc.MaxLevel != 5 {
		t.Errorf("MaxLevel = %d, want 5", lc.MaxLevel)
	}
	wantThresholds := []int{0, 1000, 2000, 3000, 4000}
	if len(lc.Thresholds) != len(wantThresholds) {
		t.Fatalf("Thresholds len = %d, want %d", len(lc.Thresholds), len(wantThresholds))
	}
	for i, v := range wantThresholds {
		if lc.Thresholds[i] != v {
			t.Errorf("Thresholds[%d] = %d, want %d", i, lc.Thresholds[i], v)
		}
	}
}

func TestCalcLevel(t *testing.T) {
	lc := NewLevelConfig(5000, 1000)
	tests := []struct {
		coins int
		want  int
	}{
		{0, 1}, {999, 1},
		{1000, 2}, {1999, 2},
		{2000, 3}, {2999, 3},
		{3000, 4}, {3999, 4},
		{4000, 5}, {5000, 5},
	}
	for _, tt := range tests {
		got := lc.CalcLevel(tt.coins)
		if got != tt.want {
			t.Errorf("CalcLevel(%d) = %d, want %d", tt.coins, got, tt.want)
		}
	}
}

func TestCalcLevelInfo(t *testing.T) {
	lc := NewLevelConfig(5000, 1000)
	// 1500 coins → Lv2, base=10, extra=10 → daily limit = 10 + 2*10 = 30
	info := CalcLevelInfo(1500, lc, 10, 10)
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

func TestCalcLevelInfo_MaxLevel(t *testing.T) {
	lc := NewLevelConfig(5000, 1000)
	info := CalcLevelInfo(4000, lc, 10, 10)
	if info.Level != 5 {
		t.Errorf("Level = %d, want 5", info.Level)
	}
	if info.DailyLimit != 60 {
		t.Errorf("DailyLimit = %d, want 60 (10 + 5*10)", info.DailyLimit)
	}
}

func TestCalcDailyLimit(t *testing.T) {
	tests := []struct {
		level, base, extra, want int
	}{
		{1, 10, 10, 20},
		{3, 10, 10, 40},
		{5, 10, 5, 35},
		{1, 0, 10, 0}, // base=0 → unlimited
	}
	for _, tt := range tests {
		got := CalcDailyLimit(tt.level, tt.base, tt.extra)
		if got != tt.want {
			t.Errorf("CalcDailyLimit(%d, %d, %d) = %d, want %d", tt.level, tt.base, tt.extra, got, tt.want)
		}
	}
}

func TestCalcCoins(t *testing.T) {
	tests := []struct {
		preferred bool
		want      int
	}{
		{true, 5},   // preferred
		{false, 10}, // non-preferred
	}
	for _, tt := range tests {
		got := CalcCoins(tt.preferred)
		if got != tt.want {
			t.Errorf("CalcCoins(preferred=%v) = %d, want %d", tt.preferred, got, tt.want)
		}
	}
}
