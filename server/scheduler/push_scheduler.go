package scheduler

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"ota/domain/push"

	"github.com/google/uuid"
)

// PushExecutor is accepted by PushScheduler to avoid a concrete dependency on ScheduledService.
type PushExecutor interface {
	ExecuteBySchedule(ctx context.Context, id uuid.UUID) error
}

// PushScheduler manages time.AfterFunc-based timers for scheduled push notifications.
// It is independent of the cron-based Scheduler and wired directly in main.go.
type PushScheduler struct {
	timers      map[uuid.UUID]*time.Timer
	mu          sync.Mutex
	executor    PushExecutor
	shutdownCtx context.Context
}

// NewPushScheduler creates a new PushScheduler.
func NewPushScheduler(executor PushExecutor, shutdownCtx context.Context) *PushScheduler {
	return &PushScheduler{
		timers:      make(map[uuid.UUID]*time.Timer),
		executor:    executor,
		shutdownCtx: shutdownCtx,
	}
}

// Schedule registers a timer for the push notification's scheduled_at time.
// If scheduled_at is nil, this is a no-op. Past-due pushes fire immediately via goroutine.
func (ps *PushScheduler) Schedule(p push.ScheduledPush) error {
	if p.ScheduledAt == nil {
		return nil
	}

	dur := time.Until(*p.ScheduledAt)
	id := p.ID

	ps.mu.Lock()
	defer ps.mu.Unlock()

	// Stop any existing timer for this ID before replacing.
	if t, ok := ps.timers[id]; ok {
		t.Stop()
	}

	if dur <= 0 {
		// Past-due: fire immediately in a goroutine.
		slog.Info("push past-due, firing immediately", "id", id, "scheduled_at", p.ScheduledAt)
		go ps.fire(id)
		return nil
	}

	slog.Info("push scheduled", "id", id, "scheduled_at", p.ScheduledAt, "in", dur)
	ps.timers[id] = time.AfterFunc(dur, func() {
		ps.fire(id)
	})
	return nil
}

// Unschedule cancels the timer for the given push ID. No-op if not found.
func (ps *PushScheduler) Unschedule(id uuid.UUID) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if t, ok := ps.timers[id]; ok {
		t.Stop()
		delete(ps.timers, id)
		slog.Info("push unscheduled", "id", id)
	}
}

// Stop cancels all active timers. Must be called before shutdownCancel() in main.go.
func (ps *PushScheduler) Stop() {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	for id, t := range ps.timers {
		t.Stop()
		delete(ps.timers, id)
	}
	slog.Info("push scheduler stopped")
}

// ReloadPending schedules all provided pending pushes. Past-due pushes fire immediately.
// The caller is responsible for fetching pending pushes from DB before calling this.
func (ps *PushScheduler) ReloadPending(ctx context.Context, pending []push.ScheduledPush) error {
	for _, p := range pending {
		if err := ps.Schedule(p); err != nil {
			return err
		}
	}
	slog.Info("pending pushes reloaded", "count", len(pending))
	return nil
}

func (ps *PushScheduler) fire(id uuid.UUID) {
	// Remove timer entry on fire.
	ps.mu.Lock()
	delete(ps.timers, id)
	ps.mu.Unlock()

	ctx, cancel := context.WithTimeout(ps.shutdownCtx, 30*time.Second)
	defer cancel()

	slog.Info("push firing", "id", id)
	if err := ps.executor.ExecuteBySchedule(ctx, id); err != nil {
		slog.Error("push send failed", "id", id, "error", err)
	}
}
