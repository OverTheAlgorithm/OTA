package storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"ota/domain/delivery"
)

// DeliveryRepository implements delivery.Repository using PostgreSQL
type DeliveryRepository struct {
	pool *pgxpool.Pool
}

// NewDeliveryRepository creates a new PostgreSQL-based delivery repository
func NewDeliveryRepository(pool *pgxpool.Pool) *DeliveryRepository {
	return &DeliveryRepository{pool: pool}
}

// GetEligibleUsers returns all users who should receive messages
// Joins users, user_preferences, and user_subscriptions tables
func (r *DeliveryRepository) GetEligibleUsers(ctx context.Context) ([]delivery.EligibleUser, error) {
	query := `
		SELECT
			u.id,
			u.email,
			COALESCE(
				ARRAY_AGG(us.category) FILTER (WHERE us.category IS NOT NULL),
				ARRAY[]::VARCHAR[]
			) AS subscriptions
		FROM users u
		INNER JOIN user_preferences up ON u.id = up.user_id
		LEFT JOIN user_subscriptions us ON u.id = us.user_id
		WHERE up.delivery_enabled = true
		  AND u.email IS NOT NULL
		  AND u.email != ''
		GROUP BY u.id, u.email
		ORDER BY u.created_at
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query eligible users: %w", err)
	}
	defer rows.Close()

	var users []delivery.EligibleUser
	for rows.Next() {
		var user delivery.EligibleUser
		var subs []string

		err := rows.Scan(&user.UserID, &user.Email, &subs)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user row: %w", err)
		}

		user.Subscriptions = subs
		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating user rows: %w", err)
	}

	return users, nil
}

// LogDelivery records a delivery attempt
func (r *DeliveryRepository) LogDelivery(ctx context.Context, log delivery.DeliveryLog) error {
	query := `
		INSERT INTO delivery_logs (id, run_id, user_id, channel, status, error_message, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err := r.pool.Exec(ctx, query,
		log.ID,
		log.RunID,
		log.UserID,
		log.Channel,
		log.Status,
		log.ErrorMessage,
		log.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to insert delivery log: %w", err)
	}

	return nil
}

// HasDeliveryLog checks if a delivery was already attempted
func (r *DeliveryRepository) HasDeliveryLog(ctx context.Context, runID string, userID string, channel delivery.DeliveryChannel) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM delivery_logs
			WHERE run_id = $1 AND user_id = $2 AND channel = $3
		)
	`

	var exists bool
	err := r.pool.QueryRow(ctx, query, runID, userID, channel).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check delivery log: %w", err)
	}

	return exists, nil
}
