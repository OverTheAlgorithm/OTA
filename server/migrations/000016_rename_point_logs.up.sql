-- Rename point_logs → coin_logs to align with coin terminology
ALTER TABLE point_logs RENAME TO coin_logs;
ALTER TABLE coin_logs RENAME COLUMN points_earned TO coins_earned;

-- Rename indexes
ALTER INDEX idx_point_logs_user_id RENAME TO idx_coin_logs_user_id;
ALTER INDEX idx_point_logs_user_created RENAME TO idx_coin_logs_user_created;

-- Rename constraints
ALTER TABLE coin_logs RENAME CONSTRAINT point_logs_pkey TO coin_logs_pkey;
ALTER TABLE coin_logs RENAME CONSTRAINT point_logs_user_run_item_key TO coin_logs_user_run_item_key;
