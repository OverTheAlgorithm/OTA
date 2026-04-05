package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/robfig/cron/v3"

	"ota/domain/collector"
	"ota/domain/delivery"
)

// Scheduler manages all scheduled tasks (collection, delivery, retry)
type Scheduler struct {
	cron             *cron.Cron
	collectorService *collector.Service
	deliveryService  *delivery.Service
	shutdownCtx      context.Context
}

// New creates a new Scheduler. The shutdownCtx is used as a parent context for
// long-running tasks (e.g. collection) so they are cancelled on server shutdown.
func New(collectorService *collector.Service, deliveryService *delivery.Service, shutdownCtx context.Context) *Scheduler {
	return &Scheduler{
		cron:             cron.New(cron.WithLocation(time.UTC)),
		collectorService: collectorService,
		deliveryService:  deliveryService,
		shutdownCtx:      shutdownCtx,
	}
}

// Start registers all cron jobs and starts the scheduler.
//
// Schedule (all times KST = UTC+9):
//
//	Collection: 4 AM, 5 AM, 6 AM    (multiple attempts to ensure data ready)
//	Delivery:   7:00 AM, 7:15 AM    (primary + fallback)
//	Retry:      7:30 AM, 8:00 AM, 8:30 AM (30min intervals, max 3 retries)
func (s *Scheduler) Start() error {
	// Collection: 4 AM, 5 AM, 6 AM KST → 19:00, 20:00, 21:00 UTC
	collectionSchedules := []string{"0 19 * * *", "0 20 * * *", "0 21 * * *"}
	for _, schedule := range collectionSchedules {
		if _, err := s.cron.AddFunc(schedule, s.collect); err != nil {
			return fmt.Errorf("failed to schedule collection (%s): %w", schedule, err)
		}
	}

	// Delivery: 7:00 AM, 7:15 AM KST → 22:00, 22:15 UTC
	deliverySchedules := []string{"0 22 * * *", "15 22 * * *"}
	for _, schedule := range deliverySchedules {
		if _, err := s.cron.AddFunc(schedule, s.deliver); err != nil {
			return fmt.Errorf("failed to schedule delivery (%s): %w", schedule, err)
		}
	}

	// Retry: 7:30 AM, 8:00 AM, 8:30 AM KST → 22:30, 23:00, 23:30 UTC
	retrySchedules := []string{"30 22 * * *", "0 23 * * *", "30 23 * * *"}
	for _, schedule := range retrySchedules {
		if _, err := s.cron.AddFunc(schedule, s.retryFailed); err != nil {
			return fmt.Errorf("failed to schedule retry (%s): %w", schedule, err)
		}
	}

	s.cron.Start()
	return nil
}

// Stop gracefully stops the scheduler
func (s *Scheduler) Stop() context.Context {
	return s.cron.Stop()
}

func (s *Scheduler) collect() {
	slog.Info("checking if collection is needed")
	ctx, cancel := context.WithTimeout(s.shutdownCtx, time.Hour)
	defer cancel()
	result, err := s.collectorService.CollectFromSourcesIfNeeded(ctx)
	if err != nil {
		slog.Error("collection failed", "error", err)
		return
	}
	if result == nil {
		slog.Info("collection already completed today or in progress, skipping")
		return
	}
	slog.Info("collection completed", "run_id", result.Run.ID, "items", len(result.Items))
}

func (s *Scheduler) deliver() {
	slog.Info("starting message delivery")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	result, err := s.deliveryService.DeliverAll(ctx)
	if err != nil {
		slog.Error("delivery failed", "error", err)
		return
	}
	slog.Info("delivery completed", "total", result.TotalUsers, "success", result.SuccessCount, "failed", result.FailureCount, "skipped", result.SkippedCount)
}

func (s *Scheduler) retryFailed() {
	slog.Info("starting retry for failed deliveries")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	result, err := s.deliveryService.RetryFailedDeliveries(ctx)
	if err != nil {
		slog.Error("retry failed", "error", err)
		return
	}
	if result == nil {
		slog.Info("no failed deliveries to retry")
		return
	}
	slog.Info("retry completed", "total", result.TotalUsers, "success", result.SuccessCount, "failed", result.FailureCount)
}
