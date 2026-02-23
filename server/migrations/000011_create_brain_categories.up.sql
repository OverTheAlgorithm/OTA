CREATE TABLE brain_categories (
    key VARCHAR(50) PRIMARY KEY,
    emoji VARCHAR(10) NOT NULL,
    label TEXT NOT NULL,
    accent_color VARCHAR(20) NOT NULL DEFAULT '#9b8bb4',
    display_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO brain_categories (key, emoji, label, accent_color, display_order) VALUES
('must_know',     '🔥', '모르면 나만 모르는 이야기예요',       '#e84d3d', 1),
('plan_ahead',    '📅', '기억해두고 일정을 조정하세요',       '#5ba4d9', 2),
('conversation',  '💬', '대화할 때 꺼내보세요',               '#9b8bb4', 3),
('opinion',       '⚖️', '의견이 갈리는 주제예요, 조심!',      '#f0a500', 4),
('result',        '🏆', '결과만 알면 충분해요',               '#7bc67e', 5),
('trend',         '📈', '요즘 이런 흐름이에요',               '#5ba4d9', 6),
('useful',        '💡', '생활에 도움이 돼요',                 '#7bc67e', 7),
('fun',           '😄', '가볍게 웃고 넘어가세요',             '#9b8bb4', 8);

ALTER TABLE context_items ADD COLUMN brain_category VARCHAR(50)
    REFERENCES brain_categories(key) ON UPDATE CASCADE ON DELETE SET NULL;
