package storage

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PreferenceRepository struct {
	pool *pgxpool.Pool
}

func NewPreferenceRepository(pool *pgxpool.Pool) *PreferenceRepository {
	return &PreferenceRepository{pool: pool}
}

func (r *PreferenceRepository) GetPreference(ctx context.Context, userID string) (bool, error) {
	var enabled bool
	err := r.pool.QueryRow(ctx,
		`SELECT delivery_enabled FROM user_preferences WHERE user_id = $1`,
		userID,
	).Scan(&enabled)
	if errors.Is(err, pgx.ErrNoRows) {
		return true, nil
	}
	return enabled, err
}

func (r *PreferenceRepository) UpsertPreference(ctx context.Context, userID string, deliveryEnabled bool) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO user_preferences (user_id, delivery_enabled)
		 VALUES ($1, $2)
		 ON CONFLICT (user_id) DO UPDATE
		 SET delivery_enabled = $2, updated_at = NOW()`,
		userID, deliveryEnabled,
	)
	return err
}
