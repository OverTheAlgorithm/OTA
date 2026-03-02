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

// EarnPoint awards points for visiting a topic.
// preferred=true if the topic's category is in the user's subscriptions (or is top/brief).
func (s *Service) EarnPoint(ctx context.Context, userID string, runID, contextItemID uuid.UUID, preferred bool) (EarnResult, error) {
	points := CalcPoints(preferred)

	before, err := s.repo.GetUserPoints(ctx, userID)
	if err != nil {
		return EarnResult{}, fmt.Errorf("get points before earn: %w", err)
	}
	oldLevel := CalcLevel(before.Points)

	earned, newTotal, err := s.repo.EarnPoint(ctx, userID, runID, contextItemID, points)
	if err != nil {
		return EarnResult{}, fmt.Errorf("earn point: %w", err)
	}

	if !earned {
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
		PointsEarned:    points,
	}, nil
}

// DecayAllPoints runs the daily point decay: -1pt per user (min 0), batch size 1000.
func (s *Service) DecayAllPoints(ctx context.Context) (int, error) {
	affected, err := s.repo.DecayPoints(ctx, 1000)
	if err != nil {
		return 0, fmt.Errorf("decay all points: %w", err)
	}
	return affected, nil
}
