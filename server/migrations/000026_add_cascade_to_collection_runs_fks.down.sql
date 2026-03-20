-- Revert ON DELETE CASCADE back to default (NO ACTION) for collection_runs FKs.

ALTER TABLE context_items
  DROP CONSTRAINT IF EXISTS context_items_collection_run_id_fkey,
  ADD CONSTRAINT context_items_collection_run_id_fkey
    FOREIGN KEY (collection_run_id) REFERENCES collection_runs(id);

ALTER TABLE trending_items
  DROP CONSTRAINT IF EXISTS trending_items_collection_run_id_fkey,
  ADD CONSTRAINT trending_items_collection_run_id_fkey
    FOREIGN KEY (collection_run_id) REFERENCES collection_runs(id);
