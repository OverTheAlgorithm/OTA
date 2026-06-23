package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/robfig/cron/v3"

	"ota/domain/communitytrend"
)

// CommunityTrendScheduler manages the daily community-trend collection. It runs
// on its own cron instance, fully separate from the news collector/delivery
// Scheduler above, but follows the same structure (New / Start / Stop / private
// task method) so the two read identically.
type CommunityTrendScheduler struct {
	cron        *cron.Cron
	pipeline    *communitytrend.Pipeline
	shutdownCtx context.Context
}

// NewCommunityTrend creates a new CommunityTrendScheduler. The shutdownCtx is
// used as a parent context for the run so it is cancelled on server shutdown.
func NewCommunityTrend(pipeline *communitytrend.Pipeline, shutdownCtx context.Context) *CommunityTrendScheduler {
	return &CommunityTrendScheduler{
		cron:        cron.New(cron.WithLocation(time.UTC)),
		pipeline:    pipeline,
		shutdownCtx: shutdownCtx,
	}
}

// Start registers the daily cron job and starts the scheduler.
//
// Schedule (all times KST = UTC+9):
//
//	Collection: 3 AM   (robots gate → adapter fetch → AI suggest; humans confirm during the day)
func (s *CommunityTrendScheduler) Start() error {
	// Collection: 3 AM KST → 18:00 UTC
	if _, err := s.cron.AddFunc("0 18 * * *", s.collect); err != nil {
		return fmt.Errorf("failed to schedule community-trend collection: %w", err)
	}

	s.cron.Start()
	return nil
}

// Stop gracefully stops the scheduler.
func (s *CommunityTrendScheduler) Stop() context.Context {
	return s.cron.Stop()
}

func (s *CommunityTrendScheduler) collect() {
	slog.Info("starting community-trend collection")
	ctx, cancel := context.WithTimeout(s.shutdownCtx, 30*time.Minute)
	defer cancel()

	// stat_date = the KST calendar date.
	kst := time.FixedZone("KST", 9*3600)
	now := time.Now().In(kst)
	date := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	results, err := s.pipeline.RunDaily(ctx, date)
	if err != nil {
		slog.Error("community-trend collection failed", "error", err)
		return
	}

	var suggested, manual, errored int
	for _, r := range results {
		switch r.Status {
		case "suggested":
			suggested++
		case "pending":
			manual++
		case "error":
			errored++
		}
	}
	slog.Info("community-trend collection completed",
		"communities", len(results), "suggested", suggested, "manual", manual, "errored", errored)
}
