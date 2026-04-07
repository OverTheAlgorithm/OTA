package scheduler

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"ota/domain/push"

	"github.com/google/uuid"
)

// ─── Mock Executor ────────────────────────────────────────────────────────────

type mockPushExecutorForScheduler struct {
	callCount atomic.Int32
	err       error
}

func (m *mockPushExecutorForScheduler) ExecuteBySchedule(_ context.Context, _ uuid.UUID) error {
	m.callCount.Add(1)
	return m.err
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func newTestScheduler(executor PushExecutor) *PushScheduler {
	ctx := context.Background()
	return NewPushScheduler(executor, ctx)
}

func makePush(scheduledAt *time.Time) push.ScheduledPush {
	return push.ScheduledPush{
		ID:          uuid.New(),
		Title:       "Test Push",
		Body:        "Test Body",
		Status:      push.StatusPending,
		ScheduledAt: scheduledAt,
	}
}

// ─── Tests ────────────────────────────────────────────────────────────────────

func TestPushScheduler_Schedule_Future(t *testing.T) {
	executor := &mockPushExecutorForScheduler{}
	ps := newTestScheduler(executor)

	future := time.Now().Add(100 * time.Millisecond)
	p := makePush(&future)

	if err := ps.Schedule(p); err != nil {
		t.Fatalf("Schedule error: %v", err)
	}

	ps.mu.Lock()
	_, exists := ps.timers[p.ID]
	ps.mu.Unlock()

	if !exists {
		t.Error("expected timer to be registered for future push")
	}

	ps.Stop()
}

func TestPushScheduler_Schedule_NilScheduledAt(t *testing.T) {
	executor := &mockPushExecutorForScheduler{}
	ps := newTestScheduler(executor)

	p := makePush(nil)
	if err := ps.Schedule(p); err != nil {
		t.Fatalf("Schedule error: %v", err)
	}

	ps.mu.Lock()
	_, exists := ps.timers[p.ID]
	ps.mu.Unlock()

	if exists {
		t.Error("nil scheduled_at should not create a timer")
	}
}

func TestPushScheduler_Schedule_PastDue_FiresImmediately(t *testing.T) {
	executor := &mockPushExecutorForScheduler{}
	ps := newTestScheduler(executor)

	past := time.Now().Add(-time.Second)
	p := makePush(&past)

	if err := ps.Schedule(p); err != nil {
		t.Fatalf("Schedule error: %v", err)
	}

	// Past-due pushes fire immediately in a goroutine — give it time.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if executor.callCount.Load() >= 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if executor.callCount.Load() == 0 {
		t.Error("expected ExecuteBySchedule to be called immediately for past-due push")
	}
}

func TestPushScheduler_Unschedule_CancelsTimer(t *testing.T) {
	executor := &mockPushExecutorForScheduler{}
	ps := newTestScheduler(executor)

	future := time.Now().Add(10 * time.Second)
	p := makePush(&future)

	if err := ps.Schedule(p); err != nil {
		t.Fatalf("Schedule error: %v", err)
	}

	ps.Unschedule(p.ID)

	ps.mu.Lock()
	_, exists := ps.timers[p.ID]
	ps.mu.Unlock()

	if exists {
		t.Error("timer should be removed after Unschedule")
	}
	if executor.callCount.Load() != 0 {
		t.Error("executor should not have been called after Unschedule")
	}
}

func TestPushScheduler_Unschedule_NoOp_UnknownID(t *testing.T) {
	executor := &mockPushExecutorForScheduler{}
	ps := newTestScheduler(executor)

	// Should not panic or error.
	ps.Unschedule(uuid.New())
}

func TestPushScheduler_ReloadPending_SchedulesAll(t *testing.T) {
	executor := &mockPushExecutorForScheduler{}
	ps := newTestScheduler(executor)

	future1 := time.Now().Add(10 * time.Second)
	future2 := time.Now().Add(20 * time.Second)
	pending := []push.ScheduledPush{
		makePush(&future1),
		makePush(&future2),
		makePush(nil), // nil scheduled_at — should be skipped
	}

	if err := ps.ReloadPending(context.Background(), pending); err != nil {
		t.Fatalf("ReloadPending error: %v", err)
	}

	ps.mu.Lock()
	timerCount := len(ps.timers)
	ps.mu.Unlock()

	if timerCount != 2 {
		t.Errorf("expected 2 timers registered, got %d", timerCount)
	}

	ps.Stop()
}

func TestPushScheduler_Stop_CancelsAllTimers(t *testing.T) {
	executor := &mockPushExecutorForScheduler{}
	ps := newTestScheduler(executor)

	for i := 0; i < 3; i++ {
		future := time.Now().Add(10 * time.Second)
		p := makePush(&future)
		if err := ps.Schedule(p); err != nil {
			t.Fatalf("Schedule error: %v", err)
		}
	}

	ps.mu.Lock()
	countBefore := len(ps.timers)
	ps.mu.Unlock()

	if countBefore != 3 {
		t.Fatalf("expected 3 timers before Stop, got %d", countBefore)
	}

	ps.Stop()

	ps.mu.Lock()
	countAfter := len(ps.timers)
	ps.mu.Unlock()

	if countAfter != 0 {
		t.Errorf("expected 0 timers after Stop, got %d", countAfter)
	}
}

func TestPushScheduler_FiresAfterDuration(t *testing.T) {
	executor := &mockPushExecutorForScheduler{}
	ps := newTestScheduler(executor)

	fireAt := time.Now().Add(100 * time.Millisecond)
	p := makePush(&fireAt)

	if err := ps.Schedule(p); err != nil {
		t.Fatalf("Schedule error: %v", err)
	}

	// Wait until the timer should have fired plus a buffer.
	time.Sleep(300 * time.Millisecond)

	if executor.callCount.Load() == 0 {
		t.Error("expected ExecuteBySchedule to be called after timer fired")
	}

	// Timer should have been removed from map after firing.
	ps.mu.Lock()
	_, exists := ps.timers[p.ID]
	ps.mu.Unlock()

	if exists {
		t.Error("timer entry should be removed after it fires")
	}
}

func TestPushScheduler_Schedule_ReplacesExistingTimer(t *testing.T) {
	executor := &mockPushExecutorForScheduler{}
	ps := newTestScheduler(executor)

	p := makePush(nil)
	// Schedule twice with future times — second should replace first.
	future1 := time.Now().Add(10 * time.Second)
	p.ScheduledAt = &future1
	if err := ps.Schedule(p); err != nil {
		t.Fatalf("first Schedule error: %v", err)
	}

	future2 := time.Now().Add(20 * time.Second)
	p.ScheduledAt = &future2
	if err := ps.Schedule(p); err != nil {
		t.Fatalf("second Schedule error: %v", err)
	}

	ps.mu.Lock()
	timerCount := len(ps.timers)
	ps.mu.Unlock()

	if timerCount != 1 {
		t.Errorf("expected 1 timer (replaced), got %d", timerCount)
	}

	ps.Stop()
}
