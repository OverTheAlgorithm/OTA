package scheduler

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/robfig/cron/v3"

	"ota/domain/collector"
	"ota/domain/delivery"
)

// CleanupRunner runs data-retention cleanup jobs.
type CleanupRunner interface {
	RunAll(ctx context.Context)
}

// Scheduler manages all scheduled tasks (collection, delivery, retry, cleanup)
type Scheduler struct {
	cron             *cron.Cron
	collectorService *collector.Service
	deliveryService  *delivery.Service
	cleanupRunner    CleanupRunner
}

// New creates a new Scheduler
func New(collectorService *collector.Service, deliveryService *delivery.Service) *Scheduler {
	return &Scheduler{
		cron:             cron.New(cron.WithLocation(time.UTC)),
		collectorService: collectorService,
		deliveryService:  deliveryService,
	}
}

// WithCleanup registers a CleanupRunner that will be invoked daily at 3 AM KST.
func (s *Scheduler) WithCleanup(r CleanupRunner) *Scheduler {
	s.cleanupRunner = r
	return s
}

// Start registers all cron jobs and starts the scheduler.
//
// Schedule (all times KST = UTC+9):
//
//	Cleanup:    3:00 AM              (data retention: trending, runs, delivery logs)
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

	// Cleanup: 3:00 AM KST → 18:00 UTC
	if s.cleanupRunner != nil {
		if _, err := s.cron.AddFunc("0 18 * * *", s.cleanup); err != nil {
			return fmt.Errorf("failed to schedule cleanup: %w", err)
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
	log.Println("checking if collection is needed")
	ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
	defer cancel()
	result, err := s.collectorService.CollectFromSourcesIfNeeded(ctx)
	if err != nil {
		log.Printf("collection failed: %v", err)
		return
	}
	if result == nil {
		log.Println("collection already completed today or in progress, skipping")
		return
	}
	log.Printf("collection completed: run_id=%s, items=%d", result.Run.ID, len(result.Items))
}

func (s *Scheduler) deliver() {
	log.Println("starting message delivery")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	result, err := s.deliveryService.DeliverAll(ctx)
	if err != nil {
		log.Printf("delivery failed: %v", err)
		return
	}
	log.Printf("delivery completed: total=%d, success=%d, failed=%d, skipped=%d",
		result.TotalUsers, result.SuccessCount, result.FailureCount, result.SkippedCount)
}

func (s *Scheduler) retryFailed() {
	log.Println("starting retry for failed deliveries")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	result, err := s.deliveryService.RetryFailedDeliveries(ctx)
	if err != nil {
		log.Printf("retry failed: %v", err)
		return
	}
	if result == nil {
		log.Println("no failed deliveries to retry")
		return
	}
	log.Printf("retry completed: total=%d, success=%d, failed=%d",
		result.TotalUsers, result.SuccessCount, result.FailureCount)
}

func (s *Scheduler) cleanup() {
	log.Println("starting data retention cleanup")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	s.cleanupRunner.RunAll(ctx)
	log.Println("data retention cleanup completed")
}

