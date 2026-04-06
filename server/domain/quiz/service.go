package quiz

import (
	"context"
	"errors"
	"fmt"
	"math/rand"

	"ota/domain/level"

	"github.com/google/uuid"
)

// Sentinel errors for quiz access control.
var (
	ErrNotEarned       = errors.New("user has not earned coins for this article")
	ErrAlreadyAttempted = errors.New("user has already attempted this quiz")
	ErrNotFound        = errors.New("no quiz found for this article")
)

// Service provides business logic for quiz operations.
type Service struct {
	repo      Repository
	levelRepo level.Repository
	levelCfg  level.LevelConfig
	maxBonus  int // QUIZ_MAX_BONUS_COINS
}

// NewService creates a new quiz Service.
func NewService(repo Repository, levelRepo level.Repository, levelCfg level.LevelConfig, maxBonus int) *Service {
	return &Service{
		repo:      repo,
		levelRepo: levelRepo,
		levelCfg:  levelCfg,
		maxBonus:  maxBonus,
	}
}

// GetQuizForUser returns quiz data for a user if all eligibility conditions are met:
//  1. User has earned coins for the article (coin_logs entry exists)
//  2. User has NOT already attempted the quiz
//  3. A quiz exists for the article
//
// Returns nil (no error) when no quiz exists for the article.
// Returns ErrNotEarned if the user has not earned coins for the article.
// Returns ErrAlreadyAttempted if the user already took the quiz.
func (s *Service) GetQuizForUser(ctx context.Context, userID string, contextItemID uuid.UUID) (*QuizForUser, error) {
	// Check earn-gate: user must have a coin_logs entry for this article.
	// Uses GetEarnedItemIDs (not HasEarned) because we don't have runID here.
	earned, err := s.levelRepo.GetEarnedItemIDs(ctx, userID, []uuid.UUID{contextItemID})
	if err != nil {
		return nil, fmt.Errorf("get quiz for user: check earn gate: %w", err)
	}
	if len(earned) == 0 {
		return nil, ErrNotEarned
	}

	// Fetch quiz (nil = no quiz for this article, not an error).
	quiz, err := s.repo.GetByContextItemID(ctx, contextItemID)
	if err != nil {
		return nil, fmt.Errorf("get quiz for user: fetch quiz: %w", err)
	}
	if quiz == nil {
		return nil, nil
	}

	// Check if user already attempted.
	attempted, err := s.repo.HasAttempted(ctx, userID, quiz.ID)
	if err != nil {
		return nil, fmt.Errorf("get quiz for user: check attempt: %w", err)
	}
	if attempted {
		return nil, ErrAlreadyAttempted
	}

	return &QuizForUser{
		ID:            quiz.ID,
		ContextItemID: quiz.ContextItemID,
		Question:      quiz.Question,
		Options:       quiz.Options,
	}, nil
}

// SubmitAnswer records the user's answer and awards bonus coins if correct.
// Quiz bonus coins go to coin_events (not coin_logs) — exempt from daily limit.
// Returns ErrNotEarned if earn-gate fails, ErrAlreadyAttempted on duplicate submission.
func (s *Service) SubmitAnswer(ctx context.Context, userID string, contextItemID uuid.UUID, answerIndex int, topicName string) (SubmitResult, error) {
	// Earn-gate check.
	earned, err := s.levelRepo.GetEarnedItemIDs(ctx, userID, []uuid.UUID{contextItemID})
	if err != nil {
		return SubmitResult{}, fmt.Errorf("submit answer: check earn gate: %w", err)
	}
	if len(earned) == 0 {
		return SubmitResult{}, ErrNotEarned
	}

	// Fetch quiz.
	quiz, err := s.repo.GetByContextItemID(ctx, contextItemID)
	if err != nil {
		return SubmitResult{}, fmt.Errorf("submit answer: fetch quiz: %w", err)
	}
	if quiz == nil {
		return SubmitResult{}, ErrNotFound
	}

	// Evaluate correctness.
	isCorrect := answerIndex == quiz.CorrectIndex

	// Award random coins (1~maxBonus) only if correct.
	coins := 0
	if isCorrect {
		coins = 1 + rand.Intn(s.maxBonus) // [1, maxBonus]
	}

	memo := fmt.Sprintf("퀴즈 보너스: %s", topicName)
	newTotal, err := s.repo.SaveResultAndAwardCoins(
		ctx, userID, quiz.ID, contextItemID,
		answerIndex, isCorrect, coins, s.levelCfg.CoinCap, memo,
	)
	if err != nil {
		return SubmitResult{}, fmt.Errorf("submit answer: save result: %w", err)
	}

	return SubmitResult{
		Correct:     isCorrect,
		CoinsEarned: coins,
		TotalCoins:  newTotal,
	}, nil
}
