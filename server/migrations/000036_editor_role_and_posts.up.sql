-- Backfill any pre-existing NULL or empty roles so the CHECK constraint can be applied safely.
UPDATE users SET role = 'user' WHERE role IS NULL OR role = '';

-- Tighten the role column. The existing column is already TEXT NOT NULL DEFAULT 'user'
-- in early migrations, but we re-assert defaults and add the CHECK constraint for safety.
ALTER TABLE users
    ALTER COLUMN role SET DEFAULT 'user',
    ALTER COLUMN role SET NOT NULL;

ALTER TABLE users
    ADD CONSTRAINT users_role_check CHECK (role IN ('user', 'editor', 'admin'));

-- Audit log for every role change performed through the admin UI.
CREATE TABLE role_change_logs (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    before_role TEXT NOT NULL,
    after_role  TEXT NOT NULL,
    actor_id    UUID REFERENCES users(id) ON DELETE SET NULL,
    memo        TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_role_change_logs_user_created ON role_change_logs(user_id, created_at DESC);

-- Rich-text posts authored by editors (or admins).
CREATE TABLE editor_posts (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    author_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title           TEXT NOT NULL,
    content_html    TEXT NOT NULL,
    content_text    TEXT NOT NULL DEFAULT '',
    first_image_url TEXT,
    status          TEXT NOT NULL DEFAULT 'draft'
                    CHECK (status IN ('draft', 'published')),
    published_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_editor_posts_published_at
    ON editor_posts (published_at DESC)
    WHERE status = 'published';

CREATE INDEX idx_editor_posts_author ON editor_posts (author_id, created_at DESC);
