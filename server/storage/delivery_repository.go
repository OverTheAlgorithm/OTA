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
// Joins users, user_delivery_channels, and user_subscriptions tables
func (r *DeliveryRepository) GetEligibleUsers(ctx context.Context) ([]delivery.EligibleUser, error) {
	query := `
		SELECT
			u.id,
			u.email,
			COALESCE(
				ARRAY_AGG(DISTINCT us.category) FILTER (WHERE us.category IS NOT NULL),
				ARRAY[]::VARCHAR[]
			) AS subscriptions,
			COALESCE(
				ARRAY_AGG(DISTINCT udc.channel) FILTER (WHERE udc.channel IS NOT NULL AND udc.enabled = true),
				ARRAY[]::VARCHAR[]
			) AS enabled_channels
		FROM users u
		INNER JOIN user_delivery_channels udc ON u.id = udc.user_id
		LEFT JOIN user_subscriptions us ON u.id = us.user_id
		WHERE udc.enabled = true
		  AND u.email IS NOT NULL
		  AND u.email != ''
		GROUP BY u.id, u.email
		HAVING COUNT(DISTINCT udc.channel) FILTER (WHERE udc.enabled = true) > 0
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
		var channels []string

		err := rows.Scan(&user.UserID, &user.Email, &subs, &channels)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user row: %w", err)
		}

		user.Subscriptions = subs

		// Convert string channels to DeliveryChannel type
		user.EnabledChannels = make([]delivery.DeliveryChannel, 0, len(channels))
		for _, ch := range channels {
			user.EnabledChannels = append(user.EnabledChannels, delivery.DeliveryChannel(ch))
		}

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

// GetUserDeliveryChannels returns all delivery channels for a user
func (r *DeliveryRepository) GetUserDeliveryChannels(ctx context.Context, userID string) ([]delivery.UserDeliveryChannel, error) {
	query := `
		SELECT id, user_id, channel, enabled, created_at, updated_at
		FROM user_delivery_channels
		WHERE user_id = $1
		ORDER BY channel
	`

	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query user delivery channels: %w", err)
	}
	defer rows.Close()

	var channels []delivery.UserDeliveryChannel
	for rows.Next() {
		var ch delivery.UserDeliveryChannel
		var channelStr string

		err := rows.Scan(&ch.ID, &ch.UserID, &channelStr, &ch.Enabled, &ch.CreatedAt, &ch.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan channel row: %w", err)
		}

		ch.Channel = delivery.DeliveryChannel(channelStr)
		channels = append(channels, ch)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating channel rows: %w", err)
	}

	return channels, nil
}

// UpsertUserDeliveryChannel creates or updates a user's channel preference
func (r *DeliveryRepository) UpsertUserDeliveryChannel(ctx context.Context, channel delivery.UserDeliveryChannel) error {
	query := `
		INSERT INTO user_delivery_channels (id, user_id, channel, enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
		ON CONFLICT (user_id, channel)
		DO UPDATE SET
			enabled = EXCLUDED.enabled,
			updated_at = NOW()
	`

	_, err := r.pool.Exec(ctx, query,
		channel.ID,
		channel.UserID,
		string(channel.Channel),
		channel.Enabled,
	)

	if err != nil {
		return fmt.Errorf("failed to upsert delivery channel: %w", err)
	}

	return nil
}
