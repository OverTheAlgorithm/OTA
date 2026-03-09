-- General coin events for non-topic balance changes (signup bonus, promotions, admin adjustments, etc.)
CREATE TABLE coin_events (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount     INT NOT NULL,
    type       VARCHAR(50) NOT NULL,
    memo       VARCHAR(200),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_coin_events_user_id ON coin_events(user_id);
CREATE INDEX idx_coin_events_user_created ON coin_events(user_id, created_at DESC);
