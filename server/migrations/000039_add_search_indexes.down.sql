DROP INDEX IF EXISTS idx_context_items_detail_trgm;
DROP INDEX IF EXISTS idx_context_items_summary_trgm;
DROP INDEX IF EXISTS idx_context_items_topic_trgm;
-- pg_trgm extension is left in place; other code may rely on it.
