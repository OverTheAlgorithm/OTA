package storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"ota/domain/terms"
)

// TermsRepository implements terms.Repository using PostgreSQL.
type TermsRepository struct {
	pool *pgxpool.Pool
}

// NewTermsRepository creates a new terms repository.
func NewTermsRepository(pool *pgxpool.Pool) *TermsRepository {
	return &TermsRepository{pool: pool}
}

func (r *TermsRepository) Create(ctx context.Context, t terms.Term) (terms.Term, error) {
	query := `
		INSERT INTO terms (title, description, url, active, required, version)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, title, description, url, active, required, version, created_at`

	var result terms.Term
	err := r.pool.QueryRow(ctx, query,
		t.Title, t.Description, t.URL, t.Active, t.Required, t.Version,
	).Scan(
		&result.ID, &result.Title, &result.Description, &result.URL,
		&result.Active, &result.Required, &result.Version, &result.CreatedAt,
	)
	if err != nil {
		return terms.Term{}, fmt.Errorf("create term: %w", err)
	}
	return result, nil
}

func (r *TermsRepository) ListAll(ctx context.Context) ([]terms.Term, error) {
	query := `SELECT id, title, description, url, active, required, version, created_at
		FROM terms ORDER BY created_at DESC`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list all terms: %w", err)
	}
	defer rows.Close()

	var result []terms.Term
	for rows.Next() {
		var t terms.Term
		if err := rows.Scan(&t.ID, &t.Title, &t.Description, &t.URL, &t.Active, &t.Required, &t.Version, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan term: %w", err)
		}
		result = append(result, t)
	}
	return result, nil
}

func (r *TermsRepository) ListActive(ctx context.Context) ([]terms.Term, error) {
	query := `SELECT id, title, description, url, active, required, version, created_at
		FROM terms WHERE active = true ORDER BY created_at DESC`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list active terms: %w", err)
	}
	defer rows.Close()

	var result []terms.Term
	for rows.Next() {
		var t terms.Term
		if err := rows.Scan(&t.ID, &t.Title, &t.Description, &t.URL, &t.Active, &t.Required, &t.Version, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan term: %w", err)
		}
		result = append(result, t)
	}
	return result, nil
}

func (r *TermsRepository) FindActiveRequired(ctx context.Context) ([]terms.Term, error) {
	query := `SELECT id, title, description, url, active, required, version, created_at
		FROM terms WHERE active = true AND required = true`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("find active required terms: %w", err)
	}
	defer rows.Close()

	var result []terms.Term
	for rows.Next() {
		var t terms.Term
		if err := rows.Scan(&t.ID, &t.Title, &t.Description, &t.URL, &t.Active, &t.Required, &t.Version, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan term: %w", err)
		}
		result = append(result, t)
	}
	return result, nil
}

func (r *TermsRepository) UpdateActive(ctx context.Context, termID string, active bool) error {
	tag, err := r.pool.Exec(ctx, `UPDATE terms SET active = $2 WHERE id = $1`, termID, active)
	if err != nil {
		return fmt.Errorf("update term active: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("term not found")
	}
	return nil
}

func (r *TermsRepository) SaveConsents(ctx context.Context, userID string, termIDs []string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	for _, termID := range termIDs {
		_, err := tx.Exec(ctx,
			`INSERT INTO user_term_consents (user_id, term_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
			userID, termID,
		)
		if err != nil {
			return fmt.Errorf("insert consent for term %s: %w", termID, err)
		}
	}

	return tx.Commit(ctx)
}

func (r *TermsRepository) GetUserConsents(ctx context.Context, userID string) ([]terms.UserTermConsent, error) {
	query := `SELECT id, user_id, term_id, created_at FROM user_term_consents WHERE user_id = $1 ORDER BY created_at`

	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("get user consents: %w", err)
	}
	defer rows.Close()

	var result []terms.UserTermConsent
	for rows.Next() {
		var c terms.UserTermConsent
		if err := rows.Scan(&c.ID, &c.UserID, &c.TermID, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan consent: %w", err)
		}
		result = append(result, c)
	}
	return result, nil
}
