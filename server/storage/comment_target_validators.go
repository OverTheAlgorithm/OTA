package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TopicTargetValidator implements comment.TargetValidator for news topics
// (context_items rows). A comment can only attach to a context_item whose
// row still exists; deletion of the topic cascades to its comments via
// the FK contract, but the validator gives us an early 404 response.
type TopicTargetValidator struct {
	pool *pgxpool.Pool
}

// NewTopicTargetValidator constructs a validator.
func NewTopicTargetValidator(pool *pgxpool.Pool) *TopicTargetValidator {
	return &TopicTargetValidator{pool: pool}
}

// Exists reports whether the topic row exists.
func (v *TopicTargetValidator) Exists(ctx context.Context, id uuid.UUID) (bool, error) {
	var exists bool
	err := v.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM context_items WHERE id = $1)`, id).Scan(&exists)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("validate topic target: %w", err)
	}
	return exists, nil
}

// EditorPickTargetValidator implements comment.TargetValidator for editor
// picks (editor_posts rows). Only published posts can receive comments;
// drafts are author-only and not part of the public feed.
type EditorPickTargetValidator struct {
	pool *pgxpool.Pool
}

// NewEditorPickTargetValidator constructs a validator.
func NewEditorPickTargetValidator(pool *pgxpool.Pool) *EditorPickTargetValidator {
	return &EditorPickTargetValidator{pool: pool}
}

// Exists reports whether a published editor_post row exists.
func (v *EditorPickTargetValidator) Exists(ctx context.Context, id uuid.UUID) (bool, error) {
	var exists bool
	err := v.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM editor_posts WHERE id = $1 AND status = 'published')`,
		id,
	).Scan(&exists)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("validate editor pick target: %w", err)
	}
	return exists, nil
}
