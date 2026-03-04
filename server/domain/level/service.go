package level

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

type Service struct {
	repo           Repository
	dailyCoinLimit int
}

func NewService(repo Repository, dailyCoinLimit int) *Service {
	return &Service{repo: repo, dailyCoinLimit: dailyCoinLimit}
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

// HasEarned reports whether the user already has a coin_log entry for this run+item.
func (s *Service) HasEarned(ctx context.Context, userID string, runID, contextItemID uuid.UUID) (bool, error) {
	ok, err := s.repo.HasEarned(ctx, userID, runID, contextItemID)
	if err != nil {
		return false, fmt.Errorf("has earned: %w", err)
	}
	return ok, nil
}

// IsAtDailyLimit reports whether the user has reached today's coin earn limit.
// Returns false when the limit is 0 (unlimited).
func (s *Service) IsAtDailyLimit(ctx context.Context, userID string) (bool, error) {
	if s.dailyCoinLimit == 0 {
		return false, nil
	}
	todayEarned, err := s.repo.GetTodayEarnedCoins(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("is at daily limit: %w", err)
	}
	return todayEarned >= s.dailyCoinLimit, nil
}

// EarnCoin awards coins for visiting a topic.
// preferred=true if the topic's category is in the user's subscriptions (or is top/brief).
func (s *Service) EarnCoin(ctx context.Context, userID string, runID, contextItemID uuid.UUID, preferred bool) (EarnResult, error) {
	coins := CalcCoins(preferred)

	before, err := s.repo.GetUserCoins(ctx, userID)
	if err != nil {
		return EarnResult{}, fmt.Errorf("get coins before earn: %w", err)
	}

	// Check daily coin limit (0 = unlimited)
	if s.dailyCoinLimit > 0 {
		todayEarned, err := s.repo.GetTodayEarnedCoins(ctx, userID)
		if err != nil {
			return EarnResult{}, fmt.Errorf("get today earned coins: %w", err)
		}
		if todayEarned >= s.dailyCoinLimit {
			info := CalcLevelInfo(before.Coins)
			return EarnResult{
				Earned:          false,
				Reason:          ReasonDailyLimit,
				Level:           info.Level,
				TotalCoins:      info.TotalCoins,
				CurrentProgress: info.CurrentProgress,
				CoinsToNext:     info.CoinsToNext,
			}, nil
		}
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
			Reason:          ReasonDuplicate,
			Level:           info.Level,
			TotalCoins:      info.TotalCoins,
			CurrentProgress: info.CurrentProgress,
			CoinsToNext:     info.CoinsToNext,
		}, nil
	}

	info := CalcLevelInfo(newTotal)
	return EarnResult{
		Earned:          true,
		Reason:          ReasonEarned,
		Level:           info.Level,
		TotalCoins:      info.TotalCoins,
		CurrentProgress: info.CurrentProgress,
		CoinsToNext:     info.CoinsToNext,
		LeveledUp:       info.Level > oldLevel,
		CoinsEarned:     coins,
	}, nil
}
