package level

import "testing"

func TestCalcLevel(t *testing.T) {
	// Thresholds: [0, 50, 200, 500, 1000]
	tests := []struct {
		coins int
		want  int
	}{
		{0, 1}, {49, 1},
		{50, 2}, {199, 2},
		{200, 3}, {499, 3},
		{500, 4}, {999, 4},
		{1000, 5}, {2000, 5},
	}
	for _, tt := range tests {
		got := CalcLevel(tt.coins)
		if got != tt.want {
			t.Errorf("CalcLevel(%d) = %d, want %d", tt.coins, got, tt.want)
		}
	}
}

func TestCalcLevelInfo_Mid(t *testing.T) {
	// 100코인 → Lv2 (50-199), start=50, end=200, progress=50 (100-50), needed=150 (200-50)
	info := CalcLevelInfo(100)
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

func TestCalcLevelInfo_MaxLevel(t *testing.T) {
	info := CalcLevelInfo(1000)
	if info.Level != 5 {
		t.Errorf("Level = %d, want 5", info.Level)
	}
	if info.CoinsToNext != 0 {
		t.Errorf("CoinsToNext = %d, want 0 at max level", info.CoinsToNext)
	}
}

func TestCalcLevelInfo_Boundary(t *testing.T) {
	// Exactly at level 2 threshold
	info := CalcLevelInfo(50)
	if info.Level != 2 {
		t.Errorf("Level = %d, want 2 at exactly 50 coins", info.Level)
	}
	if info.CurrentProgress != 0 {
		t.Errorf("CurrentProgress = %d, want 0 (just entered lv2)", info.CurrentProgress)
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
