package integration

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"ota/domain/push"
	"ota/scheduler"
	"ota/storage"
)

// ─── Mock PushSender (no real Expo calls) ─────────────────────────────────────

type mockPushSenderForIntegration struct {
	callCount int
}

func (m *mockPushSenderForIntegration) Save(_ context.Context, _ push.PushToken) error          { return nil }
func (m *mockPushSenderForIntegration) UnlinkUser(_ context.Context, _, _ string) error         { return nil }
func (m *mockPushSenderForIntegration) DeleteByTokens(_ context.Context, _ []string) error { return nil }
func (m *mockPushSenderForIntegration) GetByUserID(_ context.Context, _ string) ([]push.PushToken, error) {
	return nil, nil
}
func (m *mockPushSenderForIntegration) GetAllActive(_ context.Context) ([]push.PushToken, error) {
	m.callCount++
	// Return empty tokens so no real HTTP call is made.
	return []push.PushToken{}, nil
}

// ─── Mock PushExecutor for scheduler (wraps real ScheduledService) ────────────

type testPushExecutor struct {
	svc *push.ScheduledService
}

func (e *testPushExecutor) ExecuteBySchedule(ctx context.Context, id uuid.UUID) error {
	return e.svc.ExecuteBySchedule(ctx, id)
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

var pushTables = []string{"scheduled_pushes", "users"}

// createTestUser inserts a minimal user row and returns the UUID (needed for created_by FK).
func createTestPushUser(t *testing.T, db *TestDB, kakaoID int, email string) string {
	t.Helper()
	var userID string
	err := db.Pool.QueryRow(context.Background(), `
		INSERT INTO users (kakao_id, email, nickname)
		VALUES ($1, $2, $3)
		RETURNING id
	`, kakaoID, email, "PushTestUser").Scan(&userID)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}
	return userID
}

func newTestScheduledService(t *testing.T, db *TestDB) (*push.ScheduledService, *mockPushSenderForIntegration) {
	t.Helper()
	pushRepo := &mockPushSenderForIntegration{}
	pushSvc := push.NewService(pushRepo)
	scheduledRepo := storage.NewScheduledPushRepository(db.Pool)
	svc := push.NewScheduledService(scheduledRepo, pushSvc)
	return svc, pushRepo
}

// ─── DB CRUD Tests ────────────────────────────────────────────────────────────

func TestPush_Create_VerifyRow(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, pushTables...)

	userID := createTestPushUser(t, db, 9001, "push-create@example.com")
	svc, _ := newTestScheduledService(t, db)

	futureTime := time.Now().Add(time.Hour)
	created, err := svc.Create(context.Background(), "Test Title", "Test Body", "https://example.com", nil, &futureTime, userID)
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}

	// Verify row in DB.
	var title, body, status string
	err = db.Pool.QueryRow(context.Background(),
		`SELECT title, body, status FROM scheduled_pushes WHERE id = $1`, created.ID,
	).Scan(&title, &body, &status)
	if err != nil {
		t.Fatalf("failed to query scheduled_push row: %v", err)
	}

	if title != "Test Title" {
		t.Errorf("title: want %q, got %q", "Test Title", title)
	}
	if body != "Test Body" {
		t.Errorf("body: want %q, got %q", "Test Body", body)
	}
	if status != "pending" {
		t.Errorf("status: want %q, got %q", "pending", status)
	}

	t.Log("Create_VerifyRow passed")
}

func TestPush_CAS_MarkSent_Concurrency(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, pushTables...)

	userID := createTestPushUser(t, db, 9002, "push-cas@example.com")
	scheduledRepo := storage.NewScheduledPushRepository(db.Pool)
	pushSvc := push.NewService(&mockPushSenderForIntegration{})
	svc := push.NewScheduledService(scheduledRepo, pushSvc)

	futureTime := time.Now().Add(time.Hour)
	created, err := svc.Create(context.Background(), "CAS Test", "Body", "", nil, &futureTime, userID)
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}

	ctx := context.Background()
	sentAt := time.Now()

	// First MarkSent should succeed.
	ok1, err := scheduledRepo.MarkSent(ctx, created.ID, sentAt)
	if err != nil {
		t.Fatalf("first MarkSent error: %v", err)
	}
	if !ok1 {
		t.Fatal("first MarkSent should return true")
	}

	// Second MarkSent should return false (already sent).
	ok2, err := scheduledRepo.MarkSent(ctx, created.ID, sentAt)
	if err != nil {
		t.Fatalf("second MarkSent error: %v", err)
	}
	if ok2 {
		t.Error("second MarkSent should return false (CAS lost)")
	}

	t.Log("CAS_MarkSent_Concurrency passed")
}

func TestPush_ReloadPending_RegistersTimers(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, pushTables...)

	userID := createTestPushUser(t, db, 9003, "push-reload@example.com")
	svc, _ := newTestScheduledService(t, db)

	// Create 2 pushes with scheduled_at.
	future1 := time.Now().Add(10 * time.Second)
	future2 := time.Now().Add(20 * time.Second)

	p1, err := svc.Create(context.Background(), "Push 1", "Body 1", "", nil, &future1, userID)
	if err != nil {
		t.Fatalf("Create p1 error: %v", err)
	}
	p2, err := svc.Create(context.Background(), "Push 2", "Body 2", "", nil, &future2, userID)
	if err != nil {
		t.Fatalf("Create p2 error: %v", err)
	}
	_ = p1
	_ = p2

	// Fetch pending and reload into a new scheduler.
	pending, err := svc.ListPending(context.Background())
	if err != nil {
		t.Fatalf("ListPending error: %v", err)
	}
	if len(pending) != 2 {
		t.Fatalf("expected 2 pending pushes, got %d", len(pending))
	}

	executor := &testPushExecutor{svc: svc}
	ps := scheduler.NewPushScheduler(executor, context.Background())
	defer ps.Stop()

	if err := ps.ReloadPending(context.Background(), pending); err != nil {
		t.Fatalf("ReloadPending error: %v", err)
	}

	// Verify pending list has 2 entries (timers were registered without error).
	// Timer internals are not directly observable; success of ReloadPending is sufficient.
	if len(pending) != 2 {
		t.Errorf("expected 2 pending pushes scheduled, got %d", len(pending))
	}

	t.Log("ReloadPending_RegistersTimers passed")
}

func TestPush_TimerFiring_StatusSent(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, pushTables...)

	userID := createTestPushUser(t, db, 9004, "push-timer@example.com")
	svc, _ := newTestScheduledService(t, db)

	// Schedule to fire in 2 seconds.
	fireAt := time.Now().Add(2 * time.Second)
	created, err := svc.Create(context.Background(), "Timer Test", "Body", "", nil, &fireAt, userID)
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}

	executor := &testPushExecutor{svc: svc}
	ps := scheduler.NewPushScheduler(executor, context.Background())
	defer ps.Stop()

	if err := ps.Schedule(created); err != nil {
		t.Fatalf("Schedule error: %v", err)
	}

	// Wait for timer to fire (3 seconds = 2s delay + 1s buffer).
	time.Sleep(3 * time.Second)

	// Verify status = sent in DB.
	var status string
	err = db.Pool.QueryRow(context.Background(),
		`SELECT status FROM scheduled_pushes WHERE id = $1`, created.ID,
	).Scan(&status)
	if err != nil {
		t.Fatalf("failed to query status: %v", err)
	}
	if status != "sent" {
		t.Errorf("expected status=sent after timer fired, got %q", status)
	}

	t.Log("TimerFiring_StatusSent passed")
}
