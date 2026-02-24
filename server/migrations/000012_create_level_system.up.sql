CREATE TABLE user_points (
    user_id    UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    level      INT NOT NULL DEFAULT 1,
    points     INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE point_logs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    context_item_id UUID NOT NULL,
    points_earned   INT NOT NULL DEFAULT 1,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, context_item_id)
);

CREATE INDEX idx_point_logs_user_id ON point_logs(user_id);

INSERT INTO brain_categories (key, emoji, label, accent_color, display_order)
VALUES ('over_the_algorithm', '🌈', 'Over the Algorithm', '#7bc67e', 9);
