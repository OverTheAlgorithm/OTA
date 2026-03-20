-- Add ON DELETE CASCADE to foreign keys referencing collection_runs
-- so that data retention cleanup can delete old runs without FK violations.

-- context_items.collection_run_id
ALTER TABLE context_items
  DROP CONSTRAINT IF EXISTS context_items_collection_run_id_fkey,
  ADD CONSTRAINT context_items_collection_run_id_fkey
    FOREIGN KEY (collection_run_id) REFERENCES collection_runs(id) ON DELETE CASCADE;

-- trending_items.collection_run_id
ALTER TABLE trending_items
  DROP CONSTRAINT IF EXISTS trending_items_collection_run_id_fkey,
  ADD CONSTRAINT trending_items_collection_run_id_fkey
    FOREIGN KEY (collection_run_id) REFERENCES collection_runs(id) ON DELETE CASCADE;
