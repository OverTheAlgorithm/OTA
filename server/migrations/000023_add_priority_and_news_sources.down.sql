-- Restore top/brief in category from priority
UPDATE context_items SET category = priority WHERE priority IN ('top', 'brief');

-- Drop priority column
ALTER TABLE context_items DROP COLUMN IF EXISTS priority;

-- Drop news_sources and categories tables
DROP TABLE IF EXISTS news_sources;
DROP TABLE IF EXISTS categories;
