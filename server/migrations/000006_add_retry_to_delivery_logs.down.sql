DROP INDEX IF EXISTS idx_delivery_logs_failed_retry;
ALTER TABLE delivery_logs DROP CONSTRAINT IF EXISTS delivery_logs_run_user_channel_retry_unique;
ALTER TABLE delivery_logs ADD CONSTRAINT delivery_logs_run_id_user_id_channel_key UNIQUE(run_id, user_id, channel);
ALTER TABLE delivery_logs DROP COLUMN IF EXISTS retry_count;
