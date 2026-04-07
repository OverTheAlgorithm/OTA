package push

import (
	"time"

	"github.com/google/uuid"
)

// ScheduledPushStatus represents the lifecycle state of a scheduled push notification.
type ScheduledPushStatus string

const (
	StatusPending   ScheduledPushStatus = "pending"
	StatusSent      ScheduledPushStatus = "sent"
	StatusFailed    ScheduledPushStatus = "failed"
	StatusCancelled ScheduledPushStatus = "cancelled"
)

// ScheduledPush represents an admin-created push notification, optionally scheduled for future delivery.
type ScheduledPush struct {
	ID           uuid.UUID           `json:"id"`
	Title        string              `json:"title"`
	Body         string              `json:"body"`
	Link         string              `json:"link"`
	Data         map[string]any      `json:"data,omitempty"`
	Status       ScheduledPushStatus `json:"status"`
	ScheduledAt  *time.Time          `json:"scheduled_at"`
	SentAt       *time.Time          `json:"sent_at,omitempty"`
	ErrorMessage string              `json:"error_message,omitempty"`
	CreatedBy    string              `json:"created_by"`
	CreatedAt    time.Time           `json:"created_at"`
	UpdatedAt    time.Time           `json:"updated_at"`
}
