-- Trigram indexes accelerate substring search on editor posts so the public
-- search box can include editor picks alongside news topics. Mirrors
-- migration 000039 for context_items. pg_trgm is already enabled there.

CREATE INDEX IF NOT EXISTS idx_editor_posts_title_trgm
    ON editor_posts USING gin (title gin_trgm_ops);

CREATE INDEX IF NOT EXISTS idx_editor_posts_content_text_trgm
    ON editor_posts USING gin (content_text gin_trgm_ops);
