package delivery

import "time"

// UserPreference represents a user's message delivery settings
type UserPreference struct {
	UserID          string
	DeliveryEnabled bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// UserSubscription represents a user's subscription to a specific topic category
type UserSubscription struct {
	ID        string
	UserID    string
	Category  string
	CreatedAt time.Time
}

// DeliveryLog represents a record of a message delivery attempt
type DeliveryLog struct {
	ID           string
	RunID        string
	UserID       string
	Channel      DeliveryChannel
	Status       DeliveryStatus
	ErrorMessage string
	CreatedAt    time.Time
}

// DeliveryChannel represents the delivery method
type DeliveryChannel string

const (
	ChannelEmail DeliveryChannel = "email"
	ChannelKakao DeliveryChannel = "kakao"
)

// DeliveryStatus represents the delivery result
type DeliveryStatus string

const (
	StatusSent    DeliveryStatus = "sent"
	StatusFailed  DeliveryStatus = "failed"
	StatusSkipped DeliveryStatus = "skipped"
)

// FormattedMessage represents a formatted message ready for delivery
type FormattedMessage struct {
	Subject  string
	TextBody string
	HTMLBody string
}

// EligibleUser represents a user who should receive a message
type EligibleUser struct {
	UserID        string
	Email         string
	Subscriptions []string
}
