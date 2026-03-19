-- categories table (7 categories: general + 6 domain)
CREATE TABLE IF NOT EXISTS categories (
    key VARCHAR(50) PRIMARY KEY,
    label TEXT NOT NULL,
    display_order INT NOT NULL DEFAULT 0
);
INSERT INTO categories (key, label, display_order) VALUES
    ('general', '종합', 0),
    ('entertainment', '연예', 1),
    ('business', '경제', 2),
    ('sports', '스포츠', 3),
    ('technology', '기술', 4),
    ('science', '과학', 5),
    ('health', '건강', 6)
ON CONFLICT (key) DO NOTHING;

-- news_sources table (category-based RSS URLs)
CREATE TABLE IF NOT EXISTS news_sources (
    id SERIAL PRIMARY KEY,
    category_key VARCHAR(50) NOT NULL REFERENCES categories(key),
    provider VARCHAR(50) NOT NULL DEFAULT 'google_news',
    url TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT true
);

INSERT INTO news_sources (category_key, provider, url) VALUES
    ('general', 'google_news', 'https://news.google.com/rss?hl=ko&gl=KR&ceid=KR:ko'),
    ('entertainment', 'google_news', 'https://news.google.com/rss/topics/CAAqJggKIiBDQkFTRWdvSUwyMHZNREpxYW5RU0FtdHZHZ0pMVWlnQVAB?hl=ko&gl=KR&ceid=KR:ko'),
    ('business', 'google_news', 'https://news.google.com/rss/topics/CAAqJggKIiBDQkFTRWdvSUwyMHZNRGx6TVdZU0FtdHZHZ0pMVWlnQVAB?hl=ko&gl=KR&ceid=KR:ko'),
    ('sports', 'google_news', 'https://news.google.com/rss/topics/CAAqJggKIiBDQkFTRWdvSUwyMHZNRFp1ZEdvU0FtdHZHZ0pMVWlnQVAB?hl=ko&gl=KR&ceid=KR:ko'),
    ('technology', 'google_news', 'https://news.google.com/rss/topics/CAAqJggKIiBDQkFTRWdvSUwyMHZNRGRqTVhZU0FtdHZHZ0pMVWlnQVAB?hl=ko&gl=KR&ceid=KR:ko'),
    ('science', 'google_news', 'https://news.google.com/rss/topics/CAAqJggKIiBDQkFTRWdvSUwyMHZNRFp0Y1RjU0FtdHZHZ0pMVWlnQVAB?hl=ko&gl=KR&ceid=KR:ko'),
    ('health', 'google_news', 'https://news.google.com/rss/topics/CAAqIQgKIhtDQkFTRGdvSUwyMHZNR3QwTlRFU0FtdHZLQUFQAQ?hl=ko&gl=KR&ceid=KR:ko');

-- Add priority column to context_items
ALTER TABLE context_items ADD COLUMN IF NOT EXISTS priority VARCHAR(10) NOT NULL DEFAULT 'none';

-- Migrate existing data: top/brief → priority, then set category to general
UPDATE context_items SET priority = category WHERE category IN ('top', 'brief');
UPDATE context_items SET category = 'general' WHERE category IN ('top', 'brief');
