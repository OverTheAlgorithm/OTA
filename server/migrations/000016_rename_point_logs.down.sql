-- Revert coin_logs → point_logs
ALTER TABLE coin_logs RENAME CONSTRAINT coin_logs_user_run_item_key TO point_logs_user_run_item_key;
ALTER TABLE coin_logs RENAME CONSTRAINT coin_logs_pkey TO point_logs_pkey;

ALTER INDEX idx_coin_logs_user_created RENAME TO idx_point_logs_user_created;
ALTER INDEX idx_coin_logs_user_id RENAME TO idx_point_logs_user_id;

ALTER TABLE coin_logs RENAME COLUMN coins_earned TO points_earned;
ALTER TABLE coin_logs RENAME TO point_logs;
