package delivery

import (
	"time"

	"ota/domain/collector"
)

// UserDeliveryChannel represents a user's delivery channel preference
type UserDeliveryChannel struct {
	ID        string
	UserID    string
	Channel   DeliveryChannel
	Enabled   bool
	CreatedAt time.Time
	UpdatedAt time.Time
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
	RetryCount   int
	CreatedAt    time.Time
}

// DeliveryTarget represents a specific user+channel pair to deliver to
type DeliveryTarget struct {
	User       EligibleUser
	Channel    DeliveryChannel
	RetryCount int
}

// FailedDelivery represents a delivery that failed and may be retryable
type FailedDelivery struct {
	RunID         string
	UserID        string
	Email         string
	Channel       DeliveryChannel
	RetryCount    int
	Subscriptions []string
	FailedAt      time.Time
}

// DeliveryChannel represents the delivery method
type DeliveryChannel string

const (
	ChannelEmail    DeliveryChannel = "email"
	ChannelKakao    DeliveryChannel = "kakao"
	ChannelTelegram DeliveryChannel = "telegram"
	ChannelSMS      DeliveryChannel = "sms"
	ChannelPush     DeliveryChannel = "push"
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

// MaxRetries is the maximum number of retry attempts for failed deliveries
const MaxRetries = 3

// DeliveryPlan holds everything needed to execute deliveries
type DeliveryPlan struct {
	RunID   string
	Items   []collector.ContextItem
	Targets []DeliveryTarget
}

// EligibleUser represents a user who should receive a message
type EligibleUser struct {
	UserID          string
	Email           string
	Subscriptions   []string
	EnabledChannels []DeliveryChannel // Channels user has enabled (email, kakao, etc.)
}

// UserLevelInfo holds per-user level data for email rendering. Nil = no level card.
type UserLevelInfo struct {
	Level       int
	TotalCoins  int
	DailyLimit  int
	Description string
}

// MessageContext holds per-user context for personalized link generation and coin display.
// Nil = no personalization (e.g. welcome emails, admin previews).
type MessageContext struct {
	UserID string
	RunID  string
}
