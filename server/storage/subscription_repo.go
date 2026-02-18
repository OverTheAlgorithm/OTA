package storage

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type SubscriptionRepository struct {
	pool *pgxpool.Pool
}

func NewSubscriptionRepository(pool *pgxpool.Pool) *SubscriptionRepository {
	return &SubscriptionRepository{pool: pool}
}

func (r *SubscriptionRepository) GetSubscriptions(ctx context.Context, userID string) ([]string, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT category FROM user_subscriptions WHERE user_id = $1 ORDER BY created_at`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []string
	for rows.Next() {
		var cat string
		if err := rows.Scan(&cat); err != nil {
			return nil, err
		}
		categories = append(categories, cat)
	}
	if categories == nil {
		categories = []string{}
	}
	return categories, nil
}

func (r *SubscriptionRepository) AddSubscription(ctx context.Context, userID, category string) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO user_subscriptions (user_id, category)
		 VALUES ($1, $2)
		 ON CONFLICT (user_id, category) DO NOTHING`,
		userID, category,
	)
	return err
}

func (r *SubscriptionRepository) DeleteSubscription(ctx context.Context, userID, category string) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM user_subscriptions WHERE user_id = $1 AND category = $2`,
		userID, category,
	)
	return err
}
