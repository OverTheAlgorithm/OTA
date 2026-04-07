package push

import (
	"time"

	"github.com/google/uuid"
)

// PushToken represents a device push token registered by a user.
type PushToken struct {
	ID        uuid.UUID `json:"id"`
	UserID    string    `json:"user_id"`
	Token     string    `json:"token"`
	Platform  string    `json:"platform"`
	CreatedAt time.Time `json:"created_at"`
}
