package integration

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"ota/domain/level"
	"ota/storage"
)

// TestPointEarning_FirstEarn: 최초 포인트 획득 플로우를 검증합니다.
// 오늘 생성된 run + 새 topic → 포인트가 적립되어야 합니다.
func TestPointEarning_FirstEarn(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, "point_logs", "user_points", "context_items", "collection_runs", "users")

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

	// 4. 포인트 적립 실행
	levelRepo := storage.NewLevelRepository(db.Pool)
	historyRepo := storage.NewHistoryRepository(db.Pool)
	svc := level.NewService(levelRepo)

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

	result, err := svc.EarnPoint(ctx, userID, runID, itemID, preferred)
	if err != nil {
		t.Fatalf("EarnPoint error: %v", err)
	}

	if !result.Earned {
		t.Error("expected points to be earned")
	}

	expectedPoints := level.CalcPoints(true) // 첫 적립, days=0
	if result.PointsEarned != expectedPoints {
		t.Errorf("expected %d points earned, got %d", expectedPoints, result.PointsEarned)
	}

	// 5. DB에서 실제 point_log 확인
	var logCount int
	err = db.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM point_logs WHERE user_id = $1 AND run_id = $2 AND context_item_id = $3
	`, userID, runID, itemID).Scan(&logCount)
	if err != nil {
		t.Fatalf("failed to count point logs: %v", err)
	}
	if logCount != 1 {
		t.Errorf("expected 1 point_log, got %d", logCount)
	}

	// 6. user_points 총계 확인
	var totalPoints int
	err = db.Pool.QueryRow(ctx, `
		SELECT points FROM user_points WHERE user_id = $1
	`, userID).Scan(&totalPoints)
	if err != nil {
		t.Fatalf("failed to get user_points: %v", err)
	}
	if totalPoints != expectedPoints {
		t.Errorf("expected %d total points in DB, got %d", expectedPoints, totalPoints)
	}

	t.Logf("✓ FirstEarn passed: +%dpt (preferred=top)", result.PointsEarned)
}

// TestPointEarning_DuplicateEarnBlocked: 같은 run+topic 조합으로 두 번 클릭 시 중복 적립 방지를 검증합니다.
func TestPointEarning_DuplicateEarnBlocked(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, "point_logs", "user_points", "context_items", "collection_runs", "users")

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
	svc := level.NewService(levelRepo)

	// 첫 번째 적립
	result1, err := svc.EarnPoint(ctx, userID, runID, itemID, true)
	if err != nil {
		t.Fatalf("first EarnPoint error: %v", err)
	}
	if !result1.Earned {
		t.Error("expected first earn to succeed")
	}

	// 두 번째 클릭 (동일 run+item → 중복 방지)
	result2, err := svc.EarnPoint(ctx, userID, runID, itemID, true)
	if err != nil {
		t.Fatalf("second EarnPoint error: %v", err)
	}
	if result2.Earned {
		t.Error("expected second earn to be blocked (duplicate)")
	}

	// point_logs 에 딱 1건만 있어야 함
	var logCount int
	err = db.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM point_logs WHERE user_id = $1 AND run_id = $2 AND context_item_id = $3
	`, userID, runID, itemID).Scan(&logCount)
	if err != nil {
		t.Fatalf("failed to count point logs: %v", err)
	}
	if logCount != 1 {
		t.Errorf("expected 1 point_log (no duplicate), got %d", logCount)
	}

	t.Log("✓ DuplicateEarnBlocked passed: second click correctly blocked")
}

// TestPointEarning_NonPreferredHigherPoints: 비선호 카테고리는 선호보다 더 많은 포인트를 획득합니다.
func TestPointEarning_NonPreferredHigherPoints(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, "point_logs", "user_points", "context_items", "collection_runs", "users")

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
	svc := level.NewService(levelRepo)

	// 비선호 카테고리로 포인트 적립
	result, err := svc.EarnPoint(ctx, userID, runID, itemID, false)
	if err != nil {
		t.Fatalf("EarnPoint error: %v", err)
	}

	expectedPoints := level.CalcPoints(false) // 비선호 기본 10pt
	if result.PointsEarned != expectedPoints {
		t.Errorf("expected %d points for non-preferred, got %d", expectedPoints, result.PointsEarned)
	}

	// 비선호 > 선호 포인트 확인
	preferredPoints := level.CalcPoints(true)
	if result.PointsEarned <= preferredPoints {
		t.Errorf("non-preferred points (%d) should be greater than preferred (%d)", result.PointsEarned, preferredPoints)
	}

	t.Logf("✓ NonPreferredHigherPoints passed: non-pref=%dpt, pref=%dpt", result.PointsEarned, preferredPoints)
}

// TestPointEarning_IsRunCreatedToday_OldRun: 과거에 생성된 run이면 false를 반환합니다.
func TestPointEarning_IsRunCreatedToday_OldRun(t *testing.T) {
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

	t.Log("✓ IsRunCreatedToday_OldRun passed: correctly returned false for old run")
}

// TestPointEarning_LevelUp: 충분한 포인트 적립 시 레벨업 감지를 검증합니다.
func TestPointEarning_LevelUp(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, "point_logs", "user_points", "context_items", "collection_runs", "users")

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
	svc := level.NewService(levelRepo)

	// 레벨 1→2 경계(50pt)에 가깝게 사전 설정 (비선호 10pt → 40+10=50 → lv2)
	// level.Thresholds 기준: 0=lv1, 50=lv2, 200=lv3, 500=lv4, 1000=lv5
	if err := levelRepo.SetPoints(ctx, userID, 40); err != nil {
		t.Fatalf("SetPoints error: %v", err)
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

	// 비선호(10pt) 클릭 → 40+10=50pt → lv2
	result, err := svc.EarnPoint(ctx, userID, runID, itemID, false)
	if err != nil {
		t.Fatalf("EarnPoint error: %v", err)
	}

	if !result.Earned {
		t.Error("expected earn to succeed")
	}

	if !result.LeveledUp {
		t.Errorf("expected level up, got level=%d totalPoints=%d", result.Level, result.TotalPoints)
	}

	if result.Level < 2 {
		t.Errorf("expected level >= 2 after levelup, got %d", result.Level)
	}

	t.Logf("✓ LevelUp passed: lv%d, totalPts=%d, leveledUp=%v", result.Level, result.TotalPoints, result.LeveledUp)
}
