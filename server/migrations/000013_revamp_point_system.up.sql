-- Add run_id column to point_logs.
-- Existing rows receive gen_random_uuid() so the new UNIQUE constraint stays valid.
ALTER TABLE point_logs ADD COLUMN run_id UUID NOT NULL DEFAULT gen_random_uuid();

-- Drop the old unique constraint (one-time-per-item-ever)
ALTER TABLE point_logs DROP CONSTRAINT point_logs_user_id_context_item_id_key;

-- New constraint: one earn per (user, run, item) — same topic in tomorrow's run is a fresh opportunity
ALTER TABLE point_logs ADD CONSTRAINT point_logs_user_run_item_key
    UNIQUE (user_id, run_id, context_item_id);

-- Index for GetLastEarnedAt / GetLastEarnedAtBatch queries
CREATE INDEX idx_point_logs_user_created ON point_logs (user_id, created_at DESC);

-- Update level thresholds comment (actual values live in Go code)
-- New: Lv1:0, Lv2:15, Lv3:45, Lv4:90, Lv5:165
