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

// GetQuizForUser returns quiz data for a user (or non-logged-in viewer when userID is empty).
//
// Earn-gate is intentionally NOT checked here — it is enforced authoritatively in
// SubmitAnswer. Exposing the quiz before earn is safe because submission is the gated
// operation, and the frontend hides quiz interaction until earn completes.
//
// When userID is non-empty and the user has already attempted the quiz, the returned
// QuizForUser includes a non-nil PastAttempt so the frontend can hydrate a static
// "already completed" card. For non-logged-in viewers (empty userID), PastAttempt is
// always nil.
//
// Returns nil (no error) when no quiz exists for the article.
// ErrNotEarned and ErrAlreadyAttempted are NOT returned from this function — they are
// only used by SubmitAnswer.
func (s *Service) GetQuizForUser(ctx context.Context, userID string, contextItemID uuid.UUID) (*QuizForUser, error) {
	// Fetch quiz (nil = no quiz for this article, not an error).
	quiz, err := s.repo.GetByContextItemID(ctx, contextItemID)
	if err != nil {
		return nil, fmt.Errorf("get quiz for user: fetch quiz: %w", err)
	}
	if quiz == nil {
		return nil, nil
	}

	result := &QuizForUser{
		ID:            quiz.ID,
		ContextItemID: quiz.ContextItemID,
		Question:      quiz.Question,
		Options:       quiz.Options,
	}

	// Past-attempt hydration only applies to logged-in users. Skip the DB round-trip
	// for non-logged-in viewers.
	if userID != "" {
		attempt, err := s.repo.GetUserAttempt(ctx, userID, quiz.ID)
		if err != nil {
			return nil, fmt.Errorf("get quiz for user: check past attempt: %w", err)
		}
		if attempt != nil {
			result.PastAttempt = attempt
		}
	}

	return result, nil
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
