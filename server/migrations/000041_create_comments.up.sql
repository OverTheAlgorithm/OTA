-- Comments for topics (context_items) and editor picks (editor_posts).
-- Two-level threading: depth 0 (root) and depth 1 (reply). Attempts to reply
-- beneath a depth-1 comment are recorded as siblings at depth 1.
--
-- Ordering uses lexorank strings (rank_key) so mid-list insertions do not
-- require rewriting other rows. Root sorts are served from indexed columns
-- (likes_count, created_at); reply sorts are served from (group_id, rank_key).
--
-- Like/dislike counts are write-through caches kept up to date by the
-- comment_flusher scheduler. The authoritative live state lives in Redis;
-- DB columns provide durability and queryable ordering.

CREATE TABLE comments (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    target_type    TEXT NOT NULL CHECK (target_type IN ('topic', 'editor_pick')),
    target_id      UUID NOT NULL,
    user_id        UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    group_id       UUID NOT NULL,
    parent_id      UUID REFERENCES comments(id) ON DELETE SET NULL,
    depth          SMALLINT NOT NULL CHECK (depth IN (0, 1)),
    rank_key       TEXT NOT NULL,
    content        TEXT NOT NULL CHECK (char_length(content) BETWEEN 1 AND 2000),
    likes_count    INTEGER NOT NULL DEFAULT 0 CHECK (likes_count >= 0),
    dislikes_count INTEGER NOT NULL DEFAULT 0 CHECK (dislikes_count >= 0),
    edited_at      TIMESTAMPTZ,
    deleted_at     TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Root listing: popular ordering plus tiebreakers.
CREATE INDEX idx_comments_roots_popular
    ON comments (target_type, target_id, likes_count DESC, created_at DESC, id DESC)
    WHERE depth = 0;

-- Root listing: recent ordering tiebreak by id for keyset cursors.
CREATE INDEX idx_comments_roots_recent
    ON comments (target_type, target_id, created_at DESC, id DESC)
    WHERE depth = 0;

-- Reply listing: lexorank-ordered within a thread.
CREATE INDEX idx_comments_replies_rank
    ON comments (group_id, rank_key)
    WHERE depth = 1;

-- Reply count aggregation per group.
CREATE INDEX idx_comments_group_depth
    ON comments (group_id, depth);

-- Author lookup for "my comments" pages, ordered newest first.
CREATE INDEX idx_comments_user_created
    ON comments (user_id, created_at DESC);

CREATE TABLE comment_reactions (
    comment_id UUID NOT NULL REFERENCES comments(id) ON DELETE CASCADE,
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    reaction   SMALLINT NOT NULL CHECK (reaction IN (-1, 1)),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (comment_id, user_id)
);

CREATE INDEX idx_comment_reactions_user ON comment_reactions (user_id);
