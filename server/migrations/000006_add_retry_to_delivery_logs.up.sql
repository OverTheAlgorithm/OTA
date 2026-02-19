-- Add retry tracking to delivery_logs
ALTER TABLE delivery_logs ADD COLUMN retry_count INTEGER NOT NULL DEFAULT 0;

-- Drop old unique constraint (one log per run/user/channel)
ALTER TABLE delivery_logs DROP CONSTRAINT delivery_logs_run_id_user_id_channel_key;

-- New unique constraint: one log per run/user/channel/attempt
ALTER TABLE delivery_logs ADD CONSTRAINT delivery_logs_run_user_channel_retry_unique
    UNIQUE(run_id, user_id, channel, retry_count);

-- Index for finding failed deliveries eligible for retry
CREATE INDEX idx_delivery_logs_failed_retry
    ON delivery_logs(run_id, status, retry_count)
    WHERE status = 'failed';
