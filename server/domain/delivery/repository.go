package delivery

import "context"

// Repository defines data access operations for delivery system
type Repository interface {
	// GetEligibleUsers returns users who should receive messages
	// Users must have delivery_enabled=true
	// Returns user info with their subscriptions
	GetEligibleUsers(ctx context.Context) ([]EligibleUser, error)

	// LogDelivery records a delivery attempt
	LogDelivery(ctx context.Context, log DeliveryLog) error

	// HasDeliveryLog checks if a delivery was already attempted
	// Used for idempotency check
	HasDeliveryLog(ctx context.Context, runID string, userID string, channel DeliveryChannel) (bool, error)

	// GetUserDeliveryChannels returns all delivery channels for a user
	GetUserDeliveryChannels(ctx context.Context, userID string) ([]UserDeliveryChannel, error)

	// UpsertUserDeliveryChannel creates or updates a user's channel preference
	UpsertUserDeliveryChannel(ctx context.Context, channel UserDeliveryChannel) error
}
