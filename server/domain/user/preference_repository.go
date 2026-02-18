package user

import "context"

type PreferenceRepository interface {
	GetPreference(ctx context.Context, userID string) (deliveryEnabled bool, err error)
	UpsertPreference(ctx context.Context, userID string, deliveryEnabled bool) error
}
