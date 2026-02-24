package level

import (
	"time"

	"github.com/google/uuid"
)

// Thresholds는 레벨별 누적 포인트 기준 (인덱스 = 레벨-1)
// Lv1: 0pt~, Lv2: 5pt~, Lv3: 15pt~, Lv4: 30pt~, Lv5: 55pt~
var Thresholds = []int{0, 5, 15, 30, 55}

// Descriptions는 레벨별 설명
var Descriptions = []string{
	"알고리즘 속에 갇혀 있어요",
	"구름 너머를 엿보기 시작했어요",
	"세상의 맥락이 보이기 시작해요",
	"알고리즘을 넘어서고 있어요",
	"알고리즘을 완전히 넘어섰어요!",
}

const MaxLevel = 5

// BrainCategoryKey is the key for the "Over the Algorithm" brain category
const BrainCategoryKey = "over_the_algorithm"

type UserPoints struct {
	UserID    string
	Level     int
	Points    int
	CreatedAt time.Time
	UpdatedAt time.Time
}

type PointLog struct {
	ID            uuid.UUID
	UserID        string
	ContextItemID uuid.UUID
	PointsEarned  int
	CreatedAt     time.Time
}

type LevelInfo struct {
	Level           int    `json:"level"`
	TotalPoints     int    `json:"total_points"`
	CurrentProgress int    `json:"current_progress"`
	PointsToNext    int    `json:"points_to_next"`
	Description     string `json:"description"`
}

type EarnResult struct {
	Earned          bool `json:"earned"`
	Level           int  `json:"level"`
	TotalPoints     int  `json:"total_points"`
	CurrentProgress int  `json:"current_progress"`
	PointsToNext    int  `json:"points_to_next"`
	LeveledUp       bool `json:"leveled_up"`
}

// CalcLevel returns the level (1-5) for the given total accumulated points.
func CalcLevel(totalPoints int) int {
	lv := 1
	for i := 1; i < len(Thresholds); i++ {
		if totalPoints >= Thresholds[i] {
			lv = i + 1
		}
	}
	return lv
}

// CalcLevelInfo returns full level info for the given total accumulated points.
func CalcLevelInfo(totalPoints int) LevelInfo {
	lv := CalcLevel(totalPoints)
	desc := Descriptions[lv-1]

	if lv >= MaxLevel {
		return LevelInfo{
			Level:           lv,
			TotalPoints:     totalPoints,
			CurrentProgress: 0,
			PointsToNext:    0,
			Description:     desc,
		}
	}

	start := Thresholds[lv-1]
	end := Thresholds[lv]
	progress := totalPoints - start
	needed := end - start

	return LevelInfo{
		Level:           lv,
		TotalPoints:     totalPoints,
		CurrentProgress: progress,
		PointsToNext:    needed,
		Description:     desc,
	}
}
