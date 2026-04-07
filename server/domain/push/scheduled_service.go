package push

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ScheduledService handles CRUD and execution of scheduled push notifications.
type ScheduledService struct {
	repo        ScheduledRepository
	pushService *Service
}

// NewScheduledService creates a new ScheduledService.
func NewScheduledService(repo ScheduledRepository, pushService *Service) *ScheduledService {
	return &ScheduledService{repo: repo, pushService: pushService}
}

// Create validates and persists a new scheduled push notification.
// Scheduling in PushScheduler is the caller's responsibility.
func (s *ScheduledService) Create(ctx context.Context, title, body, link string, data map[string]any, scheduledAt *time.Time, createdBy string) (ScheduledPush, error) {
	if title == "" {
		return ScheduledPush{}, fmt.Errorf("title is required")
	}
	if body == "" {
		return ScheduledPush{}, fmt.Errorf("body is required")
	}

	p := ScheduledPush{
		ID:          uuid.New(),
		Title:       title,
		Body:        body,
		Link:        link,
		Data:        data,
		Status:      StatusPending,
		ScheduledAt: scheduledAt,
		CreatedBy:   createdBy,
	}

	created, err := s.repo.Create(ctx, p)
	if err != nil {
		return ScheduledPush{}, fmt.Errorf("create scheduled push: %w", err)
	}
	return created, nil
}

// Update validates and updates a pending scheduled push notification.
func (s *ScheduledService) Update(ctx context.Context, id uuid.UUID, title, body, link string, data map[string]any, scheduledAt *time.Time) (ScheduledPush, error) {
	if title == "" {
		return ScheduledPush{}, fmt.Errorf("title is required")
	}
	if body == "" {
		return ScheduledPush{}, fmt.Errorf("body is required")
	}

	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return ScheduledPush{}, fmt.Errorf("get scheduled push: %w", err)
	}
	if existing.Status != StatusPending {
		return ScheduledPush{}, fmt.Errorf("push notification is not in pending status")
	}

	updated := ScheduledPush{
		ID:          existing.ID,
		Title:       title,
		Body:        body,
		Link:        link,
		Data:        data,
		Status:      existing.Status,
		ScheduledAt: scheduledAt,
		SentAt:      existing.SentAt,
		CreatedBy:   existing.CreatedBy,
		CreatedAt:   existing.CreatedAt,
	}

	if err := s.repo.Update(ctx, updated); err != nil {
		return ScheduledPush{}, fmt.Errorf("update scheduled push: %w", err)
	}
	return updated, nil
}

// Delete soft-deletes a push by marking it as cancelled (CAS on pending status).
func (s *ScheduledService) Delete(ctx context.Context, id uuid.UUID) error {
	updated, err := s.repo.MarkCancelled(ctx, id)
	if err != nil {
		return fmt.Errorf("cancel scheduled push: %w", err)
	}
	if !updated {
		return fmt.Errorf("push notification is not in pending status")
	}
	return nil
}

// List returns push notifications ordered by created_at DESC, with an optional status filter.
func (s *ScheduledService) List(ctx context.Context, status *string) ([]ScheduledPush, error) {
	pushes, err := s.repo.List(ctx, status)
	if err != nil {
		return nil, fmt.Errorf("list scheduled pushes: %w", err)
	}
	return pushes, nil
}

// ListPending returns all pending pushes that have a scheduled_at set (for scheduler reload on startup).
func (s *ScheduledService) ListPending(ctx context.Context) ([]ScheduledPush, error) {
	pushes, err := s.repo.ListPending(ctx)
	if err != nil {
		return nil, fmt.Errorf("list pending scheduled pushes: %w", err)
	}
	return pushes, nil
}

// ExecuteNow sends to the requesting admin's devices only (for testing).
// CAS-marks the push as sent first, then calls SendToUser only if CAS succeeds.
func (s *ScheduledService) ExecuteNow(ctx context.Context, id uuid.UUID, adminUserID string) error {
	return s.execute(ctx, id, &adminUserID)
}

// ExecuteBySchedule sends to ALL users (production delivery).
// The CAS guarantees only one caller wins, preventing double-send.
func (s *ScheduledService) ExecuteBySchedule(ctx context.Context, id uuid.UUID) error {
	return s.execute(ctx, id, nil)
}

func (s *ScheduledService) execute(ctx context.Context, id uuid.UUID, targetUserID *string) error {
	push, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("get scheduled push for execution: %w", err)
	}

	sentAt := time.Now()
	won, err := s.repo.MarkSent(ctx, id, sentAt)
	if err != nil {
		return fmt.Errorf("mark push as sent: %w", err)
	}
	if !won {
		// Another caller already processed this push (CAS lost).
		return nil
	}

	// Build data map with link included.
	data := make(map[string]any)
	for k, v := range push.Data {
		data[k] = v
	}
	if push.Link != "" {
		data["link"] = push.Link
	}

	var sendErr error
	if targetUserID != nil {
		// Send to specific user only (admin test send)
		sendErr = s.pushService.SendToUser(ctx, *targetUserID, push.Title, push.Body, data)
	} else {
		// Send to all users (scheduled delivery)
		sendErr = s.pushService.SendToAll(ctx, push.Title, push.Body, data)
	}

	if sendErr != nil {
		_, _ = s.repo.MarkFailed(ctx, id, sendErr.Error())
		return fmt.Errorf("send push notification: %w", sendErr)
	}
	return nil
}
