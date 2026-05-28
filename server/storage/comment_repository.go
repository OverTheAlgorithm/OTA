package storage

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ota/domain/comment"
)

// CommentRepository implements comment.Repository over PostgreSQL.
type CommentRepository struct {
	pool *pgxpool.Pool
}

// NewCommentRepository constructs a repository.
func NewCommentRepository(pool *pgxpool.Pool) *CommentRepository {
	return &CommentRepository{pool: pool}
}

// commentSelectCols intentionally omits pen_name: comments always show the
// user's nickname. pen_name is reserved for editor-pick author bylines.
const commentSelectCols = `
    c.id, c.target_type, c.target_id, c.user_id, c.group_id, c.parent_id,
    c.depth, c.rank_key, c.content, c.likes_count, c.dislikes_count,
    c.edited_at, c.deleted_at, c.created_at,
    COALESCE(u.nickname, '') AS nickname,
    COALESCE(u.profile_image, '') AS profile_image
`

func scanComment(row pgx.Row) (comment.Comment, error) {
	var c comment.Comment
	var parentID *uuid.UUID
	var editedAt, deletedAt *time.Time
	err := row.Scan(
		&c.ID, &c.TargetType, &c.TargetID, &c.UserID, &c.GroupID, &parentID,
		&c.Depth, &c.RankKey, &c.Content, &c.LikesCount, &c.DislikesCount,
		&editedAt, &deletedAt, &c.CreatedAt,
		&c.AuthorNickname, &c.AuthorProfileImage,
	)
	if err != nil {
		return comment.Comment{}, err
	}
	c.ParentID = parentID
	c.EditedAt = editedAt
	c.DeletedAt = deletedAt
	return c, nil
}

// InsertRoot creates a depth-0 comment. The caller pre-populates an ID and
// uses it as both id and group_id; the DB enforces the same via CHECK
// constraints in the schema where applicable.
func (r *CommentRepository) InsertRoot(ctx context.Context, c comment.Comment) (comment.Comment, error) {
	const query = `
        INSERT INTO comments (id, target_type, target_id, user_id, group_id, parent_id, depth, rank_key, content)
        VALUES ($1, $2, $3, $4, $5, NULL, 0, $6, $7)
        RETURNING id`
	if _, err := r.pool.Exec(ctx, query,
		c.ID, string(c.TargetType), c.TargetID, c.UserID, c.GroupID, c.RankKey, c.Content,
	); err != nil {
		return comment.Comment{}, fmt.Errorf("insert root comment: %w", err)
	}
	return r.GetByID(ctx, c.ID)
}

// InsertReply creates a depth-1 comment under an existing group.
func (r *CommentRepository) InsertReply(ctx context.Context, c comment.Comment) (comment.Comment, error) {
	const query = `
        INSERT INTO comments (id, target_type, target_id, user_id, group_id, parent_id, depth, rank_key, content)
        VALUES ($1, $2, $3, $4, $5, $6, 1, $7, $8)`
	if _, err := r.pool.Exec(ctx, query,
		c.ID, string(c.TargetType), c.TargetID, c.UserID, c.GroupID, c.ParentID, c.RankKey, c.Content,
	); err != nil {
		return comment.Comment{}, fmt.Errorf("insert reply comment: %w", err)
	}
	return r.GetByID(ctx, c.ID)
}

// GetByID fetches one comment with author fields hydrated.
func (r *CommentRepository) GetByID(ctx context.Context, id uuid.UUID) (comment.Comment, error) {
	const query = `
        SELECT ` + commentSelectCols + `
        FROM comments c
        LEFT JOIN users u ON u.id = c.user_id
        WHERE c.id = $1`
	row := r.pool.QueryRow(ctx, query, id)
	c, err := scanComment(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return comment.Comment{}, comment.ErrCommentNotFound
		}
		return comment.Comment{}, fmt.Errorf("get comment: %w", err)
	}
	return c, nil
}

// rootCursor encodes the keyset cursor for root listing.
//
// Popular cursor: "likes:created:id" base64-encoded.
// Recent cursor:  "created:id" base64-encoded.
type rootCursor struct {
	Likes     int
	CreatedAt time.Time
	ID        uuid.UUID
}

func (c rootCursor) encode(sort comment.SortOrder) string {
	var raw string
	switch sort {
	case comment.SortPopular:
		raw = fmt.Sprintf("p|%d|%s|%s", c.Likes, c.CreatedAt.UTC().Format(time.RFC3339Nano), c.ID.String())
	default:
		raw = fmt.Sprintf("r|%s|%s", c.CreatedAt.UTC().Format(time.RFC3339Nano), c.ID.String())
	}
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeRootCursor(s string) (rootCursor, comment.SortOrder, error) {
	if s == "" {
		return rootCursor{}, "", nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return rootCursor{}, "", fmt.Errorf("decode cursor: %w", err)
	}
	parts := strings.Split(string(raw), "|")
	switch parts[0] {
	case "p":
		if len(parts) != 4 {
			return rootCursor{}, "", fmt.Errorf("invalid popular cursor")
		}
		var likes int
		if _, err := fmt.Sscanf(parts[1], "%d", &likes); err != nil {
			return rootCursor{}, "", fmt.Errorf("invalid likes in cursor: %w", err)
		}
		ts, err := time.Parse(time.RFC3339Nano, parts[2])
		if err != nil {
			return rootCursor{}, "", fmt.Errorf("invalid created_at: %w", err)
		}
		id, err := uuid.Parse(parts[3])
		if err != nil {
			return rootCursor{}, "", fmt.Errorf("invalid id: %w", err)
		}
		return rootCursor{Likes: likes, CreatedAt: ts, ID: id}, comment.SortPopular, nil
	case "r":
		if len(parts) != 3 {
			return rootCursor{}, "", fmt.Errorf("invalid recent cursor")
		}
		ts, err := time.Parse(time.RFC3339Nano, parts[1])
		if err != nil {
			return rootCursor{}, "", fmt.Errorf("invalid created_at: %w", err)
		}
		id, err := uuid.Parse(parts[2])
		if err != nil {
			return rootCursor{}, "", fmt.Errorf("invalid id: %w", err)
		}
		return rootCursor{CreatedAt: ts, ID: id}, comment.SortRecent, nil
	default:
		return rootCursor{}, "", fmt.Errorf("unknown cursor variant %q", parts[0])
	}
}

// ListRoots returns one page of depth-0 comments for a target.
func (r *CommentRepository) ListRoots(ctx context.Context, target comment.TargetType, targetID uuid.UUID, sort comment.SortOrder, cursor string, limit int) (comment.RootPage, error) {
	cur, cursorSort, err := decodeRootCursor(cursor)
	if err != nil {
		return comment.RootPage{}, err
	}
	if cursorSort != "" && cursorSort != sort {
		// Cursor was produced for a different sort; ignore it rather than
		// erroring so toggling sort just resets pagination.
		cur = rootCursor{}
	}

	rows, err := r.queryRoots(ctx, target, targetID, sort, cur, cursor != "" && cursorSort == sort, limit)
	if err != nil {
		return comment.RootPage{}, err
	}
	defer rows.Close()

	items := make([]comment.Comment, 0, limit)
	for rows.Next() {
		c, err := scanComment(rows)
		if err != nil {
			return comment.RootPage{}, fmt.Errorf("scan root: %w", err)
		}
		items = append(items, c)
	}
	if err := rows.Err(); err != nil {
		return comment.RootPage{}, err
	}

	var next string
	if len(items) > 0 {
		// We over-fetch by one if there could be more pages. Simpler:
		// derive next cursor from the last item and require the caller to
		// detect "no more" by comparing item count to limit.
		last := items[len(items)-1]
		next = rootCursor{Likes: last.LikesCount, CreatedAt: last.CreatedAt, ID: last.ID}.encode(sort)
	}
	if len(items) < limit {
		next = "" // last page
	}
	return comment.RootPage{Items: items, NextCursor: next}, nil
}

func (r *CommentRepository) queryRoots(ctx context.Context, target comment.TargetType, targetID uuid.UUID, sort comment.SortOrder, cur rootCursor, hasCursor bool, limit int) (pgx.Rows, error) {
	base := `
        SELECT ` + commentSelectCols + `
        FROM comments c
        LEFT JOIN users u ON u.id = c.user_id
        WHERE c.target_type = $1 AND c.target_id = $2 AND c.depth = 0`

	switch sort {
	case comment.SortRecent:
		if hasCursor {
			return r.pool.Query(ctx,
				base+` AND (c.created_at, c.id) < ($3, $4)
                  ORDER BY c.created_at DESC, c.id DESC
                  LIMIT $5`,
				string(target), targetID, cur.CreatedAt, cur.ID, limit,
			)
		}
		return r.pool.Query(ctx,
			base+` ORDER BY c.created_at DESC, c.id DESC LIMIT $3`,
			string(target), targetID, limit,
		)
	default: // popular
		if hasCursor {
			return r.pool.Query(ctx,
				base+` AND (c.likes_count, c.created_at, c.id) < ($3, $4, $5)
                  ORDER BY c.likes_count DESC, c.created_at DESC, c.id DESC
                  LIMIT $6`,
				string(target), targetID, cur.Likes, cur.CreatedAt, cur.ID, limit,
			)
		}
		return r.pool.Query(ctx,
			base+` ORDER BY c.likes_count DESC, c.created_at DESC, c.id DESC LIMIT $3`,
			string(target), targetID, limit,
		)
	}
}

// ListReplies returns one page of depth-1 comments under a group.
func (r *CommentRepository) ListReplies(ctx context.Context, groupID uuid.UUID, cursor string, limit int) (comment.ReplyPage, error) {
	base := `
        SELECT ` + commentSelectCols + `
        FROM comments c
        LEFT JOIN users u ON u.id = c.user_id
        WHERE c.group_id = $1 AND c.depth = 1`

	var rows pgx.Rows
	var err error
	if cursor == "" {
		rows, err = r.pool.Query(ctx,
			base+` ORDER BY c.rank_key ASC LIMIT $2`,
			groupID, limit,
		)
	} else {
		rows, err = r.pool.Query(ctx,
			base+` AND c.rank_key > $2 ORDER BY c.rank_key ASC LIMIT $3`,
			groupID, cursor, limit,
		)
	}
	if err != nil {
		return comment.ReplyPage{}, fmt.Errorf("list replies: %w", err)
	}
	defer rows.Close()

	items := make([]comment.Comment, 0, limit)
	for rows.Next() {
		c, err := scanComment(rows)
		if err != nil {
			return comment.ReplyPage{}, fmt.Errorf("scan reply: %w", err)
		}
		items = append(items, c)
	}
	if err := rows.Err(); err != nil {
		return comment.ReplyPage{}, err
	}

	var next string
	if len(items) == limit && limit > 0 {
		next = items[len(items)-1].RankKey
	}
	return comment.ReplyPage{Items: items, NextCursor: next}, nil
}

// LastReplyRankKey returns the largest rank_key in the group (or "").
func (r *CommentRepository) LastReplyRankKey(ctx context.Context, groupID uuid.UUID) (string, error) {
	const query = `
        SELECT COALESCE(MAX(rank_key), '')
        FROM comments
        WHERE group_id = $1 AND depth = 1`
	var rk string
	if err := r.pool.QueryRow(ctx, query, groupID).Scan(&rk); err != nil {
		return "", fmt.Errorf("last reply rank: %w", err)
	}
	return rk, nil
}

// CountReplies returns reply counts per group, skipping deleted rows.
func (r *CommentRepository) CountReplies(ctx context.Context, groupIDs []uuid.UUID) (map[uuid.UUID]int, error) {
	if len(groupIDs) == 0 {
		return map[uuid.UUID]int{}, nil
	}
	const query = `
        SELECT group_id, COUNT(*)
        FROM comments
        WHERE group_id = ANY($1) AND depth = 1 AND deleted_at IS NULL
        GROUP BY group_id`
	rows, err := r.pool.Query(ctx, query, groupIDs)
	if err != nil {
		return nil, fmt.Errorf("count replies: %w", err)
	}
	defer rows.Close()

	out := make(map[uuid.UUID]int, len(groupIDs))
	for rows.Next() {
		var g uuid.UUID
		var n int
		if err := rows.Scan(&g, &n); err != nil {
			return nil, err
		}
		out[g] = n
	}
	return out, rows.Err()
}

// UpdateContent overwrites a comment's body and stamps edited_at.
func (r *CommentRepository) UpdateContent(ctx context.Context, id uuid.UUID, newContent string) error {
	const query = `
        UPDATE comments
        SET content = $2, edited_at = NOW()
        WHERE id = $1 AND deleted_at IS NULL`
	tag, err := r.pool.Exec(ctx, query, id, newContent)
	if err != nil {
		return fmt.Errorf("update content: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return comment.ErrCommentNotFound
	}
	return nil
}

// SoftDelete stamps deleted_at on the row. Content is retained because the
// DB CHECK constraint forbids empty strings; the handler is responsible
// for masking content in responses when is_deleted is true.
func (r *CommentRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	const query = `
        UPDATE comments
        SET deleted_at = NOW()
        WHERE id = $1 AND deleted_at IS NULL`
	tag, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("soft delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return comment.ErrCommentNotFound
	}
	return nil
}

// ApplyCounters writes the cached counts back to the row.
func (r *CommentRepository) ApplyCounters(ctx context.Context, id uuid.UUID, likes, dislikes int) error {
	if likes < 0 {
		likes = 0
	}
	if dislikes < 0 {
		dislikes = 0
	}
	const query = `
        UPDATE comments
        SET likes_count = $2, dislikes_count = $3
        WHERE id = $1`
	if _, err := r.pool.Exec(ctx, query, id, likes, dislikes); err != nil {
		return fmt.Errorf("apply counters: %w", err)
	}
	return nil
}

// UpsertReactions reconciles the comment_reactions rows for one comment
// with the supplied set in a single transaction.
func (r *CommentRepository) UpsertReactions(ctx context.Context, commentID uuid.UUID, rows []comment.ReactionRow) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin upsert reactions: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if len(rows) == 0 {
		if _, err := tx.Exec(ctx, `DELETE FROM comment_reactions WHERE comment_id = $1`, commentID); err != nil {
			return fmt.Errorf("delete reactions: %w", err)
		}
		return tx.Commit(ctx)
	}

	userIDs := make([]uuid.UUID, 0, len(rows))
	for _, row := range rows {
		userIDs = append(userIDs, row.UserID)
	}
	if _, err := tx.Exec(ctx,
		`DELETE FROM comment_reactions WHERE comment_id = $1 AND user_id <> ALL($2)`,
		commentID, userIDs,
	); err != nil {
		return fmt.Errorf("trim reactions: %w", err)
	}

	const upsertQuery = `
        INSERT INTO comment_reactions (comment_id, user_id, reaction)
        VALUES ($1, $2, $3)
        ON CONFLICT (comment_id, user_id) DO UPDATE
        SET reaction = EXCLUDED.reaction, updated_at = NOW()`
	for _, row := range rows {
		if _, err := tx.Exec(ctx, upsertQuery, commentID, row.UserID, int(row.Reaction)); err != nil {
			return fmt.Errorf("upsert reaction: %w", err)
		}
	}

	return tx.Commit(ctx)
}
