package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"ota/domain/poll"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PollRepository is the PostgreSQL implementation of poll.Repository.
type PollRepository struct {
	pool *pgxpool.Pool
}

// NewPollRepository creates a new PollRepository.
func NewPollRepository(pool *pgxpool.Pool) *PollRepository {
	return &PollRepository{pool: pool}
}

// SavePollBatch inserts polls individually; duplicates (same context_item_id) are skipped.
func (r *PollRepository) SavePollBatch(ctx context.Context, polls []poll.Poll) error {
	for _, p := range polls {
		optsJSON, err := json.Marshal(p.Options)
		if err != nil {
			return fmt.Errorf("save poll batch: marshal options: %w", err)
		}
		_, err = r.pool.Exec(ctx,
			`INSERT INTO polls (id, context_item_id, question, options)
			 VALUES ($1, $2, $3, $4)
			 ON CONFLICT (context_item_id) DO NOTHING`,
			p.ID, p.ContextItemID, p.Question, optsJSON,
		)
		if err != nil {
			return fmt.Errorf("save poll batch: item %s: %w", p.ContextItemID, err)
		}
	}
	return nil
}

// GetByContextItemID returns the poll for an article, or nil if none.
func (r *PollRepository) GetByContextItemID(ctx context.Context, contextItemID uuid.UUID) (*poll.Poll, error) {
	var p poll.Poll
	var optsJSON []byte
	err := r.pool.QueryRow(ctx,
		`SELECT id, context_item_id, question, options, created_at, updated_at
		 FROM polls WHERE context_item_id = $1`,
		contextItemID,
	).Scan(&p.ID, &p.ContextItemID, &p.Question, &optsJSON, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get poll by context_item_id: %w", err)
	}
	if err := json.Unmarshal(optsJSON, &p.Options); err != nil {
		return nil, fmt.Errorf("get poll by context_item_id: unmarshal options: %w", err)
	}
	return &p, nil
}

// CountRawTallies returns per-option vote counts (sparse — only option_index values with >0 votes).
func (r *PollRepository) CountRawTallies(ctx context.Context, pollID uuid.UUID) ([]poll.VoteTally, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT option_index, COUNT(*) FROM poll_votes WHERE poll_id = $1 GROUP BY option_index`,
		pollID,
	)
	if err != nil {
		return nil, fmt.Errorf("count raw tallies: %w", err)
	}
	defer rows.Close()
	out := []poll.VoteTally{}
	for rows.Next() {
		var t poll.VoteTally
		if err := rows.Scan(&t.OptionIndex, &t.Count); err != nil {
			return nil, fmt.Errorf("count raw tallies: scan: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("count raw tallies: rows: %w", err)
	}
	return out, nil
}

// GetUserVoteIndex returns the user's option_index for a poll, or nil if not voted.
func (r *PollRepository) GetUserVoteIndex(ctx context.Context, userID string, pollID uuid.UUID) (*int, error) {
	var idx int
	err := r.pool.QueryRow(ctx,
		`SELECT option_index FROM poll_votes WHERE user_id = $1 AND poll_id = $2`,
		userID, pollID,
	).Scan(&idx)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user vote index: %w", err)
	}
	return &idx, nil
}

// InsertVote records a single vote. Maps 23505 unique violation → poll.ErrAlreadyVoted.
func (r *PollRepository) InsertVote(ctx context.Context, userID string, pollID uuid.UUID, optionIndex int) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO poll_votes (poll_id, user_id, option_index) VALUES ($1, $2, $3)`,
		pollID, userID, optionIndex,
	)
	if err != nil {
		var pg *pgconn.PgError
		if errors.As(err, &pg) && pg.Code == "23505" {
			return poll.ErrAlreadyVoted
		}
		return fmt.Errorf("insert vote: %w", err)
	}
	return nil
}

// UpdatePollAndMaybeResetVotes updates the poll; if resetVotes=true, clears poll_votes in the same tx.
func (r *PollRepository) UpdatePollAndMaybeResetVotes(ctx context.Context, pollID uuid.UUID, question string, options []string, resetVotes bool) error {
	optsJSON, err := json.Marshal(options)
	if err != nil {
		return fmt.Errorf("update poll: marshal options: %w", err)
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("update poll: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if resetVotes {
		if _, err := tx.Exec(ctx, `DELETE FROM poll_votes WHERE poll_id = $1`, pollID); err != nil {
			return fmt.Errorf("update poll: delete votes: %w", err)
		}
	}
	if _, err := tx.Exec(ctx,
		`UPDATE polls SET question = $1, options = $2, updated_at = NOW() WHERE id = $3`,
		question, optsJSON, pollID,
	); err != nil {
		return fmt.Errorf("update poll: update: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("update poll: commit: %w", err)
	}
	return nil
}

// DeleteByContextItemID removes the poll (and its votes via ON DELETE CASCADE).
func (r *PollRepository) DeleteByContextItemID(ctx context.Context, contextItemID uuid.UUID) error {
	if _, err := r.pool.Exec(ctx, `DELETE FROM polls WHERE context_item_id = $1`, contextItemID); err != nil {
		return fmt.Errorf("delete poll by context_item_id: %w", err)
	}
	return nil
}
