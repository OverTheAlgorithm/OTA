-- user_preferences: tracks who receives messages and their delivery settings
CREATE TABLE user_preferences (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    delivery_enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- user_subscriptions: tracks which topics each user wants to receive
CREATE TABLE user_subscriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    category VARCHAR(50) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, category)
);

CREATE INDEX idx_user_subscriptions_user_id ON user_subscriptions(user_id);

-- delivery_logs: prevents duplicate sends and provides audit trail
CREATE TABLE delivery_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id UUID NOT NULL REFERENCES collection_runs(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    channel VARCHAR(20) NOT NULL, -- 'email' or 'kakao'
    status VARCHAR(20) NOT NULL,  -- 'sent', 'failed', 'skipped'
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(run_id, user_id, channel) -- idempotency: one delivery per run/user/channel
);

CREATE INDEX idx_delivery_logs_run_id ON delivery_logs(run_id);
CREATE INDEX idx_delivery_logs_user_id ON delivery_logs(user_id);
CREATE INDEX idx_delivery_logs_created_at ON delivery_logs(created_at);
