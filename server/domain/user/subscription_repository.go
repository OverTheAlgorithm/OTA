package user

import "context"

type SubscriptionRepository interface {
	GetSubscriptions(ctx context.Context, userID string) ([]string, error)
	AddSubscription(ctx context.Context, userID, category string) error
	DeleteSubscription(ctx context.Context, userID, category string) error
}
