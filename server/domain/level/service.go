package level

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// GetLevel returns the current level info for a user.
func (s *Service) GetLevel(ctx context.Context, userID string) (LevelInfo, error) {
	uc, err := s.repo.GetUserCoins(ctx, userID)
	if err != nil {
		return LevelInfo{}, fmt.Errorf("get level: %w", err)
	}
	return CalcLevelInfo(uc.Coins), nil
}

// SetCoins directly overwrites a user's coins and recalculates level. For testing only.
func (s *Service) SetCoins(ctx context.Context, userID string, coins int) (LevelInfo, error) {
	if err := s.repo.SetCoins(ctx, userID, coins); err != nil {
		return LevelInfo{}, fmt.Errorf("set coins: %w", err)
	}
	return CalcLevelInfo(coins), nil
}

// EarnCoin awards coins for visiting a topic.
// preferred=true if the topic's category is in the user's subscriptions (or is top/brief).
func (s *Service) EarnCoin(ctx context.Context, userID string, runID, contextItemID uuid.UUID, preferred bool) (EarnResult, error) {
	coins := CalcCoins(preferred)

	before, err := s.repo.GetUserCoins(ctx, userID)
	if err != nil {
		return EarnResult{}, fmt.Errorf("get coins before earn: %w", err)
	}
	oldLevel := CalcLevel(before.Coins)

	earned, newTotal, err := s.repo.EarnCoin(ctx, userID, runID, contextItemID, coins)
	if err != nil {
		return EarnResult{}, fmt.Errorf("earn coin: %w", err)
	}

	if !earned {
		info := CalcLevelInfo(before.Coins)
		return EarnResult{
			Earned:          false,
			Level:           info.Level,
			TotalCoins:      info.TotalCoins,
			CurrentProgress: info.CurrentProgress,
			CoinsToNext:     info.CoinsToNext,
			LeveledUp:       false,
		}, nil
	}

	info := CalcLevelInfo(newTotal)
	return EarnResult{
		Earned:          true,
		Level:           info.Level,
		TotalCoins:      info.TotalCoins,
		CurrentProgress: info.CurrentProgress,
		CoinsToNext:     info.CoinsToNext,
		LeveledUp:       info.Level > oldLevel,
		CoinsEarned:     coins,
	}, nil
}

// DecayAllCoins runs the daily coin decay: -1 per user (min 0), batch size 1000.
func (s *Service) DecayAllCoins(ctx context.Context) (int, error) {
	affected, err := s.repo.DecayCoins(ctx, 1000)
	if err != nil {
		return 0, fmt.Errorf("decay all coins: %w", err)
	}
	return affected, nil
}
