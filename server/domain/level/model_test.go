package level

import "testing"

func TestCalcLevel(t *testing.T) {
	// Thresholds: [0, 15, 45, 90, 165]
	tests := []struct {
		points int
		want   int
	}{
		{0, 1}, {14, 1},
		{15, 2}, {44, 2},
		{45, 3}, {89, 3},
		{90, 4}, {164, 4},
		{165, 5}, {200, 5},
	}
	for _, tt := range tests {
		got := CalcLevel(tt.points)
		if got != tt.want {
			t.Errorf("CalcLevel(%d) = %d, want %d", tt.points, got, tt.want)
		}
	}
}

func TestCalcLevelInfo_Mid(t *testing.T) {
	// 22pt → Lv2 (15-44), start=15, end=45, progress=7 (22-15), needed=30 (45-15)
	info := CalcLevelInfo(22)
	if info.Level != 2 {
		t.Errorf("Level = %d, want 2", info.Level)
	}
	if info.CurrentProgress != 7 {
		t.Errorf("CurrentProgress = %d, want 7 (22-15)", info.CurrentProgress)
	}
	if info.PointsToNext != 30 {
		t.Errorf("PointsToNext = %d, want 30 (45-15)", info.PointsToNext)
	}
}

func TestCalcLevelInfo_MaxLevel(t *testing.T) {
	info := CalcLevelInfo(165)
	if info.Level != 5 {
		t.Errorf("Level = %d, want 5", info.Level)
	}
	if info.PointsToNext != 0 {
		t.Errorf("PointsToNext = %d, want 0 at max level", info.PointsToNext)
	}
}

func TestCalcLevelInfo_Boundary(t *testing.T) {
	// Exactly at level 2 threshold
	info := CalcLevelInfo(15)
	if info.Level != 2 {
		t.Errorf("Level = %d, want 2 at exactly 15 points", info.Level)
	}
	if info.CurrentProgress != 0 {
		t.Errorf("CurrentProgress = %d, want 0 (just entered lv2)", info.CurrentProgress)
	}
}

func TestCalcPoints(t *testing.T) {
	tests := []struct {
		preferred bool
		days      int
		want      int
	}{
		{true, 0, 5},   // preferred base only
		{false, 0, 15}, // non-preferred base only
		{true, 1, 10},  // preferred + 1 day bonus (5 + 1*5)
		{false, 1, 20}, // non-preferred + 1 day bonus (15 + 1*5)
		{true, 3, 20},  // preferred + 3 day bonus (5 + 3*5)
		{false, 3, 30}, // non-preferred + 3 day bonus (15 + 3*5)
	}
	for _, tt := range tests {
		got := CalcPoints(tt.preferred, tt.days)
		if got != tt.want {
			t.Errorf("CalcPoints(preferred=%v, days=%d) = %d, want %d", tt.preferred, tt.days, got, tt.want)
		}
	}
}
