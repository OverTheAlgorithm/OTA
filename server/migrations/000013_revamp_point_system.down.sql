DROP INDEX IF EXISTS idx_point_logs_user_created;

ALTER TABLE point_logs DROP CONSTRAINT IF EXISTS point_logs_user_run_item_key;
ALTER TABLE point_logs ADD CONSTRAINT point_logs_user_id_context_item_id_key UNIQUE (user_id, context_item_id);

ALTER TABLE point_logs DROP COLUMN IF EXISTS run_id;
