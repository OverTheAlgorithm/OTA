-- Replace user_preferences with user_delivery_channels for per-channel control

-- Drop old table
DROP TABLE IF EXISTS user_preferences;

-- Create new table with per-channel granularity
CREATE TABLE user_delivery_channels (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    channel VARCHAR(20) NOT NULL, -- 'email', 'kakao', 'telegram', etc.
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT unique_user_channel UNIQUE(user_id, channel)
);

-- Indexes for efficient queries
CREATE INDEX idx_user_delivery_channels_user_id ON user_delivery_channels(user_id);
CREATE INDEX idx_user_delivery_channels_enabled ON user_delivery_channels(user_id, channel) WHERE enabled = true;

-- Add check constraint for valid channel types
ALTER TABLE user_delivery_channels
    ADD CONSTRAINT check_valid_channel
    CHECK (channel IN ('email', 'kakao', 'telegram', 'sms', 'push'));

COMMENT ON TABLE user_delivery_channels IS 'Per-channel delivery preferences for each user';
COMMENT ON COLUMN user_delivery_channels.channel IS 'Delivery channel type: email, kakao, telegram, sms, push';
COMMENT ON COLUMN user_delivery_channels.enabled IS 'Whether this channel is enabled for the user';
