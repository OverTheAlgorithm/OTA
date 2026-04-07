package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"ota/domain/push"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ScheduledPushRepository implements push.ScheduledRepository using pgxpool.
type ScheduledPushRepository struct {
	pool *pgxpool.Pool
}

// NewScheduledPushRepository creates a new ScheduledPushRepository.
func NewScheduledPushRepository(pool *pgxpool.Pool) *ScheduledPushRepository {
	return &ScheduledPushRepository{pool: pool}
}

// Create inserts a new scheduled push and returns the persisted row.
func (r *ScheduledPushRepository) Create(ctx context.Context, p push.ScheduledPush) (push.ScheduledPush, error) {
	var dataJSON []byte
	if p.Data != nil {
		var err error
		dataJSON, err = json.Marshal(p.Data)
		if err != nil {
			return push.ScheduledPush{}, fmt.Errorf("marshal push data: %w", err)
		}
	}

	row := r.pool.QueryRow(ctx, `
		INSERT INTO scheduled_pushes (id, title, body, link, data, status, scheduled_at, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, title, body, link, data, status, scheduled_at, sent_at, error_message, created_by, created_at, updated_at
	`, p.ID, p.Title, p.Body, p.Link, dataJSON, p.Status, p.ScheduledAt, p.CreatedBy)

	return scanScheduledPush(row)
}

// GetByID returns a single scheduled push by ID.
func (r *ScheduledPushRepository) GetByID(ctx context.Context, id uuid.UUID) (push.ScheduledPush, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, title, body, link, data, status, scheduled_at, sent_at, error_message, created_by, created_at, updated_at
		FROM scheduled_pushes WHERE id = $1
	`, id)
	p, err := scanScheduledPush(row)
	if err != nil {
		return push.ScheduledPush{}, fmt.Errorf("get scheduled push by id: %w", err)
	}
	return p, nil
}

// Update overwrites a pending push's mutable fields. Returns error if not found or not pending.
func (r *ScheduledPushRepository) Update(ctx context.Context, p push.ScheduledPush) error {
	var dataJSON []byte
	if p.Data != nil {
		var err error
		dataJSON, err = json.Marshal(p.Data)
		if err != nil {
			return fmt.Errorf("marshal push data: %w", err)
		}
	}

	tag, err := r.pool.Exec(ctx, `
		UPDATE scheduled_pushes
		SET title = $2, body = $3, link = $4, data = $5, scheduled_at = $6, updated_at = now()
		WHERE id = $1 AND status = 'pending'
	`, p.ID, p.Title, p.Body, p.Link, dataJSON, p.ScheduledAt)
	if err != nil {
		return fmt.Errorf("update scheduled push: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("scheduled push not found or not in pending status")
	}
	return nil
}

// List returns all pushes, optionally filtered by status, ordered by created_at DESC.
func (r *ScheduledPushRepository) List(ctx context.Context, status *string) ([]push.ScheduledPush, error) {
	var rows interface {
		Next() bool
		Scan(...any) error
		Err() error
		Close()
	}

	if status != nil {
		r2, err := r.pool.Query(ctx, `
			SELECT id, title, body, link, data, status, scheduled_at, sent_at, error_message, created_by, created_at, updated_at
			FROM scheduled_pushes WHERE status = $1 ORDER BY created_at DESC
		`, *status)
		if err != nil {
			return nil, fmt.Errorf("list scheduled pushes by status: %w", err)
		}
		rows = r2
	} else {
		r2, err := r.pool.Query(ctx, `
			SELECT id, title, body, link, data, status, scheduled_at, sent_at, error_message, created_by, created_at, updated_at
			FROM scheduled_pushes ORDER BY created_at DESC
		`)
		if err != nil {
			return nil, fmt.Errorf("list scheduled pushes: %w", err)
		}
		rows = r2
	}
	defer rows.Close()

	return scanScheduledPushRows(rows)
}

// ListPending returns pending pushes that have a scheduled_at set (for scheduler reload on startup).
func (r *ScheduledPushRepository) ListPending(ctx context.Context) ([]push.ScheduledPush, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, title, body, link, data, status, scheduled_at, sent_at, error_message, created_by, created_at, updated_at
		FROM scheduled_pushes WHERE status = 'pending' AND scheduled_at IS NOT NULL
		ORDER BY scheduled_at ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list pending scheduled pushes: %w", err)
	}
	defer rows.Close()

	return scanScheduledPushRows(rows)
}

// MarkSent CAS-updates status to 'sent' WHERE status='pending'. Returns (true, nil) on success.
func (r *ScheduledPushRepository) MarkSent(ctx context.Context, id uuid.UUID, sentAt time.Time) (bool, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE scheduled_pushes
		SET status = 'sent', sent_at = $2, updated_at = now()
		WHERE id = $1 AND status = 'pending'
	`, id, sentAt)
	if err != nil {
		return false, fmt.Errorf("mark push sent: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// MarkFailed CAS-updates status to 'failed' WHERE status='pending'. Returns (true, nil) on success.
func (r *ScheduledPushRepository) MarkFailed(ctx context.Context, id uuid.UUID, errMsg string) (bool, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE scheduled_pushes
		SET status = 'failed', error_message = $2, updated_at = now()
		WHERE id = $1 AND status = 'pending'
	`, id, errMsg)
	if err != nil {
		return false, fmt.Errorf("mark push failed: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// MarkCancelled CAS-updates status to 'cancelled' WHERE status='pending'. Returns (true, nil) on success.
func (r *ScheduledPushRepository) MarkCancelled(ctx context.Context, id uuid.UUID) (bool, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE scheduled_pushes
		SET status = 'cancelled', updated_at = now()
		WHERE id = $1 AND status = 'pending'
	`, id)
	if err != nil {
		return false, fmt.Errorf("mark push cancelled: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// scanner is a common interface for both pgx.Row and pgx.Rows.
type scanner interface {
	Scan(dest ...any) error
}

func scanScheduledPush(s scanner) (push.ScheduledPush, error) {
	var p push.ScheduledPush
	var dataJSON []byte
	var errMsg *string

	if err := s.Scan(
		&p.ID, &p.Title, &p.Body, &p.Link,
		&dataJSON, &p.Status,
		&p.ScheduledAt, &p.SentAt, &errMsg,
		&p.CreatedBy, &p.CreatedAt, &p.UpdatedAt,
	); err != nil {
		return push.ScheduledPush{}, fmt.Errorf("scan scheduled push: %w", err)
	}

	if dataJSON != nil {
		if err := json.Unmarshal(dataJSON, &p.Data); err != nil {
			return push.ScheduledPush{}, fmt.Errorf("unmarshal push data: %w", err)
		}
	}
	if errMsg != nil {
		p.ErrorMessage = *errMsg
	}
	return p, nil
}

type rowsScanner interface {
	Next() bool
	Scan(...any) error
	Err() error
	Close()
}

func scanScheduledPushRows(rows rowsScanner) ([]push.ScheduledPush, error) {
	var pushes []push.ScheduledPush
	for rows.Next() {
		p, err := scanScheduledPush(rows)
		if err != nil {
			return nil, err
		}
		pushes = append(pushes, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate scheduled pushes: %w", err)
	}
	return pushes, nil
}
