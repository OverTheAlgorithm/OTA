package integration

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"ota/domain/delivery"
	"ota/storage"
)

func TestDeliveryRepository_GetEligibleUsers_Empty(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, "delivery_logs", "user_subscriptions", "user_preferences", "users")

	repo := storage.NewDeliveryRepository(db.Pool)

	users, err := repo.GetEligibleUsers(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(users) != 0 {
		t.Errorf("expected 0 users, got %d", len(users))
	}
}

func TestDeliveryRepository_GetEligibleUsers_WithPreferences(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, "delivery_logs", "user_subscriptions", "user_preferences", "users")

	ctx := context.Background()

	// Create test users
	var user1ID, user2ID, user3ID string

	err := db.Pool.QueryRow(ctx, `
		INSERT INTO users (kakao_id, email, nickname)
		VALUES (123, 'user1@example.com', 'User1')
		RETURNING id
	`).Scan(&user1ID)
	if err != nil {
		t.Fatalf("failed to create user1: %v", err)
	}

	err = db.Pool.QueryRow(ctx, `
		INSERT INTO users (kakao_id, email, nickname)
		VALUES (456, 'user2@example.com', 'User2')
		RETURNING id
	`).Scan(&user2ID)
	if err != nil {
		t.Fatalf("failed to create user2: %v", err)
	}

	err = db.Pool.QueryRow(ctx, `
		INSERT INTO users (kakao_id, email, nickname)
		VALUES (789, 'user3@example.com', 'User3')
		RETURNING id
	`).Scan(&user3ID)
	if err != nil {
		t.Fatalf("failed to create user3: %v", err)
	}

	// Set preferences
	_, err = db.Pool.Exec(ctx, `
		INSERT INTO user_preferences (user_id, delivery_enabled)
		VALUES ($1, true), ($2, false), ($3, true)
	`, user1ID, user2ID, user3ID)
	if err != nil {
		t.Fatalf("failed to set preferences: %v", err)
	}

	// Add subscriptions for user1
	_, err = db.Pool.Exec(ctx, `
		INSERT INTO user_subscriptions (user_id, category)
		VALUES ($1, 'entertainment'), ($1, 'sports')
	`, user1ID)
	if err != nil {
		t.Fatalf("failed to add subscriptions: %v", err)
	}

	// Query eligible users
	repo := storage.NewDeliveryRepository(db.Pool)
	users, err := repo.GetEligibleUsers(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should return user1 and user3 (delivery_enabled=true)
	// user2 should be excluded (delivery_enabled=false)
	if len(users) != 2 {
		t.Fatalf("expected 2 eligible users, got %d", len(users))
	}

	// Find user1
	var foundUser1 *delivery.EligibleUser
	for i := range users {
		if users[i].UserID == user1ID {
			foundUser1 = &users[i]
			break
		}
	}

	if foundUser1 == nil {
		t.Fatal("user1 not found in eligible users")
	}

	if foundUser1.Email != "user1@example.com" {
		t.Errorf("expected email 'user1@example.com', got '%s'", foundUser1.Email)
	}

	if len(foundUser1.Subscriptions) != 2 {
		t.Fatalf("expected 2 subscriptions, got %d", len(foundUser1.Subscriptions))
	}

	// Verify subscriptions (order doesn't matter)
	subSet := make(map[string]bool)
	for _, sub := range foundUser1.Subscriptions {
		subSet[sub] = true
	}

	if !subSet["entertainment"] || !subSet["sports"] {
		t.Errorf("expected subscriptions [entertainment, sports], got %v", foundUser1.Subscriptions)
	}
}

func TestDeliveryRepository_LogDelivery(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, "delivery_logs", "collection_runs", "users")

	ctx := context.Background()

	// Create test user
	var userID string
	err := db.Pool.QueryRow(ctx, `
		INSERT INTO users (kakao_id, email, nickname)
		VALUES (123, 'user@example.com', 'User')
		RETURNING id
	`).Scan(&userID)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// Create collection run
	var runID string
	err = db.Pool.QueryRow(ctx, `
		INSERT INTO collection_runs (status)
		VALUES ('success')
		RETURNING id
	`).Scan(&runID)
	if err != nil {
		t.Fatalf("failed to create collection run: %v", err)
	}

	// Log delivery
	repo := storage.NewDeliveryRepository(db.Pool)
	log := delivery.DeliveryLog{
		ID:           uuid.New().String(),
		RunID:        runID,
		UserID:       userID,
		Channel:      delivery.ChannelEmail,
		Status:       delivery.StatusSent,
		ErrorMessage: "",
		CreatedAt:    time.Now().UTC(),
	}

	err = repo.LogDelivery(ctx, log)
	if err != nil {
		t.Fatalf("failed to log delivery: %v", err)
	}

	// Verify log was created
	var count int
	err = db.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM delivery_logs
		WHERE run_id = $1 AND user_id = $2 AND channel = $3
	`, runID, userID, delivery.ChannelEmail).Scan(&count)
	if err != nil {
		t.Fatalf("failed to query delivery logs: %v", err)
	}

	if count != 1 {
		t.Errorf("expected 1 delivery log, got %d", count)
	}
}

func TestDeliveryRepository_HasDeliveryLog(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, "delivery_logs", "collection_runs", "users")

	ctx := context.Background()

	// Create test user
	var userID string
	err := db.Pool.QueryRow(ctx, `
		INSERT INTO users (kakao_id, email, nickname)
		VALUES (123, 'user@example.com', 'User')
		RETURNING id
	`).Scan(&userID)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// Create collection run
	var runID string
	err = db.Pool.QueryRow(ctx, `
		INSERT INTO collection_runs (status)
		VALUES ('success')
		RETURNING id
	`).Scan(&runID)
	if err != nil {
		t.Fatalf("failed to create collection run: %v", err)
	}

	repo := storage.NewDeliveryRepository(db.Pool)

	// Check before logging (should be false)
	exists, err := repo.HasDeliveryLog(ctx, runID, userID, delivery.ChannelEmail)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if exists {
		t.Error("expected no delivery log, but found one")
	}

	// Log delivery
	log := delivery.DeliveryLog{
		ID:           uuid.New().String(),
		RunID:        runID,
		UserID:       userID,
		Channel:      delivery.ChannelEmail,
		Status:       delivery.StatusSent,
		ErrorMessage: "",
		CreatedAt:    time.Now().UTC(),
	}

	err = repo.LogDelivery(ctx, log)
	if err != nil {
		t.Fatalf("failed to log delivery: %v", err)
	}

	// Check after logging (should be true)
	exists, err = repo.HasDeliveryLog(ctx, runID, userID, delivery.ChannelEmail)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !exists {
		t.Error("expected delivery log to exist, but it doesn't")
	}
}

func TestDeliveryRepository_GetEligibleUsers_NoEmail(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, "user_preferences", "users")

	ctx := context.Background()

	// Create user without email
	var userID string
	err := db.Pool.QueryRow(ctx, `
		INSERT INTO users (kakao_id, nickname)
		VALUES (123, 'NoEmailUser')
		RETURNING id
	`).Scan(&userID)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// Enable delivery
	_, err = db.Pool.Exec(ctx, `
		INSERT INTO user_preferences (user_id, delivery_enabled)
		VALUES ($1, true)
	`, userID)
	if err != nil {
		t.Fatalf("failed to set preferences: %v", err)
	}

	// Query eligible users
	repo := storage.NewDeliveryRepository(db.Pool)
	users, err := repo.GetEligibleUsers(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should return 0 users (no email)
	if len(users) != 0 {
		t.Errorf("expected 0 users without email, got %d", len(users))
	}
}
