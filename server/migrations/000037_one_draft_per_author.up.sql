-- Enforce at most one draft per author. Keep the most recently updated draft
-- when duplicates exist so we don't lose the freshest work-in-progress.
WITH ranked AS (
    SELECT id,
           ROW_NUMBER() OVER (PARTITION BY author_id ORDER BY updated_at DESC, created_at DESC) AS rn
    FROM editor_posts
    WHERE status = 'draft'
)
DELETE FROM editor_posts
WHERE id IN (SELECT id FROM ranked WHERE rn > 1);

CREATE UNIQUE INDEX uniq_editor_posts_one_draft_per_author
    ON editor_posts (author_id)
    WHERE status = 'draft';
