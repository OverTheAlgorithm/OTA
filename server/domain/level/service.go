package level

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// kstLocation is KST (UTC+9).
var kstLocation = time.FixedZone("KST", 9*60*60)

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
	return s.EarnPointWithOverride(ctx, userID, runID, contextItemID, preferred, 0)
}

// EarnPointWithOverride awards points with an optional pre-calculated override.
// If overridePts > 0, uses that value instead of recalculating (for email link consistency).
func (s *Service) EarnPointWithOverride(ctx context.Context, userID string, runID, contextItemID uuid.UUID, preferred bool, overridePts int) (EarnResult, error) {
	var points int
	if overridePts > 0 {
		points = overridePts
	} else {
		daysSince, err := s.getDaysSinceLastEarn(ctx, userID)
		if err != nil {
			return EarnResult{}, fmt.Errorf("get last earned at: %w", err)
		}
		points = CalcPoints(preferred, daysSince)
	}

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

// GetLastEarnedAtBatch returns the most recent earn time for multiple users.
func (s *Service) GetLastEarnedAtBatch(ctx context.Context, userIDs []string) (map[string]time.Time, error) {
	return s.repo.GetLastEarnedAtBatch(ctx, userIDs)
}

// DecayAllPoints runs the daily point decay: -1pt per user (min 0), batch size 1000.
func (s *Service) DecayAllPoints(ctx context.Context) (int, error) {
	affected, err := s.repo.DecayPoints(ctx, 1000)
	if err != nil {
		return 0, fmt.Errorf("decay all points: %w", err)
	}
	return affected, nil
}

// getDaysSinceLastEarn returns the number of KST calendar days since the user last earned.
// Returns 0 if never earned.
func (s *Service) getDaysSinceLastEarn(ctx context.Context, userID string) (int, error) {
	lastEarned, ok, err := s.repo.GetLastEarnedAt(ctx, userID)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, nil
	}
	return calcDaysSinceKST(lastEarned), nil
}

// calcDaysSinceKST returns the number of calendar days (KST) between lastEarned and now.
func calcDaysSinceKST(lastEarned time.Time) int {
	now := time.Now().In(kstLocation)
	last := lastEarned.In(kstLocation)
	nowDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, kstLocation)
	lastDate := time.Date(last.Year(), last.Month(), last.Day(), 0, 0, 0, 0, kstLocation)
	days := int(nowDate.Sub(lastDate).Hours() / 24)
	if days < 0 {
		return 0
	}
	return days
}
