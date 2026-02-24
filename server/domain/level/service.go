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
	up, err := s.repo.GetUserPoints(ctx, userID)
	if err != nil {
		return LevelInfo{}, fmt.Errorf("get level: %w", err)
	}
	return CalcLevelInfo(up.Points), nil
}

// SetPoints directly overwrites a user's points and recalculates level. For testing only.
func (s *Service) SetPoints(ctx context.Context, userID string, points int) (LevelInfo, error) {
	if err := s.repo.SetPoints(ctx, userID, points); err != nil {
		return LevelInfo{}, fmt.Errorf("set points: %w", err)
	}
	return CalcLevelInfo(points), nil
}

// EarnPoint awards a point if the context item belongs to the OTA brain category
// and hasn't been earned by this user yet.
func (s *Service) EarnPoint(ctx context.Context, userID string, contextItemID uuid.UUID) (EarnResult, error) {
	// Check brain category
	bc, err := s.repo.GetBrainCategory(ctx, contextItemID)
	if err != nil {
		return EarnResult{}, fmt.Errorf("check brain category: %w", err)
	}
	if bc != BrainCategoryKey {
		// Not an OTA category topic — no points, no error
		return EarnResult{Earned: false}, nil
	}

	// Capture level before earning
	before, err := s.repo.GetUserPoints(ctx, userID)
	if err != nil {
		return EarnResult{}, fmt.Errorf("get points before earn: %w", err)
	}
	oldLevel := CalcLevel(before.Points)

	// Attempt to earn
	earned, newTotal, err := s.repo.EarnPoint(ctx, userID, contextItemID)
	if err != nil {
		return EarnResult{}, fmt.Errorf("earn point: %w", err)
	}

	if !earned {
		// Duplicate — return current state
		info := CalcLevelInfo(before.Points)
		return EarnResult{
			Earned:          false,
			Level:           info.Level,
			TotalPoints:     info.TotalPoints,
			CurrentProgress: info.CurrentProgress,
			PointsToNext:    info.PointsToNext,
			LeveledUp:       false,
		}, nil
	}

	info := CalcLevelInfo(newTotal)
	return EarnResult{
		Earned:          true,
		Level:           info.Level,
		TotalPoints:     info.TotalPoints,
		CurrentProgress: info.CurrentProgress,
		PointsToNext:    info.PointsToNext,
		LeveledUp:       info.Level > oldLevel,
	}, nil
}
