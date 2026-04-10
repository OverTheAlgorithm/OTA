package integration

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"ota/domain/level"
	"ota/domain/quiz"
	"ota/storage"
)

// createQuizTestData inserts a user, collection_run, and context_item, returning their IDs.
func createQuizTestData(t *testing.T, db *TestDB, kakaoID int, email, nickname string) (userID string, runID, itemID uuid.UUID) {
	t.Helper()
	ctx := context.Background()

	err := db.Pool.QueryRow(ctx, `
		INSERT INTO users (kakao_id, email, nickname)
		VALUES ($1, $2, $3)
		RETURNING id
	`, kakaoID, email, nickname).Scan(&userID)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	err = db.Pool.QueryRow(ctx, `
		INSERT INTO collection_runs (status, started_at, completed_at)
		VALUES ('success', NOW(), NOW())
		RETURNING id
	`).Scan(&runID)
	if err != nil {
		t.Fatalf("failed to create collection_run: %v", err)
	}

	err = db.Pool.QueryRow(ctx, `
		INSERT INTO context_items (collection_run_id, category, rank, topic, summary, brain_category, sources)
		VALUES ($1, 'top', 1, '퀴즈 테스트 토픽', '요약입니다.', 'must_know', '[]')
		RETURNING id
	`, runID).Scan(&itemID)
	if err != nil {
		t.Fatalf("failed to create context_item: %v", err)
	}

	return userID, runID, itemID
}

// insertCoinLog creates a coin_logs entry (simulating the user having earned coins for this article).
func insertCoinLog(t *testing.T, db *TestDB, userID string, runID, itemID uuid.UUID, coins int) {
	t.Helper()
	ctx := context.Background()

	_, err := db.Pool.Exec(ctx, `
		INSERT INTO coin_logs (user_id, run_id, context_item_id, coins_earned)
		VALUES ($1, $2, $3, $4)
	`, userID, runID, itemID, coins)
	if err != nil {
		t.Fatalf("failed to insert coin_log: %v", err)
	}

	// Also ensure user_points row exists
	_, err = db.Pool.Exec(ctx, `
		INSERT INTO user_points (user_id, points, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (user_id) DO UPDATE SET points = user_points.points + $2, updated_at = NOW()
	`, userID, coins)
	if err != nil {
		t.Fatalf("failed to upsert user_points: %v", err)
	}
}

// makeTestQuiz creates a quiz.Quiz value for testing.
func makeTestQuiz(contextItemID uuid.UUID) quiz.Quiz {
	return quiz.Quiz{
		ID:            uuid.New(),
		ContextItemID: contextItemID,
		Question:      "다음 중 올바른 것은?",
		Options:       []string{"보기1", "보기2", "보기3", "보기4"},
		CorrectIndex:  2,
	}
}

var quizTables = []string{
	"quiz_results", "quizzes", "coin_events", "coin_logs", "user_points",
	"context_items", "collection_runs", "users",
}

// TestQuiz_SaveAndRetrieve: SaveQuizBatch로 저장 후 GetByContextItemID로 조회합니다.
func TestQuiz_SaveAndRetrieve(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, quizTables...)

	ctx := context.Background()
	_, _, itemID := createQuizTestData(t, db, 1001, "quiz-save@example.com", "QuizSaveUser")

	quizRepo := storage.NewQuizRepository(db.Pool)

	q := makeTestQuiz(itemID)
	err := quizRepo.SaveQuizBatch(ctx, []quiz.Quiz{q})
	if err != nil {
		t.Fatalf("SaveQuizBatch error: %v", err)
	}

	got, err := quizRepo.GetByContextItemID(ctx, itemID)
	if err != nil {
		t.Fatalf("GetByContextItemID error: %v", err)
	}
	if got == nil {
		t.Fatal("expected quiz to exist, got nil")
	}

	if got.Question != q.Question {
		t.Errorf("question mismatch: want %q, got %q", q.Question, got.Question)
	}
	if len(got.Options) != len(q.Options) {
		t.Fatalf("options length mismatch: want %d, got %d", len(q.Options), len(got.Options))
	}
	for i, opt := range q.Options {
		if got.Options[i] != opt {
			t.Errorf("option[%d] mismatch: want %q, got %q", i, opt, got.Options[i])
		}
	}
	if got.CorrectIndex != q.CorrectIndex {
		t.Errorf("correct_index mismatch: want %d, got %d", q.CorrectIndex, got.CorrectIndex)
	}

	t.Log("SaveAndRetrieve passed: quiz saved and retrieved correctly")
}

// TestQuiz_ReadPathOpenSubmitGated: READ 경로(GetQuizForUser)는 coin_logs 없이도 열려 있고,
// SUBMIT 경로(SubmitAnswer)는 여전히 coin_logs를 검사한다는 invariant를 검증합니다.
//
// 이전 동작에서는 GetQuizForUser가 ErrNotEarned로 거부해 프론트가 stale null을 들고
// "earn 후 카드 사라짐" 버그를 일으켰습니다. 이제 READ는 항상 열리고, 치팅 방지는
// SubmitAnswer의 authoritative gate에만 의존합니다.
func TestQuiz_ReadPathOpenSubmitGated(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, quizTables...)

	ctx := context.Background()
	userID, _, itemID := createQuizTestData(t, db, 1002, "quiz-nolearn@example.com", "QuizNoEarnUser")

	// Save quiz so GetQuizForUser has something to return.
	quizRepo := storage.NewQuizRepository(db.Pool)
	q := makeTestQuiz(itemID)
	if err := quizRepo.SaveQuiz(ctx, q); err != nil {
		t.Fatalf("SaveQuiz error: %v", err)
	}

	levelRepo := storage.NewLevelRepository(db.Pool)
	levelCfg := level.NewLevelConfig(5000, 1000)
	svc := quiz.NewService(quizRepo, levelRepo, levelCfg, 10)

	// READ: no coin_logs entry — but the read path is intentionally open.
	got, err := svc.GetQuizForUser(ctx, userID, itemID)
	if err != nil {
		t.Fatalf("GetQuizForUser should not error without coin_logs, got: %v", err)
	}
	if got == nil {
		t.Fatal("expected quiz to be returned without coin_logs, got nil")
	}
	if got.PastAttempt != nil {
		t.Errorf("expected PastAttempt nil for fresh user, got: %+v", got.PastAttempt)
	}
	if got.Question != q.Question {
		t.Errorf("question mismatch: want %q, got %q", q.Question, got.Question)
	}

	// SUBMIT: still gated — without a coin_logs entry, SubmitAnswer must reject.
	_, err = svc.SubmitAnswer(ctx, userID, itemID, 2, "퀴즈 테스트 토픽")
	if err == nil {
		t.Fatal("SubmitAnswer should reject without coin_logs, got nil error")
	}
	if err != quiz.ErrNotEarned {
		t.Fatalf("expected ErrNotEarned from SubmitAnswer, got: %v", err)
	}

	t.Log("ReadPathOpenSubmitGated passed: read open, submit still gated")
}

// TestQuiz_EarnGateAllows: coin_logs 존재 시 퀴즈 조회 성공을 검증합니다.
func TestQuiz_EarnGateAllows(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, quizTables...)

	ctx := context.Background()
	userID, runID, itemID := createQuizTestData(t, db, 1003, "quiz-earned@example.com", "QuizEarnedUser")

	// Create coin_logs entry (earn gate prerequisite)
	insertCoinLog(t, db, userID, runID, itemID, 5)

	// Save quiz
	quizRepo := storage.NewQuizRepository(db.Pool)
	q := makeTestQuiz(itemID)
	if err := quizRepo.SaveQuiz(ctx, q); err != nil {
		t.Fatalf("SaveQuiz error: %v", err)
	}

	levelRepo := storage.NewLevelRepository(db.Pool)
	levelCfg := level.NewLevelConfig(5000, 1000)
	svc := quiz.NewService(quizRepo, levelRepo, levelCfg, 10)

	got, err := svc.GetQuizForUser(ctx, userID, itemID)
	if err != nil {
		t.Fatalf("GetQuizForUser error: %v", err)
	}
	if got == nil {
		t.Fatal("expected quiz for user, got nil")
	}
	if got.Question != q.Question {
		t.Errorf("question mismatch: want %q, got %q", q.Question, got.Question)
	}

	t.Log("EarnGateAllows passed: quiz accessible with coin_logs entry")
}

// TestQuiz_PastAttemptHydration_Correct: 정답 attempt가 있을 때 GetQuizForUser가
// PastAttempt 필드를 채워서 반환하는지 검증합니다 (hydration UI용).
func TestQuiz_PastAttemptHydration_Correct(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, quizTables...)

	ctx := context.Background()
	userID, _, itemID := createQuizTestData(t, db, 1100, "quiz-hydrate-c@example.com", "QuizHydrateCorrect")

	quizRepo := storage.NewQuizRepository(db.Pool)
	q := makeTestQuiz(itemID)
	if err := quizRepo.SaveQuiz(ctx, q); err != nil {
		t.Fatalf("SaveQuiz error: %v", err)
	}

	// Insert a correct attempt directly (bypass SubmitAnswer for test isolation).
	_, err := db.Pool.Exec(ctx, `
		INSERT INTO quiz_results (user_id, quiz_id, context_item_id, answered_index, is_correct, coins_earned)
		VALUES ($1, $2, $3, 2, true, 7)
	`, userID, q.ID, itemID)
	if err != nil {
		t.Fatalf("insert quiz_results error: %v", err)
	}

	levelRepo := storage.NewLevelRepository(db.Pool)
	levelCfg := level.NewLevelConfig(5000, 1000)
	svc := quiz.NewService(quizRepo, levelRepo, levelCfg, 10)

	got, err := svc.GetQuizForUser(ctx, userID, itemID)
	if err != nil {
		t.Fatalf("GetQuizForUser error: %v", err)
	}
	if got == nil {
		t.Fatal("expected quiz, got nil")
	}
	if got.PastAttempt == nil {
		t.Fatal("expected PastAttempt to be non-nil for hydration")
	}
	if got.PastAttempt.SelectedIndex != 2 {
		t.Errorf("SelectedIndex: want 2, got %d", got.PastAttempt.SelectedIndex)
	}
	if !got.PastAttempt.IsCorrect {
		t.Error("IsCorrect: want true, got false")
	}
	if got.PastAttempt.CoinsEarned != 7 {
		t.Errorf("CoinsEarned: want 7, got %d", got.PastAttempt.CoinsEarned)
	}
	if got.PastAttempt.AttemptedAt.IsZero() {
		t.Error("AttemptedAt should not be zero")
	}

	t.Logf("PastAttemptHydration_Correct passed: hydrated +%d coins, attempted_at=%s",
		got.PastAttempt.CoinsEarned, got.PastAttempt.AttemptedAt.Format("2006-01-02T15:04:05Z"))
}

// TestQuiz_PastAttemptHydration_Wrong: 오답 attempt가 있을 때도 PastAttempt가 채워져
// 프론트가 정적 오답 카드를 hydration할 수 있는지 검증합니다.
func TestQuiz_PastAttemptHydration_Wrong(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, quizTables...)

	ctx := context.Background()
	userID, _, itemID := createQuizTestData(t, db, 1101, "quiz-hydrate-w@example.com", "QuizHydrateWrong")

	quizRepo := storage.NewQuizRepository(db.Pool)
	q := makeTestQuiz(itemID)
	if err := quizRepo.SaveQuiz(ctx, q); err != nil {
		t.Fatalf("SaveQuiz error: %v", err)
	}

	// Insert a wrong attempt directly.
	_, err := db.Pool.Exec(ctx, `
		INSERT INTO quiz_results (user_id, quiz_id, context_item_id, answered_index, is_correct, coins_earned)
		VALUES ($1, $2, $3, 0, false, 0)
	`, userID, q.ID, itemID)
	if err != nil {
		t.Fatalf("insert quiz_results error: %v", err)
	}

	levelRepo := storage.NewLevelRepository(db.Pool)
	levelCfg := level.NewLevelConfig(5000, 1000)
	svc := quiz.NewService(quizRepo, levelRepo, levelCfg, 10)

	got, err := svc.GetQuizForUser(ctx, userID, itemID)
	if err != nil {
		t.Fatalf("GetQuizForUser error: %v", err)
	}
	if got == nil || got.PastAttempt == nil {
		t.Fatal("expected quiz with PastAttempt, got nil")
	}
	if got.PastAttempt.SelectedIndex != 0 {
		t.Errorf("SelectedIndex: want 0, got %d", got.PastAttempt.SelectedIndex)
	}
	if got.PastAttempt.IsCorrect {
		t.Error("IsCorrect: want false, got true")
	}
	if got.PastAttempt.CoinsEarned != 0 {
		t.Errorf("CoinsEarned: want 0, got %d", got.PastAttempt.CoinsEarned)
	}

	t.Log("PastAttemptHydration_Wrong passed: hydrated wrong attempt with 0 coins")
}

// TestQuiz_PastAttemptHydration_NoAttempt: attempt가 없을 때 PastAttempt가 nil인지 검증합니다.
// 새 유저(IDLE 상태)가 퀴즈를 처음 볼 때의 정상 케이스.
func TestQuiz_PastAttemptHydration_NoAttempt(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, quizTables...)

	ctx := context.Background()
	userID, _, itemID := createQuizTestData(t, db, 1102, "quiz-hydrate-n@example.com", "QuizHydrateNone")

	quizRepo := storage.NewQuizRepository(db.Pool)
	q := makeTestQuiz(itemID)
	if err := quizRepo.SaveQuiz(ctx, q); err != nil {
		t.Fatalf("SaveQuiz error: %v", err)
	}

	levelRepo := storage.NewLevelRepository(db.Pool)
	levelCfg := level.NewLevelConfig(5000, 1000)
	svc := quiz.NewService(quizRepo, levelRepo, levelCfg, 10)

	got, err := svc.GetQuizForUser(ctx, userID, itemID)
	if err != nil {
		t.Fatalf("GetQuizForUser error: %v", err)
	}
	if got == nil {
		t.Fatal("expected quiz, got nil")
	}
	if got.PastAttempt != nil {
		t.Errorf("expected PastAttempt nil, got: %+v", got.PastAttempt)
	}
	if got.Question != q.Question {
		t.Errorf("question mismatch: want %q, got %q", q.Question, got.Question)
	}

	t.Log("PastAttemptHydration_NoAttempt passed: nil PastAttempt for fresh user")
}

// TestQuiz_CorrectAnswerAwardsCoins: 정답 제출 시 코인 지급을 검증합니다.
func TestQuiz_CorrectAnswerAwardsCoins(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, quizTables...)

	ctx := context.Background()
	userID, runID, itemID := createQuizTestData(t, db, 1004, "quiz-correct@example.com", "QuizCorrectUser")

	insertCoinLog(t, db, userID, runID, itemID, 5)

	quizRepo := storage.NewQuizRepository(db.Pool)
	q := makeTestQuiz(itemID)
	if err := quizRepo.SaveQuiz(ctx, q); err != nil {
		t.Fatalf("SaveQuiz error: %v", err)
	}

	levelRepo := storage.NewLevelRepository(db.Pool)
	levelCfg := level.NewLevelConfig(5000, 1000)
	maxBonus := 10
	svc := quiz.NewService(quizRepo, levelRepo, levelCfg, maxBonus)

	// Get initial points
	var initialPoints int
	err := db.Pool.QueryRow(ctx, `SELECT COALESCE((SELECT points FROM user_points WHERE user_id = $1), 0)`, userID).Scan(&initialPoints)
	if err != nil {
		t.Fatalf("failed to read initial points: %v", err)
	}

	// Submit correct answer (correct_index = 2)
	result, err := svc.SubmitAnswer(ctx, userID, itemID, 2, "퀴즈 테스트 토픽")
	if err != nil {
		t.Fatalf("SubmitAnswer error: %v", err)
	}

	if !result.Correct {
		t.Error("expected correct=true")
	}
	if result.CoinsEarned <= 0 {
		t.Errorf("expected coins_earned > 0, got %d", result.CoinsEarned)
	}

	// Verify quiz_results row
	var isCorrect bool
	var coinsEarned int
	err = db.Pool.QueryRow(ctx, `
		SELECT is_correct, coins_earned FROM quiz_results
		WHERE user_id = $1 AND quiz_id = $2
	`, userID, q.ID).Scan(&isCorrect, &coinsEarned)
	if err != nil {
		t.Fatalf("failed to read quiz_results: %v", err)
	}
	if !isCorrect {
		t.Error("quiz_results.is_correct should be true")
	}
	if coinsEarned <= 0 {
		t.Errorf("quiz_results.coins_earned should be > 0, got %d", coinsEarned)
	}

	// Verify coin_events row with type='quiz_bonus'
	var eventCount int
	err = db.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM coin_events WHERE user_id = $1 AND type = 'quiz_bonus'
	`, userID).Scan(&eventCount)
	if err != nil {
		t.Fatalf("failed to count coin_events: %v", err)
	}
	if eventCount != 1 {
		t.Errorf("expected 1 coin_event with type='quiz_bonus', got %d", eventCount)
	}

	// Verify user_points increased
	var finalPoints int
	err = db.Pool.QueryRow(ctx, `SELECT points FROM user_points WHERE user_id = $1`, userID).Scan(&finalPoints)
	if err != nil {
		t.Fatalf("failed to read final points: %v", err)
	}
	if finalPoints != initialPoints+coinsEarned {
		t.Errorf("expected points=%d, got %d", initialPoints+coinsEarned, finalPoints)
	}

	t.Logf("CorrectAnswerAwardsCoins passed: +%d coins, total=%d", coinsEarned, finalPoints)
}

// TestQuiz_WrongAnswerNoCoins: 오답 제출 시 코인 미지급을 검증합니다.
func TestQuiz_WrongAnswerNoCoins(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, quizTables...)

	ctx := context.Background()
	userID, runID, itemID := createQuizTestData(t, db, 1005, "quiz-wrong@example.com", "QuizWrongUser")

	insertCoinLog(t, db, userID, runID, itemID, 5)

	quizRepo := storage.NewQuizRepository(db.Pool)
	q := makeTestQuiz(itemID) // correct_index = 2
	if err := quizRepo.SaveQuiz(ctx, q); err != nil {
		t.Fatalf("SaveQuiz error: %v", err)
	}

	levelRepo := storage.NewLevelRepository(db.Pool)
	levelCfg := level.NewLevelConfig(5000, 1000)
	svc := quiz.NewService(quizRepo, levelRepo, levelCfg, 10)

	var initialPoints int
	err := db.Pool.QueryRow(ctx, `SELECT COALESCE((SELECT points FROM user_points WHERE user_id = $1), 0)`, userID).Scan(&initialPoints)
	if err != nil {
		t.Fatalf("failed to read initial points: %v", err)
	}

	// Submit wrong answer (correct is 2, submit 0)
	result, err := svc.SubmitAnswer(ctx, userID, itemID, 0, "퀴즈 테스트 토픽")
	if err != nil {
		t.Fatalf("SubmitAnswer error: %v", err)
	}

	if result.Correct {
		t.Error("expected correct=false for wrong answer")
	}
	if result.CoinsEarned != 0 {
		t.Errorf("expected coins_earned=0, got %d", result.CoinsEarned)
	}

	// Verify quiz_results row with is_correct=false
	var isCorrect bool
	var coinsEarned int
	err = db.Pool.QueryRow(ctx, `
		SELECT is_correct, coins_earned FROM quiz_results
		WHERE user_id = $1 AND quiz_id = $2
	`, userID, q.ID).Scan(&isCorrect, &coinsEarned)
	if err != nil {
		t.Fatalf("failed to read quiz_results: %v", err)
	}
	if isCorrect {
		t.Error("quiz_results.is_correct should be false")
	}
	if coinsEarned != 0 {
		t.Errorf("quiz_results.coins_earned should be 0, got %d", coinsEarned)
	}

	// Verify NO coin_events created
	var eventCount int
	err = db.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM coin_events WHERE user_id = $1 AND type = 'quiz_bonus'
	`, userID).Scan(&eventCount)
	if err != nil {
		t.Fatalf("failed to count coin_events: %v", err)
	}
	if eventCount != 0 {
		t.Errorf("expected 0 coin_events for wrong answer, got %d", eventCount)
	}

	// Verify user_points unchanged
	var finalPoints int
	err = db.Pool.QueryRow(ctx, `SELECT points FROM user_points WHERE user_id = $1`, userID).Scan(&finalPoints)
	if err != nil {
		t.Fatalf("failed to read final points: %v", err)
	}
	if finalPoints != initialPoints {
		t.Errorf("expected points unchanged at %d, got %d", initialPoints, finalPoints)
	}

	t.Log("WrongAnswerNoCoins passed: no coins awarded for wrong answer")
}

// TestQuiz_DuplicateSubmissionBlocked: 동일 퀴즈 이중 제출 시 에러 반환을 검증합니다.
func TestQuiz_DuplicateSubmissionBlocked(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, quizTables...)

	ctx := context.Background()
	userID, runID, itemID := createQuizTestData(t, db, 1006, "quiz-dup@example.com", "QuizDupUser")

	insertCoinLog(t, db, userID, runID, itemID, 5)

	quizRepo := storage.NewQuizRepository(db.Pool)
	q := makeTestQuiz(itemID)
	if err := quizRepo.SaveQuiz(ctx, q); err != nil {
		t.Fatalf("SaveQuiz error: %v", err)
	}

	levelRepo := storage.NewLevelRepository(db.Pool)
	levelCfg := level.NewLevelConfig(5000, 1000)
	svc := quiz.NewService(quizRepo, levelRepo, levelCfg, 10)

	// First submission (correct answer)
	_, err := svc.SubmitAnswer(ctx, userID, itemID, 2, "퀴즈 테스트 토픽")
	if err != nil {
		t.Fatalf("first SubmitAnswer error: %v", err)
	}

	// Second submission (should fail with UNIQUE constraint)
	_, err = svc.SubmitAnswer(ctx, userID, itemID, 0, "퀴즈 테스트 토픽")
	if err == nil {
		t.Fatal("expected error on duplicate submission, got nil")
	}

	t.Logf("DuplicateSubmissionBlocked passed: second attempt returned error: %v", err)
}

// TestQuiz_BonusExemptFromDailyLimit: 일일 한도 도달 후에도 퀴즈 보너스가 지급됨을 검증합니다.
func TestQuiz_BonusExemptFromDailyLimit(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, quizTables...)

	ctx := context.Background()
	userID, runID, itemID := createQuizTestData(t, db, 1007, "quiz-limit@example.com", "QuizLimitUser")

	// Fill user to daily coin limit via multiple coin_logs entries
	dailyLimit := 10
	for i := 0; i < dailyLimit; i++ {
		var extraItemID uuid.UUID
		err := db.Pool.QueryRow(ctx, `
			INSERT INTO context_items (collection_run_id, category, rank, topic, summary, brain_category, sources)
			VALUES ($1, 'top', $2, $3, '요약', 'must_know', '[]')
			RETURNING id
		`, runID, i+10, "한도 채우기 토픽 "+string(rune('A'+i))).Scan(&extraItemID)
		if err != nil {
			t.Fatalf("failed to create extra context_item: %v", err)
		}

		_, err = db.Pool.Exec(ctx, `
			INSERT INTO coin_logs (user_id, run_id, context_item_id, coins_earned)
			VALUES ($1, $2, $3, 1)
		`, userID, runID, extraItemID)
		if err != nil {
			t.Fatalf("failed to insert coin_log for limit: %v", err)
		}
	}

	// Set user_points to match the daily limit coin_logs
	_, err := db.Pool.Exec(ctx, `
		INSERT INTO user_points (user_id, points, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (user_id) DO UPDATE SET points = $2, updated_at = NOW()
	`, userID, dailyLimit)
	if err != nil {
		t.Fatalf("failed to set user_points: %v", err)
	}

	// Verify GetTodayEarnedCoins = dailyLimit
	levelRepo := storage.NewLevelRepository(db.Pool)
	todayEarned, err := levelRepo.GetTodayEarnedCoins(ctx, userID)
	if err != nil {
		t.Fatalf("GetTodayEarnedCoins error: %v", err)
	}
	if todayEarned < dailyLimit {
		t.Fatalf("expected today earned >= %d, got %d", dailyLimit, todayEarned)
	}

	// Now create the coin_log for the quiz's article so earn-gate passes
	insertCoinLog(t, db, userID, runID, itemID, 0)
	// Reset points back (insertCoinLog adds to points)
	_, err = db.Pool.Exec(ctx, `
		UPDATE user_points SET points = $2 WHERE user_id = $1
	`, userID, dailyLimit)
	if err != nil {
		t.Fatalf("failed to reset user_points: %v", err)
	}

	// Save quiz and submit correct answer
	quizRepo := storage.NewQuizRepository(db.Pool)
	q := makeTestQuiz(itemID)
	if err := quizRepo.SaveQuiz(ctx, q); err != nil {
		t.Fatalf("SaveQuiz error: %v", err)
	}

	levelCfg := level.NewLevelConfig(5000, 1000)
	svc := quiz.NewService(quizRepo, levelRepo, levelCfg, 10)

	result, err := svc.SubmitAnswer(ctx, userID, itemID, 2, "퀴즈 테스트 토픽")
	if err != nil {
		t.Fatalf("SubmitAnswer error: %v", err)
	}

	if !result.Correct {
		t.Error("expected correct answer")
	}
	// Quiz bonus goes through coin_events, NOT coin_logs, so it bypasses daily limit
	if result.CoinsEarned <= 0 {
		t.Errorf("expected quiz bonus coins > 0 even at daily limit, got %d", result.CoinsEarned)
	}

	// Verify total points increased beyond the daily limit
	if result.TotalCoins <= dailyLimit {
		t.Errorf("expected total > %d after quiz bonus, got %d", dailyLimit, result.TotalCoins)
	}

	t.Logf("BonusExemptFromDailyLimit passed: +%d bonus coins at daily limit, total=%d", result.CoinsEarned, result.TotalCoins)
}

// TestQuiz_BatchQuizStatus: 복수 퀴즈의 존재/완료 상태 맵 조회를 검증합니다.
func TestQuiz_BatchQuizStatus(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, quizTables...)

	ctx := context.Background()
	userID, runID, _ := createQuizTestData(t, db, 1008, "quiz-batch@example.com", "QuizBatchUser")

	// Create 3 context_items
	itemIDs := make([]uuid.UUID, 3)
	for i := 0; i < 3; i++ {
		err := db.Pool.QueryRow(ctx, `
			INSERT INTO context_items (collection_run_id, category, rank, topic, summary, brain_category, sources)
			VALUES ($1, 'top', $2, $3, '요약', 'must_know', '[]')
			RETURNING id
		`, runID, i+100, "배치 토픽 "+string(rune('A'+i))).Scan(&itemIDs[i])
		if err != nil {
			t.Fatalf("failed to create context_item[%d]: %v", i, err)
		}
	}

	quizRepo := storage.NewQuizRepository(db.Pool)

	// Save quizzes for items 0 and 1 (not item 2)
	for i := 0; i < 2; i++ {
		q := makeTestQuiz(itemIDs[i])
		if err := quizRepo.SaveQuiz(ctx, q); err != nil {
			t.Fatalf("SaveQuiz[%d] error: %v", i, err)
		}
	}

	// Verify GetQuizExistenceMap: items 0,1 should exist, item 2 should not
	existMap, err := quizRepo.GetQuizExistenceMap(ctx, itemIDs)
	if err != nil {
		t.Fatalf("GetQuizExistenceMap error: %v", err)
	}
	if !existMap[itemIDs[0]] {
		t.Error("expected item[0] to have quiz")
	}
	if !existMap[itemIDs[1]] {
		t.Error("expected item[1] to have quiz")
	}
	if existMap[itemIDs[2]] {
		t.Error("expected item[2] to NOT have quiz")
	}

	// Complete quiz for item 0 only (via direct insert to quiz_results)
	var quizID uuid.UUID
	err = db.Pool.QueryRow(ctx, `SELECT id FROM quizzes WHERE context_item_id = $1`, itemIDs[0]).Scan(&quizID)
	if err != nil {
		t.Fatalf("failed to get quizID: %v", err)
	}
	_, err = db.Pool.Exec(ctx, `
		INSERT INTO quiz_results (user_id, quiz_id, context_item_id, answered_index, is_correct, coins_earned)
		VALUES ($1, $2, $3, 2, true, 5)
	`, userID, quizID, itemIDs[0])
	if err != nil {
		t.Fatalf("failed to insert quiz_result: %v", err)
	}

	// Verify GetQuizCompletionMap: only item 0 should be completed
	completionMap, err := quizRepo.GetQuizCompletionMap(ctx, userID, itemIDs)
	if err != nil {
		t.Fatalf("GetQuizCompletionMap error: %v", err)
	}
	if !completionMap[itemIDs[0]] {
		t.Error("expected item[0] to be completed")
	}
	if completionMap[itemIDs[1]] {
		t.Error("expected item[1] to NOT be completed")
	}
	if completionMap[itemIDs[2]] {
		t.Error("expected item[2] to NOT be completed")
	}

	t.Log("BatchQuizStatus passed: existence and completion maps correct")
}
