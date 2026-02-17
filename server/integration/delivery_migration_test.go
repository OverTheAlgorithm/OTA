package integration

import (
	"context"
	"testing"
)

// TestDeliveryTables_Migration verifies the delivery tables migration applies correctly
func TestDeliveryTables_Migration(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, "delivery_logs", "user_subscriptions", "user_preferences")

	ctx := context.Background()

	// Verify user_preferences table exists
	var prefsExists bool
	err := db.Pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables
			WHERE table_name = 'user_preferences'
		)
	`).Scan(&prefsExists)
	if err != nil {
		t.Fatalf("failed to check user_preferences table: %v", err)
	}
	if !prefsExists {
		t.Error("user_preferences table does not exist")
	}

	// Verify user_subscriptions table exists
	var subsExists bool
	err = db.Pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables
			WHERE table_name = 'user_subscriptions'
		)
	`).Scan(&subsExists)
	if err != nil {
		t.Fatalf("failed to check user_subscriptions table: %v", err)
	}
	if !subsExists {
		t.Error("user_subscriptions table does not exist")
	}

	// Verify delivery_logs table exists
	var logsExists bool
	err = db.Pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables
			WHERE table_name = 'delivery_logs'
		)
	`).Scan(&logsExists)
	if err != nil {
		t.Fatalf("failed to check delivery_logs table: %v", err)
	}
	if !logsExists {
		t.Error("delivery_logs table does not exist")
	}

	t.Log("✓ All delivery tables exist")
}

// TestDeliveryTables_Constraints verifies foreign keys and unique constraints
func TestDeliveryTables_Constraints(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, "delivery_logs", "user_subscriptions", "user_preferences")

	ctx := context.Background()

	// Create a test user first
	var userID string
	err := db.Pool.QueryRow(ctx, `
		INSERT INTO users (kakao_id, email, nickname)
		VALUES (123456789, 'test@example.com', 'TestUser')
		RETURNING id
	`).Scan(&userID)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	// Test user_preferences
	_, err = db.Pool.Exec(ctx, `
		INSERT INTO user_preferences (user_id, delivery_enabled)
		VALUES ($1, true)
	`, userID)
	if err != nil {
		t.Fatalf("failed to insert user_preferences: %v", err)
	}

	// Test user_subscriptions with unique constraint
	_, err = db.Pool.Exec(ctx, `
		INSERT INTO user_subscriptions (user_id, category)
		VALUES ($1, 'entertainment')
	`, userID)
	if err != nil {
		t.Fatalf("failed to insert user_subscription: %v", err)
	}

	// Test duplicate category should fail
	_, err = db.Pool.Exec(ctx, `
		INSERT INTO user_subscriptions (user_id, category)
		VALUES ($1, 'entertainment')
	`, userID)
	if err == nil {
		t.Error("expected duplicate subscription to fail, but it succeeded")
	}

	// Test delivery_logs requires a collection run
	var runID string
	err = db.Pool.QueryRow(ctx, `
		INSERT INTO collection_runs (status)
		VALUES ('completed')
		RETURNING id
	`).Scan(&runID)
	if err != nil {
		t.Fatalf("failed to create test collection run: %v", err)
	}

	_, err = db.Pool.Exec(ctx, `
		INSERT INTO delivery_logs (run_id, user_id, channel, status)
		VALUES ($1, $2, 'email', 'sent')
	`, runID, userID)
	if err != nil {
		t.Fatalf("failed to insert delivery_log: %v", err)
	}

	// Test duplicate delivery should fail (idempotency)
	_, err = db.Pool.Exec(ctx, `
		INSERT INTO delivery_logs (run_id, user_id, channel, status)
		VALUES ($1, $2, 'email', 'sent')
	`, runID, userID)
	if err == nil {
		t.Error("expected duplicate delivery to fail, but it succeeded")
	}

	t.Log("✓ All constraints work correctly")
}
