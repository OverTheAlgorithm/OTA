package level

import (
	"time"

	"github.com/google/uuid"
)

// Thresholds는 레벨별 누적 코인 기준 (인덱스 = 레벨-1)
// Lv1: 0~, Lv2: 50~, Lv3: 200~, Lv4: 500~, Lv5: 1000~
var Thresholds = []int{0, 50, 200, 500, 1000}

// Descriptions는 레벨별 설명
var Descriptions = []string{
	"알고리즘 속에 갇혀 있어요",
	"구름 너머를 엿보기 시작했어요",
	"세상의 맥락이 보이기 시작해요",
	"알고리즘을 넘어서고 있어요",
	"알고리즘을 완전히 넘어섰어요!",
}

const MaxLevel = 5

// Coin constants for earn calculation.
const (
	BaseCoinPreferred    = 5  // coins for visiting a topic in a subscribed category
	BaseCoinNonPreferred = 10 // coins for visiting a topic outside subscribed categories
)

// EarnResult reason constants.
const (
	ReasonEarned     = "EARNED"
	ReasonDuplicate  = "DUPLICATE"
	ReasonDailyLimit = "DAILY_LIMIT"
)

type UserCoins struct {
	UserID    string
	Coins     int
	CreatedAt time.Time
	UpdatedAt time.Time
}

type CoinLog struct {
	ID            uuid.UUID
	UserID        string
	RunID         uuid.UUID
	ContextItemID uuid.UUID
	CoinsEarned   int
	CreatedAt     time.Time
}

type LevelInfo struct {
	Level           int    `json:"level"`
	TotalCoins      int    `json:"total_coins"`
	CurrentProgress int    `json:"current_progress"`
	CoinsToNext     int    `json:"coins_to_next"`
	Description     string `json:"description"`
}

type EarnResult struct {
	Earned          bool   `json:"earned"`
	Reason          string `json:"reason"`
	Level           int    `json:"level"`
	TotalCoins      int    `json:"total_coins"`
	CurrentProgress int    `json:"current_progress"`
	CoinsToNext     int    `json:"coins_to_next"`
	LeveledUp       bool   `json:"leveled_up"`
	CoinsEarned     int    `json:"coins_earned"`
}

// CalcLevel returns the level (1-5) for the given total accumulated coins.
func CalcLevel(totalCoins int) int {
	lv := 1
	for i := 1; i < len(Thresholds); i++ {
		if totalCoins >= Thresholds[i] {
			lv = i + 1
		}
	}
	return lv
}

// CalcLevelInfo returns full level info for the given total accumulated coins.
func CalcLevelInfo(totalCoins int) LevelInfo {
	lv := CalcLevel(totalCoins)
	desc := Descriptions[lv-1]

	if lv >= MaxLevel {
		return LevelInfo{
			Level:           lv,
			TotalCoins:      totalCoins,
			CurrentProgress: 0,
			CoinsToNext:     0,
			Description:     desc,
		}
	}

	start := Thresholds[lv-1]
	end := Thresholds[lv]
	progress := totalCoins - start
	needed := end - start

	return LevelInfo{
		Level:           lv,
		TotalCoins:      totalCoins,
		CurrentProgress: progress,
		CoinsToNext:     needed,
		Description:     desc,
	}
}

// CalcCoins returns the coins to award for visiting a topic.
// preferred=true means the topic belongs to a category the user subscribes to.
func CalcCoins(preferred bool) int {
	if preferred {
		return BaseCoinPreferred
	}
	return BaseCoinNonPreferred
}

// IsPreferredCategory returns true if the category is always shown (top/brief) or is in the user's subscriptions.
func IsPreferredCategory(category string, subscriptions []string) bool {
	if category == "top" || category == "brief" {
		return true
	}
	for _, sub := range subscriptions {
		if category == sub {
			return true
		}
	}
	return false
}
