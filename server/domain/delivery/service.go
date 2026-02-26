package delivery

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"ota/domain/collector"
	"ota/platform/email"
)

const deliveryBatchSize = 1000

// kstLocation is KST (UTC+9).
var kstLocation = time.FixedZone("KST", 9*60*60)

// LevelProvider fetches level info and earn history per user for email rendering.
type LevelProvider interface {
	GetLevel(ctx context.Context, userID string) (UserLevelInfo, error)
	GetLastEarnedAtBatch(ctx context.Context, userIDs []string) (map[string]time.Time, error)
}

// Service orchestrates message delivery
type Service struct {
	repo          Repository
	emailSender   email.Sender
	collectorSvc  CollectorService
	brainCatRepo  collector.BrainCategoryRepository
	frontendURL   string
	levelProvider LevelProvider
}

// CollectorService defines the interface for fetching context items
type CollectorService interface {
	GetLatestRun(ctx context.Context) (*collector.CollectionRun, error)
	GetLastDeliveredRun(ctx context.Context) (*collector.CollectionRun, error)
	GetContextItems(ctx context.Context, runID uuid.UUID) ([]collector.ContextItem, error)
}

// WelcomeDeliverer sends the most recent briefing to newly registered users
type WelcomeDeliverer interface {
	DeliverToNewUser(ctx context.Context, userID string, email string) error
}

// NewService creates a new delivery service
func NewService(repo Repository, emailSender email.Sender, collectorSvc CollectorService, brainCatRepo collector.BrainCategoryRepository, frontendURL string) *Service {
	return &Service{
		repo:         repo,
		emailSender:  emailSender,
		collectorSvc: collectorSvc,
		brainCatRepo: brainCatRepo,
		frontendURL:  frontendURL,
	}
}

// WithLevelProvider sets the optional level provider for per-user level cards and point tracking.
func (s *Service) WithLevelProvider(lp LevelProvider) *Service {
	s.levelProvider = lp
	return s
}

// loadUserLevel fetches level info for a user. Returns nil if provider not set or on error.
func (s *Service) loadUserLevel(ctx context.Context, userID string) *UserLevelInfo {
	if s.levelProvider == nil {
		return nil
	}
	info, err := s.levelProvider.GetLevel(ctx, userID)
	if err != nil {
		return nil
	}
	return &info
}

// loadBrainCategories fetches brain categories from the repository.
// Returns an empty slice on error (non-fatal — email renders without section headers).
func (s *Service) loadBrainCategories(ctx context.Context) []collector.BrainCategory {
	if s.brainCatRepo == nil {
		return nil
	}
	cats, err := s.brainCatRepo.GetAll(ctx)
	if err != nil {
		fmt.Printf("WARNING: failed to load brain categories for delivery: %v\n", err)
		return nil
	}
	return cats
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

// ResolveDeliveryTargets prepares the delivery context: finds the latest run,
// loads context items, and determines all user+channel targets.
// Returns nil if no valid run or items exist.
func (s *Service) ResolveDeliveryTargets(ctx context.Context) (*DeliveryPlan, error) {
	run, err := s.collectorSvc.GetLatestRun(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest run: %w", err)
	}

	if run.Status != collector.RunStatusSuccess {
		return nil, fmt.Errorf("latest run is not completed (status: %s)", run.Status)
	}

	items, err := s.collectorSvc.GetContextItems(ctx, run.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get context items: %w", err)
	}

	if len(items) == 0 {
		return nil, nil
	}

	users, err := s.repo.GetEligibleUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get eligible users: %w", err)
	}

	targets := make([]DeliveryTarget, 0)
	for _, user := range users {
		for _, channel := range user.EnabledChannels {
			targets = append(targets, DeliveryTarget{
				User:       user,
				Channel:    channel,
				RetryCount: 0,
			})
		}
	}

	return &DeliveryPlan{
		RunID:   run.ID.String(),
		Items:   items,
		Targets: targets,
	}, nil
}

// DeliverToTargets delivers messages to specific user+channel targets in batches of 1000.
// This is the execution engine used by both initial delivery and retries.
func (s *Service) DeliverToTargets(ctx context.Context, runID string, items []collector.ContextItem, targets []DeliveryTarget) *DeliveryResult {
	result := &DeliveryResult{
		TotalUsers:     len(targets),
		FailedUsers:    []string{},
		DeliveryErrors: make(map[string]string),
	}

	brainCats := s.loadBrainCategories(ctx)

	for i := 0; i < len(targets); i += deliveryBatchSize {
		end := i + deliveryBatchSize
		if end > len(targets) {
			end = len(targets)
		}
		s.deliverBatch(ctx, runID, items, targets[i:end], brainCats, result)
	}

	return result
}

// deliverBatch processes one batch of targets.
// Pre-loads lastEarnedAt for all users in the batch to avoid N+1 queries.
func (s *Service) deliverBatch(ctx context.Context, runID string, items []collector.ContextItem, targets []DeliveryTarget, brainCats []collector.BrainCategory, result *DeliveryResult) {
	// Pre-load lastEarnedAt for all users in this batch
	lastEarnedMap := make(map[string]time.Time)
	if s.levelProvider != nil {
		userIDs := make([]string, len(targets))
		for i, t := range targets {
			userIDs[i] = t.User.UserID
		}
		m, err := s.levelProvider.GetLastEarnedAtBatch(ctx, userIDs)
		if err != nil {
			fmt.Printf("WARNING: failed to load last earned times for batch: %v\n", err)
		} else {
			lastEarnedMap = m
		}
	}

	for _, target := range targets {
		// Check idempotency
		alreadySent, err := s.repo.HasDeliveryLog(ctx, runID, target.User.UserID, target.Channel)
		if err != nil {
			result.FailureCount++
			result.FailedUsers = append(result.FailedUsers, target.User.UserID)
			result.DeliveryErrors[fmt.Sprintf("%s_%s", target.User.UserID, target.Channel)] = fmt.Sprintf("failed to check delivery log: %v", err)
			continue
		}
		if alreadySent {
			result.SkippedCount++
			continue
		}

		// Build message context for personalized links and point display
		var msgCtx *MessageContext
		if runID != "" {
			daysSince := 0
			if lastEarned, ok := lastEarnedMap[target.User.UserID]; ok {
				daysSince = calcDaysSinceKST(lastEarned)
			}
			msgCtx = &MessageContext{
				UserID:            target.User.UserID,
				RunID:             runID,
				DaysSinceLastEarn: daysSince,
			}
		}

		levelInfo := s.loadUserLevel(ctx, target.User.UserID)
		message := FormatMessage(items, target.User.Subscriptions, brainCats, s.frontendURL, levelInfo, msgCtx)

		err = s.sendViaChannel(ctx, target.Channel, target.User, message)
		if err != nil {
			result.FailureCount++
			result.FailedUsers = append(result.FailedUsers, target.User.UserID)
			result.DeliveryErrors[fmt.Sprintf("%s_%s", target.User.UserID, target.Channel)] = err.Error()
			s.logDelivery(ctx, runID, target.User.UserID, target.Channel, target.RetryCount, StatusFailed, err.Error())
		} else {
			result.SuccessCount++
			s.logDelivery(ctx, runID, target.User.UserID, target.Channel, target.RetryCount, StatusSent, "")
		}
	}
}

// DeliverToUser sends the latest briefing to a single authenticated user on-demand.
// Uses the same DeliverToTargets execution engine as the scheduled delivery — no logic duplicated.
func (s *Service) DeliverToUser(ctx context.Context, userID string) (*DeliveryResult, error) {
	run, err := s.collectorSvc.GetLatestRun(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest run: %w", err)
	}
	if run.Status != collector.RunStatusSuccess {
		return nil, fmt.Errorf("latest run is not completed (status: %s)", run.Status)
	}

	items, err := s.collectorSvc.GetContextItems(ctx, run.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get context items: %w", err)
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("no context items available")
	}

	user, err := s.repo.GetEligibleUserByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}
	if user == nil {
		return nil, fmt.Errorf("no enabled delivery channels — please set up a channel first")
	}

	targets := make([]DeliveryTarget, 0, len(user.EnabledChannels))
	for _, channel := range user.EnabledChannels {
		targets = append(targets, DeliveryTarget{
			User:       *user,
			Channel:    channel,
			RetryCount: 0,
		})
	}

	return s.DeliverToTargets(ctx, run.ID.String(), items, targets), nil
}

// ForceDeliverToUser sends the latest briefing to a single user, bypassing idempotency.
// Used for admin testing — always sends regardless of prior delivery logs.
func (s *Service) ForceDeliverToUser(ctx context.Context, userID string) (*DeliveryResult, error) {
	run, err := s.collectorSvc.GetLatestRun(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest run: %w", err)
	}
	if run.Status != collector.RunStatusSuccess {
		return nil, fmt.Errorf("latest run is not completed (status: %s)", run.Status)
	}

	items, err := s.collectorSvc.GetContextItems(ctx, run.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get context items: %w", err)
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("no context items available")
	}

	user, err := s.repo.GetEligibleUserByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}
	if user == nil {
		return nil, fmt.Errorf("no enabled delivery channels — please set up a channel first")
	}

	result := &DeliveryResult{
		TotalUsers:     len(user.EnabledChannels),
		FailedUsers:    []string{},
		DeliveryErrors: make(map[string]string),
	}

	var msgCtx *MessageContext
	if run.ID.String() != "" {
		daysSince := 0

		msgCtx = &MessageContext{
			UserID:            userID,
			RunID:             run.ID.String(),
			DaysSinceLastEarn: daysSince,
		}
	}
	brainCats := s.loadBrainCategories(ctx)
	levelInfo := s.loadUserLevel(ctx, userID)
	message := FormatMessage(items, user.Subscriptions, brainCats, s.frontendURL, levelInfo, msgCtx)

	for _, channel := range user.EnabledChannels {
		sendErr := s.sendViaChannel(ctx, channel, *user, message)
		if sendErr != nil {
			result.FailureCount++
			result.FailedUsers = append(result.FailedUsers, user.UserID)
			result.DeliveryErrors[fmt.Sprintf("%s_%s", user.UserID, channel)] = sendErr.Error()
			s.logDelivery(ctx, run.ID.String(), user.UserID, channel, 0, StatusFailed, sendErr.Error())
		} else {
			result.SuccessCount++
			s.logDelivery(ctx, run.ID.String(), user.UserID, channel, 0, StatusSent, "")
		}
	}

	return result, nil
}

// DeliverAll sends messages to all eligible users.
// Thin wrapper: resolves targets then delivers.
func (s *Service) DeliverAll(ctx context.Context) (*DeliveryResult, error) {
	plan, err := s.ResolveDeliveryTargets(ctx)
	if err != nil {
		return nil, err
	}
	if plan == nil {
		return &DeliveryResult{}, nil
	}
	return s.DeliverToTargets(ctx, plan.RunID, plan.Items, plan.Targets), nil
}

// RetryFailedDeliveries finds deliveries that failed in the latest run
// and retries them with incremented retry_count.
func (s *Service) RetryFailedDeliveries(ctx context.Context) (*DeliveryResult, error) {
	run, err := s.collectorSvc.GetLatestRun(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest run: %w", err)
	}

	if run.Status != collector.RunStatusSuccess {
		return nil, nil
	}

	failed, err := s.repo.GetFailedDeliveries(ctx, run.ID.String(), MaxRetries)
	if err != nil {
		return nil, fmt.Errorf("failed to get failed deliveries: %w", err)
	}

	if len(failed) == 0 {
		return &DeliveryResult{}, nil
	}

	items, err := s.collectorSvc.GetContextItems(ctx, run.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get context items: %w", err)
	}

	targets := make([]DeliveryTarget, 0, len(failed))
	for _, f := range failed {
		targets = append(targets, DeliveryTarget{
			User: EligibleUser{
				UserID:        f.UserID,
				Email:         f.Email,
				Subscriptions: f.Subscriptions,
			},
			Channel:    f.Channel,
			RetryCount: f.RetryCount + 1,
		})
	}

	return s.DeliverToTargets(ctx, run.ID.String(), items, targets), nil
}

// GetUserDeliveryStatus returns the most recent delivery status per channel for a user
func (s *Service) GetUserDeliveryStatus(ctx context.Context, userID string) ([]DeliveryLog, error) {
	return s.repo.GetLatestDeliveryStatus(ctx, userID)
}

// DeliverToNewUser sends the most recent already-delivered briefing to a newly registered user
func (s *Service) DeliverToNewUser(ctx context.Context, userID string, userEmail string) error {
	if userEmail == "" {
		return nil
	}

	run, err := s.collectorSvc.GetLastDeliveredRun(ctx)
	if err != nil {
		return nil // No delivered runs yet — skip silently
	}

	alreadySent, err := s.repo.HasDeliveryLog(ctx, run.ID.String(), userID, ChannelEmail)
	if err != nil {
		return fmt.Errorf("failed to check delivery log: %w", err)
	}
	if alreadySent {
		return nil
	}

	items, err := s.collectorSvc.GetContextItems(ctx, run.ID)
	if err != nil {
		return fmt.Errorf("failed to get context items: %w", err)
	}
	if len(items) == 0 {
		return nil
	}

	brainCats := s.loadBrainCategories(ctx)
	message := FormatMessage(items, []string{}, brainCats, s.frontendURL, nil, nil)

	if err := s.emailSender.Send(userEmail, message.Subject, message.TextBody, message.HTMLBody); err != nil {
		s.logDelivery(ctx, run.ID.String(), userID, ChannelEmail, 0, StatusFailed, err.Error())
		return fmt.Errorf("failed to send welcome email: %w", err)
	}

	s.logDelivery(ctx, run.ID.String(), userID, ChannelEmail, 0, StatusSent, "")
	return nil
}

func (s *Service) logDelivery(ctx context.Context, runID string, userID string, channel DeliveryChannel, retryCount int, status DeliveryStatus, errorMsg string) {
	log := DeliveryLog{
		ID:           uuid.New().String(),
		RunID:        runID,
		UserID:       userID,
		Channel:      channel,
		Status:       status,
		ErrorMessage: errorMsg,
		RetryCount:   retryCount,
		CreatedAt:    time.Now().UTC(),
	}

	if err := s.repo.LogDelivery(ctx, log); err != nil {
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
		return fmt.Errorf("kakao channel not yet implemented")

	case ChannelTelegram:
		return fmt.Errorf("telegram channel not yet implemented")

	case ChannelSMS:
		return fmt.Errorf("sms channel not yet implemented")

	case ChannelPush:
		return fmt.Errorf("push channel not yet implemented")

	default:
		return fmt.Errorf("unknown delivery channel: %s", channel)
	}
}

// calcDaysSinceKST returns the number of KST calendar days between lastEarned and now.
func calcDaysSinceKST(lastEarned time.Time) int {
	now := time.Now().In(kstLocation)
	last := lastEarned.In(kstLocation)
	nowDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, kstLocation)
	lastDate := time.Date(last.Year(), last.Month(), last.Day(), 0, 0, 0, 0, kstLocation)
	days := int(nowDate.Sub(lastDate).Hours() / 24)
	if days < 0 {
		return 0
	}
	return days
}
