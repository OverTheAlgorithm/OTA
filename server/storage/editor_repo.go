package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ota/domain/editor"
)

// EditorRepository implements editor.Repository over PostgreSQL.
type EditorRepository struct {
	pool *pgxpool.Pool
}

func NewEditorRepository(pool *pgxpool.Pool) *EditorRepository {
	return &EditorRepository{pool: pool}
}

// columns used for full Post scans.
const editorPostCols = `id, author_id, title, content_html, content_text, first_image_url, status, published_at, created_at, updated_at`

func scanPost(row pgx.Row) (editor.Post, error) {
	var p editor.Post
	err := row.Scan(&p.ID, &p.AuthorID, &p.Title, &p.ContentHTML, &p.ContentText, &p.FirstImageURL, &p.Status, &p.PublishedAt, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

func (r *EditorRepository) Create(ctx context.Context, p editor.Post) (editor.Post, error) {
	const query = `
		INSERT INTO editor_posts (author_id, title, content_html, content_text, first_image_url, status, published_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING ` + editorPostCols

	row := r.pool.QueryRow(ctx, query, p.AuthorID, p.Title, p.ContentHTML, p.ContentText, p.FirstImageURL, p.Status, p.PublishedAt)
	result, err := scanPost(row)
	if err != nil {
		return editor.Post{}, fmt.Errorf("create editor_post: %w", err)
	}
	return result, nil
}

func (r *EditorRepository) Update(ctx context.Context, p editor.Post) (editor.Post, error) {
	const query = `
		UPDATE editor_posts
		SET title = $2,
		    content_html = $3,
		    content_text = $4,
		    first_image_url = $5,
		    status = $6,
		    published_at = $7,
		    updated_at = NOW()
		WHERE id = $1
		RETURNING ` + editorPostCols

	row := r.pool.QueryRow(ctx, query, p.ID, p.Title, p.ContentHTML, p.ContentText, p.FirstImageURL, p.Status, p.PublishedAt)
	result, err := scanPost(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return editor.Post{}, editor.ErrPostNotFound
		}
		return editor.Post{}, fmt.Errorf("update editor_post: %w", err)
	}
	return result, nil
}

func (r *EditorRepository) Delete(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM editor_posts WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete editor_post: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return editor.ErrPostNotFound
	}
	return nil
}

func (r *EditorRepository) FindByID(ctx context.Context, id string) (editor.Post, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+editorPostCols+` FROM editor_posts WHERE id = $1`, id)
	p, err := scanPost(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return editor.Post{}, editor.ErrPostNotFound
		}
		return editor.Post{}, fmt.Errorf("find editor_post: %w", err)
	}
	return p, nil
}

func (r *EditorRepository) FindDraftByAuthor(ctx context.Context, authorID string) (editor.Post, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+editorPostCols+` FROM editor_posts WHERE author_id = $1 AND status = 'draft' LIMIT 1`,
		authorID,
	)
	p, err := scanPost(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return editor.Post{}, editor.ErrPostNotFound
		}
		return editor.Post{}, fmt.Errorf("find draft editor_post: %w", err)
	}
	return p, nil
}

func (r *EditorRepository) ListByAuthor(ctx context.Context, authorID string) ([]editor.Post, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+editorPostCols+` FROM editor_posts WHERE author_id = $1 ORDER BY created_at DESC`,
		authorID,
	)
	if err != nil {
		return nil, fmt.Errorf("list editor_posts by author: %w", err)
	}
	defer rows.Close()

	var out []editor.Post
	for rows.Next() {
		p, err := scanPost(rows)
		if err != nil {
			return nil, fmt.Errorf("scan editor_post: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *EditorRepository) ListAllForAdmin(ctx context.Context) ([]editor.Post, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+editorPostCols+` FROM editor_posts ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list editor_posts: %w", err)
	}
	defer rows.Close()

	var out []editor.Post
	for rows.Next() {
		p, err := scanPost(rows)
		if err != nil {
			return nil, fmt.Errorf("scan editor_post: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *EditorRepository) ListPublishedCards(ctx context.Context, limit, offset int) ([]editor.PublicCard, error) {
	// Author byline preference: pen_name (trimmed, non-empty) ➜ nickname ➜ "".
	// The btrim character set strips ASCII whitespace (space/tab/LF/CR/VT/FF)
	// so even a value like "  \n " falls through to nickname — defence in
	// depth alongside the CHECK constraint on users.pen_name.
	const query = `
		SELECT p.id, p.author_id,
		       COALESCE(NULLIF(btrim(u.pen_name, E' \t\n\r\v\f'), ''), u.nickname, ''),
		       p.title, p.content_text, p.first_image_url, p.published_at
		FROM editor_posts p
		LEFT JOIN users u ON u.id = p.author_id
		WHERE p.status = 'published' AND p.published_at IS NOT NULL
		ORDER BY p.published_at DESC
		LIMIT $1 OFFSET $2`

	rows, err := r.pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list published editor_posts: %w", err)
	}
	defer rows.Close()

	var out []editor.PublicCard
	for rows.Next() {
		var c editor.PublicCard
		if err := rows.Scan(&c.ID, &c.AuthorID, &c.AuthorName, &c.Title, &c.Excerpt, &c.FirstImageURL, &c.PublishedAt); err != nil {
			return nil, fmt.Errorf("scan card: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *EditorRepository) GetPublishedByID(ctx context.Context, id string) (editor.PublicPost, error) {
	const query = `
		SELECT p.id, p.author_id,
		       COALESCE(NULLIF(btrim(u.pen_name, E' \t\n\r\v\f'), ''), u.nickname, ''),
		       p.title, p.content_html, p.first_image_url, p.published_at, p.updated_at
		FROM editor_posts p
		LEFT JOIN users u ON u.id = p.author_id
		WHERE p.id = $1 AND p.status = 'published' AND p.published_at IS NOT NULL`

	var p editor.PublicPost
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&p.ID, &p.AuthorID, &p.AuthorName, &p.Title, &p.ContentHTML, &p.FirstImageURL, &p.PublishedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return editor.PublicPost{}, editor.ErrPostNotFound
		}
		return editor.PublicPost{}, fmt.Errorf("get published editor_post: %w", err)
	}
	return p, nil
}

func (r *EditorRepository) CountPublished(ctx context.Context) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM editor_posts WHERE status = 'published'`).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count published editor_posts: %w", err)
	}
	return n, nil
}
