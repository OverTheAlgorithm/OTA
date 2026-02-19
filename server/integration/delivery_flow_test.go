package integration

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"ota/domain/delivery"
	"ota/platform/email"
	"ota/storage"
)

// TestDeliveryFlow_EndToEnd tests the complete email delivery flow
func TestDeliveryFlow_EndToEnd(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, "delivery_logs", "context_items", "collection_runs", "user_subscriptions", "user_delivery_channels", "users")

	ctx := context.Background()

	// 1. Create test user
	var userID string
	err := db.Pool.QueryRow(ctx, `
		INSERT INTO users (kakao_id, email, nickname)
		VALUES (123, 'test@example.com', 'Test User')
		RETURNING id
	`).Scan(&userID)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// 2. Enable delivery for user
	_, err = db.Pool.Exec(ctx, `
		INSERT INTO user_delivery_channels (id, user_id, channel, enabled)
		VALUES (gen_random_uuid(), $1, 'email', true)
	`, userID)
	if err != nil {
		t.Fatalf("failed to create user preferences: %v", err)
	}

	// 3. Add subscription
	_, err = db.Pool.Exec(ctx, `
		INSERT INTO user_subscriptions (user_id, category)
		VALUES ($1, 'entertainment')
	`, userID)
	if err != nil {
		t.Fatalf("failed to create subscription: %v", err)
	}

	// 4. Create collection run
	runID := uuid.New()
	_, err = db.Pool.Exec(ctx, `
		INSERT INTO collection_runs (id, status, started_at, completed_at)
		VALUES ($1, 'success', $2, $3)
	`, runID, time.Now().UTC(), time.Now().UTC())
	if err != nil {
		t.Fatalf("failed to create collection run: %v", err)
	}

	// 5. Create context items
	_, err = db.Pool.Exec(ctx, `
		INSERT INTO context_items (collection_run_id, category, rank, topic, summary, sources)
		VALUES
			($1, 'top', 1, '주요 이슈 1', '첫 번째 주요 이슈입니다.', '["source1"]'::jsonb),
			($1, 'top', 2, '주요 이슈 2', '두 번째 주요 이슈입니다.', '["source2"]'::jsonb),
			($1, 'entertainment', 1, '연예 소식', '연예 관련 소식입니다.', '["source3"]'::jsonb)
	`, runID)
	if err != nil {
		t.Fatalf("failed to create context items: %v", err)
	}

	// 6. Setup delivery service with mock email sender
	mockEmailSender := email.NewMockSender()
	deliveryRepo := storage.NewDeliveryRepository(db.Pool)
	collectorAdapter := storage.NewCollectorServiceAdapter(db.Pool)
	deliveryService := delivery.NewService(deliveryRepo, mockEmailSender, collectorAdapter)

	// 7. Execute delivery
	result, err := deliveryService.DeliverAll(ctx)
	if err != nil {
		t.Fatalf("delivery failed: %v", err)
	}

	// 8. Verify results
	if result.TotalUsers != 1 {
		t.Errorf("expected 1 total user, got %d", result.TotalUsers)
	}

	if result.SuccessCount != 1 {
		t.Errorf("expected 1 success, got %d", result.SuccessCount)
	}

	if result.FailureCount != 0 {
		t.Errorf("expected 0 failures, got %d", result.FailureCount)
	}

	if result.SkippedCount != 0 {
		t.Errorf("expected 0 skipped, got %d", result.SkippedCount)
	}

	// 9. Verify email was "sent"
	if mockEmailSender.GetSentCount() != 1 {
		t.Fatalf("expected 1 email sent, got %d", mockEmailSender.GetSentCount())
	}

	sentEmail := mockEmailSender.GetLastSent()
	if sentEmail == nil {
		t.Fatal("no email was sent")
	}

	// 10. Verify email content
	if sentEmail.To != "test@example.com" {
		t.Errorf("expected recipient 'test@example.com', got '%s'", sentEmail.To)
	}

	if sentEmail.Subject == "" {
		t.Error("subject should not be empty")
	}

	if sentEmail.TextBody == "" {
		t.Error("text body should not be empty")
	}

	if sentEmail.HTMLBody == "" {
		t.Error("HTML body should not be empty")
	}

	// Verify message contains expected topics
	if !contains(sentEmail.TextBody, "주요 이슈 1") {
		t.Error("text body should contain '주요 이슈 1'")
	}

	if !contains(sentEmail.TextBody, "연예 소식") {
		t.Error("text body should contain subscribed '연예 소식'")
	}

	// 11. Verify delivery log was created
	var logCount int
	err = db.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM delivery_logs
		WHERE user_id = $1 AND run_id = $2 AND status = 'sent'
	`, userID, runID).Scan(&logCount)
	if err != nil {
		t.Fatalf("failed to query delivery logs: %v", err)
	}

	if logCount != 1 {
		t.Errorf("expected 1 delivery log, got %d", logCount)
	}

	t.Log("✓ End-to-end delivery flow test passed")
	t.Logf("  - Email sent to: %s", sentEmail.To)
	t.Logf("  - Subject: %s", sentEmail.Subject)
	t.Logf("  - Text body length: %d chars", len(sentEmail.TextBody))
	t.Logf("  - HTML body length: %d chars", len(sentEmail.HTMLBody))
}

// TestDeliveryFlow_Idempotency verifies duplicate delivery is prevented
func TestDeliveryFlow_Idempotency(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, "delivery_logs", "context_items", "collection_runs", "user_subscriptions", "user_delivery_channels", "users")

	ctx := context.Background()

	// Setup test data (same as above)
	var userID string
	err := db.Pool.QueryRow(ctx, `
		INSERT INTO users (kakao_id, email, nickname)
		VALUES (123, 'test@example.com', 'Test User')
		RETURNING id
	`).Scan(&userID)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	_, err = db.Pool.Exec(ctx, `
		INSERT INTO user_delivery_channels (id, user_id, channel, enabled)
		VALUES (gen_random_uuid(), $1, 'email', true)
	`, userID)
	if err != nil {
		t.Fatalf("failed to create preferences: %v", err)
	}

	runID := uuid.New()
	_, err = db.Pool.Exec(ctx, `
		INSERT INTO collection_runs (id, status, started_at, completed_at)
		VALUES ($1, 'success', $2, $3)
	`, runID, time.Now().UTC(), time.Now().UTC())
	if err != nil {
		t.Fatalf("failed to create run: %v", err)
	}

	_, err = db.Pool.Exec(ctx, `
		INSERT INTO context_items (collection_run_id, category, rank, topic, summary, sources)
		VALUES ($1, 'top', 1, '주제', '요약', '["source"]'::jsonb)
	`, runID)
	if err != nil {
		t.Fatalf("failed to create items: %v", err)
	}

	// Setup service
	mockEmailSender := email.NewMockSender()
	deliveryRepo := storage.NewDeliveryRepository(db.Pool)
	collectorAdapter := storage.NewCollectorServiceAdapter(db.Pool)
	deliveryService := delivery.NewService(deliveryRepo, mockEmailSender, collectorAdapter)

	// First delivery
	result1, err := deliveryService.DeliverAll(ctx)
	if err != nil {
		t.Fatalf("first delivery failed: %v", err)
	}

	if result1.SuccessCount != 1 {
		t.Errorf("first delivery: expected 1 success, got %d", result1.SuccessCount)
	}

	// Second delivery (should be skipped)
	result2, err := deliveryService.DeliverAll(ctx)
	if err != nil {
		t.Fatalf("second delivery failed: %v", err)
	}

	if result2.SkippedCount != 1 {
		t.Errorf("second delivery: expected 1 skipped, got %d", result2.SkippedCount)
	}

	if result2.SuccessCount != 0 {
		t.Errorf("second delivery: expected 0 success, got %d", result2.SuccessCount)
	}

	// Verify only 1 email was sent total
	if mockEmailSender.GetSentCount() != 1 {
		t.Errorf("expected 1 email sent total, got %d", mockEmailSender.GetSentCount())
	}

	t.Log("✓ Idempotency test passed - duplicate delivery prevented")
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) >= len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
