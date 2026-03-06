package integration

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"ota/domain/level"
	"ota/storage"
)

// TestCoinEarning_FirstEarn: 최초 코인 획득 플로우를 검증합니다.
// 오늘 생성된 run + 새 topic → 코인이 적립되어야 합니다.
func TestCoinEarning_FirstEarn(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, "coin_logs", "user_points", "context_items", "collection_runs", "users")

	ctx := context.Background()

	// 1. 유저 생성
	var userID string
	err := db.Pool.QueryRow(ctx, `
		INSERT INTO users (kakao_id, email, nickname)
		VALUES (111, 'earner@example.com', 'EarnUser')
		RETURNING id
	`).Scan(&userID)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// 2. 오늘 날짜 기준의 collection_run 생성 (KST)
	var runID uuid.UUID
	err = db.Pool.QueryRow(ctx, `
		INSERT INTO collection_runs (status, started_at, completed_at)
		VALUES ('success', NOW(), NOW())
		RETURNING id
	`).Scan(&runID)
	if err != nil {
		t.Fatalf("failed to create run: %v", err)
	}

	// 3. context_item 생성 (선호 카테고리 'top')
	var itemID uuid.UUID
	err = db.Pool.QueryRow(ctx, `
		INSERT INTO context_items (collection_run_id, category, rank, topic, summary, brain_category, sources)
		VALUES ($1, 'top', 1, '오늘의 이슈', '요약입니다.', 'must_know', '[]')
		RETURNING id
	`, runID).Scan(&itemID)
	if err != nil {
		t.Fatalf("failed to create context item: %v", err)
	}

	// 4. 코인 적립 실행
	levelRepo := storage.NewLevelRepository(db.Pool)
	historyRepo := storage.NewHistoryRepository(db.Pool)
	svc := level.NewService(levelRepo, level.NewLevelConfig(5000, 1000), 0, 0)

	// run이 오늘 생성됐는지 확인
	isToday, err := historyRepo.IsRunCreatedToday(ctx, runID)
	if err != nil {
		t.Fatalf("IsRunCreatedToday error: %v", err)
	}
	if !isToday {
		t.Fatal("expected run to be created today")
	}

	// 선호 카테고리 여부: 'top'은 항상 preferred
	preferred := level.IsPreferredCategory("top", nil)
	if !preferred {
		t.Fatal("expected 'top' to be preferred")
	}

	result, err := svc.EarnCoin(ctx, userID, runID, itemID, preferred)
	if err != nil {
		t.Fatalf("EarnCoin error: %v", err)
	}

	if !result.Earned {
		t.Error("expected coins to be earned")
	}

	expectedCoins := level.CalcCoins(true) // 첫 적립, days=0
	if result.CoinsEarned != expectedCoins {
		t.Errorf("expected %d coins earned, got %d", expectedCoins, result.CoinsEarned)
	}

	// 5. DB에서 실제 coin_log 확인
	var logCount int
	err = db.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM coin_logs WHERE user_id = $1 AND run_id = $2 AND context_item_id = $3
	`, userID, runID, itemID).Scan(&logCount)
	if err != nil {
		t.Fatalf("failed to count coin logs: %v", err)
	}
	if logCount != 1 {
		t.Errorf("expected 1 coin_log, got %d", logCount)
	}

	// 6. user_points 총계 확인
	var totalCoins int
	err = db.Pool.QueryRow(ctx, `
		SELECT points FROM user_points WHERE user_id = $1
	`, userID).Scan(&totalCoins)
	if err != nil {
		t.Fatalf("failed to get user_points: %v", err)
	}
	if totalCoins != expectedCoins {
		t.Errorf("expected %d total coins in DB, got %d", expectedCoins, totalCoins)
	}

	t.Logf("FirstEarn passed: +%d코인 (preferred=top)", result.CoinsEarned)
}

// TestCoinEarning_DuplicateEarnBlocked: 같은 run+topic 조합으로 두 번 클릭 시 중복 적립 방지를 검증합니다.
func TestCoinEarning_DuplicateEarnBlocked(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, "coin_logs", "user_points", "context_items", "collection_runs", "users")

	ctx := context.Background()

	// 유저+run+item 생성
	var userID string
	err := db.Pool.QueryRow(ctx, `
		INSERT INTO users (kakao_id, email, nickname)
		VALUES (222, 'dup@example.com', 'DupUser')
		RETURNING id
	`).Scan(&userID)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	var runID uuid.UUID
	err = db.Pool.QueryRow(ctx, `
		INSERT INTO collection_runs (status, started_at, completed_at)
		VALUES ('success', NOW(), NOW())
		RETURNING id
	`).Scan(&runID)
	if err != nil {
		t.Fatalf("failed to create run: %v", err)
	}

	var itemID uuid.UUID
	err = db.Pool.QueryRow(ctx, `
		INSERT INTO context_items (collection_run_id, category, rank, topic, summary, sources)
		VALUES ($1, 'top', 1, '중복 테스트', '요약', '[]')
		RETURNING id
	`, runID).Scan(&itemID)
	if err != nil {
		t.Fatalf("failed to create item: %v", err)
	}

	levelRepo := storage.NewLevelRepository(db.Pool)
	svc := level.NewService(levelRepo, level.NewLevelConfig(5000, 1000), 0, 0)

	// 첫 번째 적립
	result1, err := svc.EarnCoin(ctx, userID, runID, itemID, true)
	if err != nil {
		t.Fatalf("first EarnCoin error: %v", err)
	}
	if !result1.Earned {
		t.Error("expected first earn to succeed")
	}

	// 두 번째 클릭 (동일 run+item → 중복 방지)
	result2, err := svc.EarnCoin(ctx, userID, runID, itemID, true)
	if err != nil {
		t.Fatalf("second EarnCoin error: %v", err)
	}
	if result2.Earned {
		t.Error("expected second earn to be blocked (duplicate)")
	}

	// coin_logs 에 딱 1건만 있어야 함
	var logCount int
	err = db.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM coin_logs WHERE user_id = $1 AND run_id = $2 AND context_item_id = $3
	`, userID, runID, itemID).Scan(&logCount)
	if err != nil {
		t.Fatalf("failed to count coin logs: %v", err)
	}
	if logCount != 1 {
		t.Errorf("expected 1 coin_log (no duplicate), got %d", logCount)
	}

	t.Log("DuplicateEarnBlocked passed: second click correctly blocked")
}

// TestCoinEarning_NonPreferredHigherCoins: 비선호 카테고리는 선호보다 더 많은 코인을 획득합니다.
func TestCoinEarning_NonPreferredHigherCoins(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, "coin_logs", "user_points", "context_items", "collection_runs", "users")

	ctx := context.Background()

	var userID string
	err := db.Pool.QueryRow(ctx, `
		INSERT INTO users (kakao_id, email, nickname)
		VALUES (333, 'nonpref@example.com', 'NonPrefUser')
		RETURNING id
	`).Scan(&userID)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	var runID uuid.UUID
	err = db.Pool.QueryRow(ctx, `
		INSERT INTO collection_runs (status, started_at, completed_at)
		VALUES ('success', NOW(), NOW())
		RETURNING id
	`).Scan(&runID)
	if err != nil {
		t.Fatalf("failed to create run: %v", err)
	}

	var itemID uuid.UUID
	err = db.Pool.QueryRow(ctx, `
		INSERT INTO context_items (collection_run_id, category, rank, topic, summary, sources)
		VALUES ($1, 'economy', 1, '경제 뉴스', '요약', '[]')
		RETURNING id
	`, runID).Scan(&itemID)
	if err != nil {
		t.Fatalf("failed to create item: %v", err)
	}

	levelRepo := storage.NewLevelRepository(db.Pool)
	svc := level.NewService(levelRepo, level.NewLevelConfig(5000, 1000), 0, 0)

	// 비선호 카테고리로 코인 적립
	result, err := svc.EarnCoin(ctx, userID, runID, itemID, false)
	if err != nil {
		t.Fatalf("EarnCoin error: %v", err)
	}

	expectedCoins := level.CalcCoins(false) // 비선호 기본 10코인
	if result.CoinsEarned != expectedCoins {
		t.Errorf("expected %d coins for non-preferred, got %d", expectedCoins, result.CoinsEarned)
	}

	// 비선호 > 선호 코인 확인
	preferredCoins := level.CalcCoins(true)
	if result.CoinsEarned <= preferredCoins {
		t.Errorf("non-preferred coins (%d) should be greater than preferred (%d)", result.CoinsEarned, preferredCoins)
	}

	t.Logf("NonPreferredHigherCoins passed: non-pref=%d코인, pref=%d코인", result.CoinsEarned, preferredCoins)
}

// TestCoinEarning_IsRunCreatedToday_OldRun: 과거에 생성된 run이면 false를 반환합니다.
func TestCoinEarning_IsRunCreatedToday_OldRun(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, "collection_runs")

	ctx := context.Background()

	// yesterday (KST)
	yesterday := time.Now().UTC().Add(-25 * time.Hour)

	var runID uuid.UUID
	err := db.Pool.QueryRow(ctx, `
		INSERT INTO collection_runs (status, started_at, completed_at)
		VALUES ('success', $1, $1)
		RETURNING id
	`, yesterday).Scan(&runID)
	if err != nil {
		t.Fatalf("failed to create old run: %v", err)
	}

	historyRepo := storage.NewHistoryRepository(db.Pool)
	isToday, err := historyRepo.IsRunCreatedToday(ctx, runID)
	if err != nil {
		t.Fatalf("IsRunCreatedToday error: %v", err)
	}
	if isToday {
		t.Error("expected old run to return false for IsRunCreatedToday")
	}

	t.Log("IsRunCreatedToday_OldRun passed: correctly returned false for old run")
}

// TestCoinEarning_LevelUp: 충분한 코인 적립 시 레벨업 감지를 검증합니다.
func TestCoinEarning_LevelUp(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, "coin_logs", "user_points", "context_items", "collection_runs", "users")

	ctx := context.Background()

	var userID string
	err := db.Pool.QueryRow(ctx, `
		INSERT INTO users (kakao_id, email, nickname)
		VALUES (444, 'levelup@example.com', 'LevelUpUser')
		RETURNING id
	`).Scan(&userID)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	levelRepo := storage.NewLevelRepository(db.Pool)
	svc := level.NewService(levelRepo, level.NewLevelConfig(5000, 1000), 0, 0)

	// 레벨 1→2 경계(1000코인)에 가깝게 사전 설정 (비선호 10코인 → 990+10=1000 → lv2)
	// NewLevelConfig(5000,1000) 기준: 0=lv1, 1000=lv2, 2000=lv3, 3000=lv4, 4000=lv5
	if err := levelRepo.SetCoins(ctx, userID, 990); err != nil {
		t.Fatalf("SetCoins error: %v", err)
	}

	var runID uuid.UUID
	err = db.Pool.QueryRow(ctx, `
		INSERT INTO collection_runs (status, started_at, completed_at)
		VALUES ('success', NOW(), NOW())
		RETURNING id
	`).Scan(&runID)
	if err != nil {
		t.Fatalf("failed to create run: %v", err)
	}

	var itemID uuid.UUID
	err = db.Pool.QueryRow(ctx, `
		INSERT INTO context_items (collection_run_id, category, rank, topic, summary, sources)
		VALUES ($1, 'economy', 1, '레벨업 테스트', '요약', '[]')
		RETURNING id
	`, runID).Scan(&itemID)
	if err != nil {
		t.Fatalf("failed to create item: %v", err)
	}

	// 비선호(10코인) 클릭 → 990+10=1000코인 → lv2
	result, err := svc.EarnCoin(ctx, userID, runID, itemID, false)
	if err != nil {
		t.Fatalf("EarnCoin error: %v", err)
	}

	if !result.Earned {
		t.Error("expected earn to succeed")
	}

	if !result.LeveledUp {
		t.Errorf("expected level up, got level=%d totalCoins=%d", result.Level, result.TotalCoins)
	}

	if result.Level < 2 {
		t.Errorf("expected level >= 2 after levelup, got %d", result.Level)
	}

	t.Logf("LevelUp passed: lv%d, totalCoins=%d, leveledUp=%v", result.Level, result.TotalCoins, result.LeveledUp)
}
