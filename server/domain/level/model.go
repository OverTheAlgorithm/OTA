package level

import (
	"time"

	"github.com/google/uuid"
)

// LevelConfig holds dynamic level thresholds computed from environment config.
type LevelConfig struct {
	CoinCap       int   // maximum coins a user can hold
	CoinsPerLevel int   // coins per level transition
	MaxLevel      int   // derived: CoinCap / CoinsPerLevel
	Thresholds    []int // derived: [0, coinsPerLevel, 2*coinsPerLevel, ...]
}

// NewLevelConfig creates a LevelConfig from coinCap and coinsPerLevel.
// Example: coinCap=5000, coinsPerLevel=1000 → MaxLevel=5, Thresholds=[0,1000,2000,3000,4000]
func NewLevelConfig(coinCap, coinsPerLevel int) LevelConfig {
	maxLevel := coinCap / coinsPerLevel
	thresholds := make([]int, maxLevel)
	for i := range thresholds {
		thresholds[i] = i * coinsPerLevel
	}
	return LevelConfig{
		CoinCap:       coinCap,
		CoinsPerLevel: coinsPerLevel,
		MaxLevel:      maxLevel,
		Thresholds:    thresholds,
	}
}

// CalcLevel returns the level (1..MaxLevel) for the given total coins.
func (lc LevelConfig) CalcLevel(totalCoins int) int {
	lv := 1
	for i := 1; i < len(lc.Thresholds); i++ {
		if totalCoins >= lc.Thresholds[i] {
			lv = i + 1
		}
	}
	return lv
}

// Descriptions are level descriptions indexed by level-1.
// When MaxLevel exceeds len(Descriptions), the last entry is reused.
var Descriptions = []string{
	"알고리즘 속에 갇혀 있어요",
	"구름 너머를 엿보기 시작했어요",
	"세상의 맥락이 보이기 시작해요",
	"알고리즘을 넘어서고 있어요",
	"알고리즘을 완전히 넘어섰어요!",
}

// Description returns the description string for the given level.
func (lc LevelConfig) Description(lv int) string {
	idx := lv - 1
	if idx >= len(Descriptions) {
		idx = len(Descriptions) - 1
	}
	return Descriptions[idx]
}

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

// CoinEvent represents a non-topic coin balance change (signup bonus, promotion, etc.).
type CoinEvent struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Amount    int       `json:"amount"`
	Type      string    `json:"type"`
	Memo      string    `json:"memo"`
	CreatedAt time.Time `json:"created_at"`
}

// CoinTransaction is a unified view combining coin_logs + coin_events + withdrawals.
type CoinTransaction struct {
	ID          string    `json:"id"`
	Amount      int       `json:"amount"`
	Type        string    `json:"type"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

type LevelInfo struct {
	Level       int    `json:"level"`
	TotalCoins  int    `json:"total_coins"`
	DailyLimit  int    `json:"daily_limit"`
	CoinCap     int    `json:"coin_cap"`
	Thresholds  []int  `json:"thresholds"`
	Description string `json:"description"`
}

type EarnResult struct {
	Earned      bool   `json:"earned"`
	Reason      string `json:"reason"`
	Level       int    `json:"level"`
	TotalCoins  int    `json:"total_coins"`
	DailyLimit  int    `json:"daily_limit"`
	LeveledUp   bool   `json:"leveled_up"`
	CoinsEarned int    `json:"coins_earned"`
}

// CalcDailyLimit returns the daily coin earning limit for the given level.
// Formula: baseDailyLimit + (level * extraPerLevel).
// Returns 0 (unlimited) when baseDailyLimit is 0.
func CalcDailyLimit(level, baseDailyLimit, extraPerLevel int) int {
	if baseDailyLimit == 0 {
		return 0
	}
	return baseDailyLimit + (level * extraPerLevel)
}

// CalcLevelInfo returns full level info using the given LevelConfig.
func CalcLevelInfo(totalCoins int, lc LevelConfig, baseDailyLimit, extraPerLevel int) LevelInfo {
	lv := lc.CalcLevel(totalCoins)
	return LevelInfo{
		Level:       lv,
		TotalCoins:  totalCoins,
		DailyLimit:  CalcDailyLimit(lv, baseDailyLimit, extraPerLevel),
		CoinCap:     lc.CoinCap,
		Thresholds:  lc.Thresholds,
		Description: lc.Description(lv),
	}
}

// CalcCoins returns the coins to award for visiting a topic.
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
