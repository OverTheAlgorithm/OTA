package delivery

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"

	"ota/domain/collector"
	"ota/platform/email"
)

// Mock implementations

type mockRepository struct {
	users         []EligibleUser
	deliveryLogs  []DeliveryLog
	hasLogResults map[string]bool
	shouldFail    bool
}

func (m *mockRepository) GetEligibleUsers(ctx context.Context) ([]EligibleUser, error) {
	if m.shouldFail {
		return nil, fmt.Errorf("mock repository error")
	}
	return m.users, nil
}

func (m *mockRepository) LogDelivery(ctx context.Context, log DeliveryLog) error {
	if m.shouldFail {
		return fmt.Errorf("mock log error")
	}
	m.deliveryLogs = append(m.deliveryLogs, log)
	return nil
}

func (m *mockRepository) HasDeliveryLog(ctx context.Context, runID string, userID string, channel DeliveryChannel) (bool, error) {
	if m.shouldFail {
		return false, fmt.Errorf("mock has log error")
	}
	key := fmt.Sprintf("%s:%s:%s", runID, userID, channel)
	return m.hasLogResults[key], nil
}

type mockCollectorService struct {
	run   *collector.CollectionRun
	items []collector.ContextItem
}

func (m *mockCollectorService) GetLatestRun(ctx context.Context) (*collector.CollectionRun, error) {
	if m.run == nil {
		return nil, fmt.Errorf("no run available")
	}
	return m.run, nil
}

func (m *mockCollectorService) GetContextItems(ctx context.Context, runID uuid.UUID) ([]collector.ContextItem, error) {
	return m.items, nil
}

// Tests

func TestDeliverAll_Success(t *testing.T) {
	mockRepo := &mockRepository{
		users: []EligibleUser{
			{
				UserID:        "user1",
				Email:         "user1@example.com",
				Subscriptions: []string{},
			},
			{
				UserID:        "user2",
				Email:         "user2@example.com",
				Subscriptions: []string{"entertainment"},
			},
		},
		hasLogResults: make(map[string]bool),
	}

	mockEmailSender := email.NewMockSender()

	mockCollector := &mockCollectorService{
		run: &collector.CollectionRun{
			ID:     uuid.New(),
			Status: collector.RunStatusSuccess,
		},
		items: []collector.ContextItem{
			{
				Category: "top",
				Rank:     1,
				Topic:    "주요 이슈",
				Summary:  "주요 이슈입니다.",
			},
		},
	}

	service := NewService(mockRepo, mockEmailSender, mockCollector)

	result, err := service.DeliverAll(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalUsers != 2 {
		t.Errorf("expected 2 total users, got %d", result.TotalUsers)
	}

	if result.SuccessCount != 2 {
		t.Errorf("expected 2 successful deliveries, got %d", result.SuccessCount)
	}

	if result.FailureCount != 0 {
		t.Errorf("expected 0 failures, got %d", result.FailureCount)
	}

	if mockEmailSender.GetSentCount() != 2 {
		t.Errorf("expected 2 emails sent, got %d", mockEmailSender.GetSentCount())
	}

	if len(mockRepo.deliveryLogs) != 2 {
		t.Errorf("expected 2 delivery logs, got %d", len(mockRepo.deliveryLogs))
	}
}

func TestDeliverAll_NoCompletedRun(t *testing.T) {
	mockRepo := &mockRepository{}
	mockEmailSender := email.NewMockSender()
	mockCollector := &mockCollectorService{
		run: &collector.CollectionRun{
			ID:     uuid.New(),
			Status: collector.RunStatusRunning,
		},
	}

	service := NewService(mockRepo, mockEmailSender, mockCollector)

	_, err := service.DeliverAll(context.Background())
	if err == nil {
		t.Fatal("expected error for incomplete run, got nil")
	}
}

func TestDeliverAll_NoItems(t *testing.T) {
	mockRepo := &mockRepository{
		users: []EligibleUser{
			{UserID: "user1", Email: "user1@example.com"},
		},
	}
	mockEmailSender := email.NewMockSender()
	mockCollector := &mockCollectorService{
		run: &collector.CollectionRun{
			ID:     uuid.New(),
			Status: collector.RunStatusSuccess,
		},
		items: []collector.ContextItem{},
	}

	service := NewService(mockRepo, mockEmailSender, mockCollector)

	result, err := service.DeliverAll(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalUsers != 0 {
		t.Errorf("expected 0 total users when no items, got %d", result.TotalUsers)
	}

	if mockEmailSender.GetSentCount() != 0 {
		t.Errorf("expected 0 emails sent, got %d", mockEmailSender.GetSentCount())
	}
}

func TestDeliverAll_Idempotency(t *testing.T) {
	runID := uuid.New()
	mockRepo := &mockRepository{
		users: []EligibleUser{
			{UserID: "user1", Email: "user1@example.com"},
		},
		hasLogResults: map[string]bool{
			fmt.Sprintf("%s:user1:email", runID.String()): true, // Already sent
		},
	}
	mockEmailSender := email.NewMockSender()
	mockCollector := &mockCollectorService{
		run: &collector.CollectionRun{
			ID:     runID,
			Status: collector.RunStatusSuccess,
		},
		items: []collector.ContextItem{
			{Category: "top", Rank: 1, Topic: "주제", Summary: "요약"},
		},
	}

	service := NewService(mockRepo, mockEmailSender, mockCollector)

	result, err := service.DeliverAll(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.SkippedCount != 1 {
		t.Errorf("expected 1 skipped, got %d", result.SkippedCount)
	}

	if mockEmailSender.GetSentCount() != 0 {
		t.Error("expected no emails sent due to idempotency")
	}
}

func TestDeliverAll_PartialFailure(t *testing.T) {
	mockRepo := &mockRepository{
		users: []EligibleUser{
			{UserID: "user1", Email: "user1@example.com"},
			{UserID: "user2", Email: "user2@example.com"},
		},
		hasLogResults: make(map[string]bool),
	}

	mockEmailSender := email.NewMockSender()
	// Make sender fail after first email
	mockEmailSender.ShouldFail = false

	mockCollector := &mockCollectorService{
		run: &collector.CollectionRun{
			ID:     uuid.New(),
			Status: collector.RunStatusSuccess,
		},
		items: []collector.ContextItem{
			{Category: "top", Rank: 1, Topic: "주제", Summary: "요약"},
		},
	}

	service := NewService(mockRepo, mockEmailSender, mockCollector)

	// First delivery succeeds
	result, err := service.DeliverAll(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.SuccessCount != 2 {
		t.Errorf("expected 2 successful, got %d", result.SuccessCount)
	}

	// Now make sender fail and try again with new users
	mockEmailSender.ShouldFail = true
	mockRepo.users = []EligibleUser{
		{UserID: "user3", Email: "user3@example.com"},
	}

	result2, err := service.DeliverAll(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result2.FailureCount != 1 {
		t.Errorf("expected 1 failure, got %d", result2.FailureCount)
	}

	if len(result2.FailedUsers) != 1 {
		t.Errorf("expected 1 failed user, got %d", len(result2.FailedUsers))
	}
}

func TestDeliverAll_NoUsers(t *testing.T) {
	mockRepo := &mockRepository{
		users: []EligibleUser{},
	}
	mockEmailSender := email.NewMockSender()
	mockCollector := &mockCollectorService{
		run: &collector.CollectionRun{
			ID:     uuid.New(),
			Status: collector.RunStatusSuccess,
		},
		items: []collector.ContextItem{
			{Category: "top", Rank: 1, Topic: "주제", Summary: "요약"},
		},
	}

	service := NewService(mockRepo, mockEmailSender, mockCollector)

	result, err := service.DeliverAll(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalUsers != 0 {
		t.Errorf("expected 0 users, got %d", result.TotalUsers)
	}

	if mockEmailSender.GetSentCount() != 0 {
		t.Error("expected no emails sent")
	}
}
