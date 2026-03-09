DROP INDEX IF EXISTS idx_coin_events_actor_id;
ALTER TABLE coin_events DROP COLUMN IF EXISTS actor_id;
