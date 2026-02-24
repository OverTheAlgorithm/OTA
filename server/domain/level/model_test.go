package level

import "testing"

func TestCalcLevel(t *testing.T) {
	tests := []struct {
		points int
		want   int
	}{
		{0, 1}, {4, 1},
		{5, 2}, {14, 2},
		{15, 3}, {29, 3},
		{30, 4}, {54, 4},
		{55, 5}, {100, 5},
	}
	for _, tt := range tests {
		got := CalcLevel(tt.points)
		if got != tt.want {
			t.Errorf("CalcLevel(%d) = %d, want %d", tt.points, got, tt.want)
		}
	}
}

func TestCalcLevelInfo_Mid(t *testing.T) {
	info := CalcLevelInfo(22)
	if info.Level != 3 {
		t.Errorf("Level = %d, want 3", info.Level)
	}
	if info.CurrentProgress != 7 {
		t.Errorf("CurrentProgress = %d, want 7 (22-15)", info.CurrentProgress)
	}
	if info.PointsToNext != 15 {
		t.Errorf("PointsToNext = %d, want 15 (30-15)", info.PointsToNext)
	}
}

func TestCalcLevelInfo_MaxLevel(t *testing.T) {
	info := CalcLevelInfo(55)
	if info.Level != 5 {
		t.Errorf("Level = %d, want 5", info.Level)
	}
	if info.PointsToNext != 0 {
		t.Errorf("PointsToNext = %d, want 0 at max level", info.PointsToNext)
	}
}

func TestCalcLevelInfo_Boundary(t *testing.T) {
	// Exactly at level 2 threshold
	info := CalcLevelInfo(5)
	if info.Level != 2 {
		t.Errorf("Level = %d, want 2 at exactly 5 points", info.Level)
	}
	if info.CurrentProgress != 0 {
		t.Errorf("CurrentProgress = %d, want 0 (just entered lv2)", info.CurrentProgress)
	}
}
