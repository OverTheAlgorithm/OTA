package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ota/domain/communitytrend"
)

// CTMemeRepository implements communitytrend.MemeRepository.
type CTMemeRepository struct {
	pool *pgxpool.Pool
}

func NewCTMemeRepository(pool *pgxpool.Pool) *CTMemeRepository {
	return &CTMemeRepository{pool: pool}
}

const ctMemeCols = `id, name, aliases, status, created_via, created_at`

func scanMeme(row interface{ Scan(...any) error }) (communitytrend.Meme, error) {
	var m communitytrend.Meme
	err := row.Scan(&m.ID, &m.Name, &m.Aliases, &m.Status, &m.CreatedVia, &m.CreatedAt)
	return m, err
}

// UpsertCandidate inserts a candidate or bumps its hit_count. Blacklisted
// expressions are silently ignored (never re-proposed, decisions.md D-012).
func (r *CTMemeRepository) UpsertCandidate(ctx context.Context, expression string, date time.Time) error {
	var blacklisted bool
	if err := r.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM ct_meme_blacklist WHERE expression=$1)`, expression).Scan(&blacklisted); err != nil {
		return fmt.Errorf("check blacklist: %w", err)
	}
	if blacklisted {
		return nil
	}
	_, err := r.pool.Exec(ctx,
		`INSERT INTO ct_meme_candidates (expression, hit_count, first_seen, last_seen)
		 VALUES ($1, 1, $2, $2)
		 ON CONFLICT (expression) DO UPDATE
		   SET hit_count = ct_meme_candidates.hit_count + 1, last_seen = $2`,
		expression, date)
	if err != nil {
		return fmt.Errorf("upsert candidate: %w", err)
	}
	return nil
}

func (r *CTMemeRepository) ListCandidates(ctx context.Context) ([]communitytrend.CandidateRow, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, expression, hit_count, first_seen, last_seen
		 FROM ct_meme_candidates ORDER BY hit_count DESC, last_seen DESC`)
	if err != nil {
		return nil, fmt.Errorf("list candidates: %w", err)
	}
	defer rows.Close()

	var out []communitytrend.CandidateRow
	for rows.Next() {
		var c communitytrend.CandidateRow
		if err := rows.Scan(&c.ID, &c.Expression, &c.HitCount, &c.FirstSeen, &c.LastSeen); err != nil {
			return nil, fmt.Errorf("scan candidate: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate candidates: %w", err)
	}
	return out, nil
}

// RejectCandidate deletes the candidate and blacklists its expression forever.
func (r *CTMemeRepository) RejectCandidate(ctx context.Context, id int) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var expression string
	err = tx.QueryRow(ctx, `DELETE FROM ct_meme_candidates WHERE id=$1 RETURNING expression`, id).Scan(&expression)
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("candidate not found")
	}
	if err != nil {
		return fmt.Errorf("delete candidate: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO ct_meme_blacklist (expression) VALUES ($1) ON CONFLICT DO NOTHING`, expression); err != nil {
		return fmt.Errorf("blacklist expression: %w", err)
	}
	return tx.Commit(ctx)
}

// PromoteCandidate creates a confirmed meme from a candidate and removes it.
func (r *CTMemeRepository) PromoteCandidate(ctx context.Context, id int, name string, aliases []string) (communitytrend.Meme, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return communitytrend.Meme{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `DELETE FROM ct_meme_candidates WHERE id=$1`, id)
	if err != nil {
		return communitytrend.Meme{}, fmt.Errorf("delete candidate: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return communitytrend.Meme{}, fmt.Errorf("candidate not found")
	}

	if aliases == nil {
		aliases = []string{}
	}
	m, err := scanMeme(tx.QueryRow(ctx,
		`INSERT INTO ct_memes (name, aliases, created_via) VALUES ($1, $2, 'promote') RETURNING `+ctMemeCols,
		name, aliases))
	if err != nil {
		return communitytrend.Meme{}, fmt.Errorf("insert promoted meme: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return communitytrend.Meme{}, fmt.Errorf("commit promote: %w", err)
	}
	return m, nil
}

func (r *CTMemeRepository) CreateMeme(ctx context.Context, name string, aliases []string) (communitytrend.Meme, error) {
	if aliases == nil {
		aliases = []string{}
	}
	m, err := scanMeme(r.pool.QueryRow(ctx,
		`INSERT INTO ct_memes (name, aliases, created_via) VALUES ($1, $2, 'manual') RETURNING `+ctMemeCols,
		name, aliases))
	if err != nil {
		return communitytrend.Meme{}, fmt.Errorf("create meme: %w", err)
	}
	return m, nil
}

func (r *CTMemeRepository) ListMemes(ctx context.Context, includeRetired bool) ([]communitytrend.Meme, error) {
	query := `SELECT ` + ctMemeCols + ` FROM ct_memes`
	if !includeRetired {
		query += ` WHERE status='active'`
	}
	query += ` ORDER BY created_at DESC`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list memes: %w", err)
	}
	defer rows.Close()

	var out []communitytrend.Meme
	for rows.Next() {
		m, err := scanMeme(rows)
		if err != nil {
			return nil, fmt.Errorf("scan meme: %w", err)
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate memes: %w", err)
	}
	return out, nil
}

func (r *CTMemeRepository) UpdateMeme(ctx context.Context, id int, name string, aliases []string) (communitytrend.Meme, error) {
	if aliases == nil {
		aliases = []string{}
	}
	m, err := scanMeme(r.pool.QueryRow(ctx,
		`UPDATE ct_memes SET name=$2, aliases=$3 WHERE id=$1 RETURNING `+ctMemeCols,
		id, name, aliases))
	if errors.Is(err, pgx.ErrNoRows) {
		return communitytrend.Meme{}, fmt.Errorf("meme not found")
	}
	if err != nil {
		return communitytrend.Meme{}, fmt.Errorf("update meme: %w", err)
	}
	return m, nil
}

func (r *CTMemeRepository) RetireMeme(ctx context.Context, id int) error {
	tag, err := r.pool.Exec(ctx, `UPDATE ct_memes SET status='retired' WHERE id=$1`, id)
	if err != nil {
		return fmt.Errorf("retire meme: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("meme not found")
	}
	return nil
}
