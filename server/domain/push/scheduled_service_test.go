package push

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// ─── Mock Repository ─────────────────────────────────────────────────────────

type mockScheduledRepoForService struct {
	created      ScheduledPush
	createErr    error
	byID         ScheduledPush
	getErr       error
	updateErr    error
	listed       []ScheduledPush
	listErr      error
	markSentOk   bool
	markSentErr  error
	markFailedOk bool
	markFailedErr error
	markCancelOk  bool
	markCancelErr error
	// track calls
	sendToAllCalled bool
}

func (m *mockScheduledRepoForService) Create(_ context.Context, p ScheduledPush) (ScheduledPush, error) {
	if m.createErr != nil {
		return ScheduledPush{}, m.createErr
	}
	m.created = p
	return p, nil
}

func (m *mockScheduledRepoForService) GetByID(_ context.Context, _ uuid.UUID) (ScheduledPush, error) {
	return m.byID, m.getErr
}

func (m *mockScheduledRepoForService) Update(_ context.Context, _ ScheduledPush) error {
	return m.updateErr
}

func (m *mockScheduledRepoForService) List(_ context.Context, status *string) ([]ScheduledPush, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	if status == nil {
		return m.listed, nil
	}
	var filtered []ScheduledPush
	for _, p := range m.listed {
		if string(p.Status) == *status {
			filtered = append(filtered, p)
		}
	}
	return filtered, nil
}

func (m *mockScheduledRepoForService) ListPending(_ context.Context) ([]ScheduledPush, error) {
	return m.listed, m.listErr
}

func (m *mockScheduledRepoForService) MarkSent(_ context.Context, _ uuid.UUID, _ time.Time) (bool, error) {
	return m.markSentOk, m.markSentErr
}

func (m *mockScheduledRepoForService) MarkFailed(_ context.Context, _ uuid.UUID, _ string) (bool, error) {
	return m.markFailedOk, m.markFailedErr
}

func (m *mockScheduledRepoForService) MarkCancelled(_ context.Context, _ uuid.UUID) (bool, error) {
	return m.markCancelOk, m.markCancelErr
}

// ─── Mock Push Sender ─────────────────────────────────────────────────────────
// ScheduledService calls s.pushService.SendToAll(). We need a test double for Service.
// We inject via a wrapper so we can intercept without modifying production code.

type mockPushSenderForService struct {
	called bool
	err    error
}

// sendToAllFunc is a func-based adapter used to stub pushService.SendToAll in tests.
// Since ScheduledService depends on *Service (concrete), we build a real Service
// backed by a mock push repository that records calls.

type mockPushRepoForSender struct {
	sendCalled bool
	err        error
}

func (m *mockPushRepoForSender) Save(_ context.Context, _ PushToken) error       { return nil }
func (m *mockPushRepoForSender) Delete(_ context.Context, _, _ string) error     { return nil }
func (m *mockPushRepoForSender) GetByUserID(_ context.Context, _ string) ([]PushToken, error) {
	m.sendCalled = true
	if m.err != nil {
		return nil, m.err
	}
	return nil, nil
}
func (m *mockPushRepoForSender) GetAllActive(_ context.Context) ([]PushToken, error) {
	m.sendCalled = true
	if m.err != nil {
		return nil, m.err
	}
	// Return empty so sendMessages is not called (no HTTP).
	return []PushToken{}, nil
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func newTestScheduledService(repo *mockScheduledRepoForService, pushRepo *mockPushRepoForSender) *ScheduledService {
	pushSvc := NewService(pushRepo)
	return NewScheduledService(repo, pushSvc)
}

// ─── Create Tests ─────────────────────────────────────────────────────────────

func TestScheduledService_Create(t *testing.T) {
	futureTime := time.Now().Add(time.Hour)

	tests := []struct {
		name      string
		title     string
		body      string
		data      map[string]any
		wantErr   bool
		errSubstr string
	}{
		{
			name:  "valid create",
			title: "Breaking News",
			body:  "Something happened",
		},
		{
			name:      "missing title",
			title:     "",
			body:      "Something happened",
			wantErr:   true,
			errSubstr: "title is required",
		},
		{
			name:      "missing body",
			title:     "Breaking News",
			body:      "",
			wantErr:   true,
			errSubstr: "body is required",
		},
		{
			name:  "with data",
			title: "News",
			body:  "Content",
			data:  map[string]any{"key": "value"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockScheduledRepoForService{}
			svc := newTestScheduledService(repo, &mockPushRepoForSender{})

			got, err := svc.Create(context.Background(), tt.title, tt.body, "", tt.data, &futureTime, "user-1")
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.errSubstr)
				}
				if !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errSubstr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Title != tt.title {
				t.Errorf("title: want %q, got %q", tt.title, got.Title)
			}
			if got.Status != StatusPending {
				t.Errorf("status: want pending, got %q", got.Status)
			}
		})
	}
}

func TestScheduledService_Create_DataTooLarge(t *testing.T) {
	// Build a data map whose JSON exceeds 4KB.
	// The handler validates data size, not the service — so the service should accept it.
	// (Validation lives at handler level per the plan; service only validates title/body.)
	// This test confirms the service does NOT reject large data — it delegates to handler.
	repo := &mockScheduledRepoForService{}
	svc := newTestScheduledService(repo, &mockPushRepoForSender{})

	bigValue := strings.Repeat("x", 5000)
	bigData := map[string]any{"payload": bigValue}

	futureTime := time.Now().Add(time.Hour)
	_, err := svc.Create(context.Background(), "title", "body", "", bigData, &futureTime, "user-1")
	// Service itself does not reject large data — that's the handler's job.
	if err != nil {
		t.Errorf("service should not reject large data, got: %v", err)
	}
}

func TestScheduledService_Create_RepoError(t *testing.T) {
	repo := &mockScheduledRepoForService{createErr: errors.New("db error")}
	svc := newTestScheduledService(repo, &mockPushRepoForSender{})

	futureTime := time.Now().Add(time.Hour)
	_, err := svc.Create(context.Background(), "title", "body", "", nil, &futureTime, "user-1")
	if err == nil {
		t.Fatal("expected error from repo, got nil")
	}
	if !strings.Contains(err.Error(), "create scheduled push") {
		t.Errorf("error should mention create scheduled push, got: %v", err)
	}
}

// ─── Update Tests ─────────────────────────────────────────────────────────────

func TestScheduledService_Update(t *testing.T) {
	id := uuid.New()
	pendingPush := ScheduledPush{ID: id, Status: StatusPending, CreatedBy: "user-1"}
	sentPush := ScheduledPush{ID: id, Status: StatusSent, CreatedBy: "user-1"}

	tests := []struct {
		name      string
		existing  ScheduledPush
		getErr    error
		wantErr   bool
		errSubstr string
	}{
		{
			name:     "pending push updates successfully",
			existing: pendingPush,
		},
		{
			name:      "non-pending push returns error",
			existing:  sentPush,
			wantErr:   true,
			errSubstr: "not in pending status",
		},
		{
			name:      "push not found returns error",
			existing:  ScheduledPush{},
			getErr:    errors.New("not found"),
			wantErr:   true,
			errSubstr: "get scheduled push",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockScheduledRepoForService{byID: tt.existing, getErr: tt.getErr}
			svc := newTestScheduledService(repo, &mockPushRepoForSender{})

			_, err := svc.Update(context.Background(), id, "new title", "new body", "", nil, nil)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errSubstr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// ─── Delete Tests ─────────────────────────────────────────────────────────────

func TestScheduledService_Delete(t *testing.T) {
	id := uuid.New()

	tests := []struct {
		name      string
		markOk    bool
		markErr   error
		wantErr   bool
		errSubstr string
	}{
		{
			name:   "pending push cancelled",
			markOk: true,
		},
		{
			name:      "non-pending push returns error",
			markOk:    false,
			wantErr:   true,
			errSubstr: "not in pending status",
		},
		{
			name:      "repo error propagated",
			markErr:   errors.New("db error"),
			wantErr:   true,
			errSubstr: "cancel scheduled push",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockScheduledRepoForService{
				markCancelOk:  tt.markOk,
				markCancelErr: tt.markErr,
			}
			svc := newTestScheduledService(repo, &mockPushRepoForSender{})

			err := svc.Delete(context.Background(), id)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errSubstr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// ─── ExecuteNow Tests ─────────────────────────────────────────────────────────

func TestScheduledService_ExecuteNow_CAS_Won(t *testing.T) {
	// CAS succeeds → SendToAll should be called (via GetAllActive on push repo).
	pushRepo := &mockPushRepoForSender{}
	repo := &mockScheduledRepoForService{
		byID:       ScheduledPush{ID: uuid.New(), Title: "t", Body: "b", Status: StatusPending},
		markSentOk: true,
	}
	svc := newTestScheduledService(repo, pushRepo)

	err := svc.ExecuteNow(context.Background(), repo.byID.ID, "admin-user-id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !pushRepo.sendCalled {
		t.Error("expected SendToAll to be called when CAS wins")
	}
}

func TestScheduledService_ExecuteNow_CAS_Lost(t *testing.T) {
	// CAS fails (already sent by another caller) → SendToAll must NOT be called.
	pushRepo := &mockPushRepoForSender{}
	repo := &mockScheduledRepoForService{
		byID:       ScheduledPush{ID: uuid.New(), Title: "t", Body: "b", Status: StatusPending},
		markSentOk: false, // CAS lost
	}
	svc := newTestScheduledService(repo, pushRepo)

	err := svc.ExecuteNow(context.Background(), repo.byID.ID, "admin-user-id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pushRepo.sendCalled {
		t.Error("SendToAll must NOT be called when CAS is lost")
	}
}

// ─── ExecuteBySchedule Tests ─────────────────────────────────────────────────

func TestScheduledService_ExecuteBySchedule_CAS_Won(t *testing.T) {
	pushRepo := &mockPushRepoForSender{}
	repo := &mockScheduledRepoForService{
		byID:       ScheduledPush{ID: uuid.New(), Title: "t", Body: "b", Status: StatusPending},
		markSentOk: true,
	}
	svc := newTestScheduledService(repo, pushRepo)

	err := svc.ExecuteBySchedule(context.Background(), repo.byID.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !pushRepo.sendCalled {
		t.Error("expected SendToAll to be called when CAS wins via ExecuteBySchedule")
	}
}

func TestScheduledService_ExecuteBySchedule_CAS_Lost(t *testing.T) {
	pushRepo := &mockPushRepoForSender{}
	repo := &mockScheduledRepoForService{
		byID:       ScheduledPush{ID: uuid.New(), Title: "t", Body: "b", Status: StatusPending},
		markSentOk: false,
	}
	svc := newTestScheduledService(repo, pushRepo)

	err := svc.ExecuteBySchedule(context.Background(), repo.byID.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pushRepo.sendCalled {
		t.Error("SendToAll must NOT be called when CAS is lost via ExecuteBySchedule")
	}
}

// ─── List Tests ───────────────────────────────────────────────────────────────

func TestScheduledService_List(t *testing.T) {
	pending := ScheduledPush{ID: uuid.New(), Status: StatusPending}
	sent := ScheduledPush{ID: uuid.New(), Status: StatusSent}
	all := []ScheduledPush{pending, sent}

	pendingStr := "pending"

	tests := []struct {
		name      string
		status    *string
		wantCount int
	}{
		{
			name:      "list all",
			status:    nil,
			wantCount: 2,
		},
		{
			name:      "filter by pending",
			status:    &pendingStr,
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockScheduledRepoForService{listed: all}
			svc := newTestScheduledService(repo, &mockPushRepoForSender{})

			got, err := svc.List(context.Background(), tt.status)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != tt.wantCount {
				t.Errorf("want %d pushes, got %d", tt.wantCount, len(got))
			}
		})
	}
}
