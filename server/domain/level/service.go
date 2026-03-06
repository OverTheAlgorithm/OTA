package level

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

type Service struct {
	repo               Repository
	levelCfg           LevelConfig
	baseDailyLimit     int
	extraLimitPerLevel int
}

func NewService(repo Repository, levelCfg LevelConfig, baseDailyLimit, extraLimitPerLevel int) *Service {
	return &Service{
		repo:               repo,
		levelCfg:           levelCfg,
		baseDailyLimit:     baseDailyLimit,
		extraLimitPerLevel: extraLimitPerLevel,
	}
}

// calcInfo is a shorthand that passes the service's config into CalcLevelInfo.
func (s *Service) calcInfo(totalCoins int) LevelInfo {
	return CalcLevelInfo(totalCoins, s.levelCfg, s.baseDailyLimit, s.extraLimitPerLevel)
}

// GetLevel returns the current level info for a user.
func (s *Service) GetLevel(ctx context.Context, userID string) (LevelInfo, error) {
	uc, err := s.repo.GetUserCoins(ctx, userID)
	if err != nil {
		return LevelInfo{}, fmt.Errorf("get level: %w", err)
	}
	return s.calcInfo(uc.Coins), nil
}

// SetCoins directly overwrites a user's coins and recalculates level. For testing only.
func (s *Service) SetCoins(ctx context.Context, userID string, coins int) (LevelInfo, error) {
	if err := s.repo.SetCoins(ctx, userID, coins); err != nil {
		return LevelInfo{}, fmt.Errorf("set coins: %w", err)
	}
	return s.calcInfo(coins), nil
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
func (s *Service) IsAtDailyLimit(ctx context.Context, userID string) (bool, error) {
	if s.baseDailyLimit == 0 {
		return false, nil
	}

	uc, err := s.repo.GetUserCoins(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("is at daily limit: %w", err)
	}

	lv := s.levelCfg.CalcLevel(uc.Coins)
	limit := CalcDailyLimit(lv, s.baseDailyLimit, s.extraLimitPerLevel)

	todayEarned, err := s.repo.GetTodayEarnedCoins(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("is at daily limit: %w", err)
	}
	return todayEarned >= limit, nil
}

// EarnCoin awards coins for visiting a topic.
func (s *Service) EarnCoin(ctx context.Context, userID string, runID, contextItemID uuid.UUID, preferred bool) (EarnResult, error) {
	coins := CalcCoins(preferred)

	before, err := s.repo.GetUserCoins(ctx, userID)
	if err != nil {
		return EarnResult{}, fmt.Errorf("get coins before earn: %w", err)
	}

	// Check level-based daily coin limit (0 = unlimited)
	if s.baseDailyLimit > 0 {
		lv := s.levelCfg.CalcLevel(before.Coins)
		limit := CalcDailyLimit(lv, s.baseDailyLimit, s.extraLimitPerLevel)

		todayEarned, err := s.repo.GetTodayEarnedCoins(ctx, userID)
		if err != nil {
			return EarnResult{}, fmt.Errorf("get today earned coins: %w", err)
		}
		if todayEarned >= limit {
			info := s.calcInfo(before.Coins)
			return EarnResult{
				Earned:     false,
				Reason:     ReasonDailyLimit,
				Level:      info.Level,
				TotalCoins: info.TotalCoins,
				DailyLimit: info.DailyLimit,
			}, nil
		}
	}

	oldLevel := s.levelCfg.CalcLevel(before.Coins)

	earned, newTotal, err := s.repo.EarnCoin(ctx, userID, runID, contextItemID, coins)
	if err != nil {
		return EarnResult{}, fmt.Errorf("earn coin: %w", err)
	}

	if !earned {
		info := s.calcInfo(before.Coins)
		return EarnResult{
			Earned:     false,
			Reason:     ReasonDuplicate,
			Level:      info.Level,
			TotalCoins: info.TotalCoins,
			DailyLimit: info.DailyLimit,
		}, nil
	}

	info := s.calcInfo(newTotal)
	return EarnResult{
		Earned:      true,
		Reason:      ReasonEarned,
		Level:       info.Level,
		TotalCoins:  info.TotalCoins,
		DailyLimit:  info.DailyLimit,
		LeveledUp:   info.Level > oldLevel,
		CoinsEarned: coins,
	}, nil
}
