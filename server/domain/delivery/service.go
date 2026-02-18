package delivery

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"ota/domain/collector"
	"ota/platform/email"
)

// Service orchestrates message delivery
type Service struct {
	repo         Repository
	emailSender  email.Sender
	collectorSvc CollectorService
}

// CollectorService defines the interface for fetching context items
type CollectorService interface {
	GetLatestRun(ctx context.Context) (*collector.CollectionRun, error)
	GetContextItems(ctx context.Context, runID uuid.UUID) ([]collector.ContextItem, error)
}

// NewService creates a new delivery service
func NewService(repo Repository, emailSender email.Sender, collectorSvc CollectorService) *Service {
	return &Service{
		repo:         repo,
		emailSender:  emailSender,
		collectorSvc: collectorSvc,
	}
}

// DeliveryResult represents the outcome of a delivery operation
type DeliveryResult struct {
	TotalUsers     int
	SuccessCount   int
	FailureCount   int
	SkippedCount   int
	FailedUsers    []string
	DeliveryErrors map[string]string
}

// DeliverAll sends messages to all eligible users
func (s *Service) DeliverAll(ctx context.Context) (*DeliveryResult, error) {
	// 1. Get latest collection run
	run, err := s.collectorSvc.GetLatestRun(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest run: %w", err)
	}

	if run.Status != collector.RunStatusSuccess {
		return nil, fmt.Errorf("latest run is not completed (status: %s)", run.Status)
	}

	// 2. Get context items
	items, err := s.collectorSvc.GetContextItems(ctx, run.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get context items: %w", err)
	}

	if len(items) == 0 {
		return &DeliveryResult{}, nil
	}

	// 3. Get eligible users
	users, err := s.repo.GetEligibleUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get eligible users: %w", err)
	}

	// 4. Deliver to each user
	result := &DeliveryResult{
		TotalUsers:     len(users),
		FailedUsers:    []string{},
		DeliveryErrors: make(map[string]string),
	}

	for _, user := range users {
		// Format message once per user (same content for all channels)
		message := FormatMessage(items, user.Subscriptions)

		// Deliver to each enabled channel for this user
		for _, channel := range user.EnabledChannels {
			// Check idempotency per channel
			alreadySent, err := s.repo.HasDeliveryLog(ctx, run.ID.String(), user.UserID, channel)
			if err != nil {
				result.FailureCount++
				result.FailedUsers = append(result.FailedUsers, user.UserID)
				result.DeliveryErrors[fmt.Sprintf("%s_%s", user.UserID, channel)] = fmt.Sprintf("failed to check delivery log: %v", err)
				continue
			}

			if alreadySent {
				result.SkippedCount++
				s.logDelivery(ctx, run.ID.String(), user.UserID, channel, StatusSkipped, "already sent")
				continue
			}

			// Send via appropriate channel
			err = s.sendViaChannel(ctx, channel, user, message)

			if err != nil {
				result.FailureCount++
				result.FailedUsers = append(result.FailedUsers, user.UserID)
				result.DeliveryErrors[fmt.Sprintf("%s_%s", user.UserID, channel)] = err.Error()
				s.logDelivery(ctx, run.ID.String(), user.UserID, channel, StatusFailed, err.Error())
			} else {
				result.SuccessCount++
				s.logDelivery(ctx, run.ID.String(), user.UserID, channel, StatusSent, "")
			}
		}
	}

	return result, nil
}

func (s *Service) logDelivery(ctx context.Context, runID string, userID string, channel DeliveryChannel, status DeliveryStatus, errorMsg string) {
	log := DeliveryLog{
		ID:           uuid.New().String(),
		RunID:        runID,
		UserID:       userID,
		Channel:      channel,
		Status:       status,
		ErrorMessage: errorMsg,
		CreatedAt:    time.Now().UTC(),
	}

	// Log errors but don't fail delivery on logging failure
	if err := s.repo.LogDelivery(ctx, log); err != nil {
		// TODO: Add proper logging
		fmt.Printf("WARNING: failed to log delivery for user %s via %s: %v\n", userID, channel, err)
	}
}

// sendViaChannel sends a message via the specified delivery channel
func (s *Service) sendViaChannel(ctx context.Context, channel DeliveryChannel, user EligibleUser, message FormattedMessage) error {
	switch channel {
	case ChannelEmail:
		if user.Email == "" {
			return fmt.Errorf("user has no email address")
		}
		return s.emailSender.Send(user.Email, message.Subject, message.TextBody, message.HTMLBody)

	case ChannelKakao:
		// TODO: Implement Kakao Talk delivery
		return fmt.Errorf("kakao channel not yet implemented")

	case ChannelTelegram:
		// TODO: Implement Telegram delivery
		return fmt.Errorf("telegram channel not yet implemented")

	case ChannelSMS:
		// TODO: Implement SMS delivery
		return fmt.Errorf("sms channel not yet implemented")

	case ChannelPush:
		// TODO: Implement Push notification delivery
		return fmt.Errorf("push channel not yet implemented")

	default:
		return fmt.Errorf("unknown delivery channel: %s", channel)
	}
}
