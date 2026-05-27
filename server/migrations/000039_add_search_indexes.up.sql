-- Enable pg_trgm extension for fast ILIKE matching on context_items (search feature).
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- GIN trigram indexes accelerate `WHERE topic ILIKE '%q%'` style queries
-- across Korean and English text. Separate indexes per column so the planner
-- can pick the cheapest one when the query targets a specific field.
CREATE INDEX IF NOT EXISTS idx_context_items_topic_trgm
    ON context_items USING gin (topic gin_trgm_ops);

CREATE INDEX IF NOT EXISTS idx_context_items_summary_trgm
    ON context_items USING gin (summary gin_trgm_ops);

CREATE INDEX IF NOT EXISTS idx_context_items_detail_trgm
    ON context_items USING gin (detail gin_trgm_ops);
