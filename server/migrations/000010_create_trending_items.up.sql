-- trending_items stores raw trending data collected from structured sources
-- (Google Trends RSS, News RSS, etc.) before AI processing.
CREATE TABLE trending_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    collection_run_id UUID NOT NULL REFERENCES collection_runs(id),
    keyword TEXT NOT NULL,
    source VARCHAR(50) NOT NULL,       -- "google_trends", "news_yonhap", etc.
    traffic INT NOT NULL DEFAULT 0,    -- raw search volume from source
    category VARCHAR(100),             -- topic category if available
    article_urls JSONB,                -- related article URLs (verified, from source)
    article_titles JSONB,              -- parallel to article_urls
    published_at TIMESTAMPTZ,          -- when the trend/article was published
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_trending_items_run_id ON trending_items(collection_run_id);
CREATE INDEX idx_trending_items_source ON trending_items(source);
CREATE INDEX idx_trending_items_keyword ON trending_items(keyword);
CREATE INDEX idx_trending_items_created ON trending_items(created_at DESC);
