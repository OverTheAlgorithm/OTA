-- 커뮤니티 트렌드 피쳐. 기존 collector와 분리된 신규 도메인.
-- 원문(본문/제목) 미저장 — 파생 집계만. 상세: docs/community-trend/technical-spec.md

-- 태그 분류 축 (성향/연령/논제 등)
CREATE TABLE ct_axes (
    id            SERIAL PRIMARY KEY,
    key           TEXT NOT NULL UNIQUE,
    label         TEXT NOT NULL,
    display_order INT  NOT NULL DEFAULT 0
);

-- 공통 태그 풀. 메타(성향) / 일일(주제) 어느 쪽으로도 부착 가능 (decisions.md D-003).
CREATE TABLE ct_tags (
    id          SERIAL PRIMARY KEY,
    axis_id     INT  NOT NULL REFERENCES ct_axes(id),
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_by  TEXT NOT NULL DEFAULT 'admin' CHECK (created_by IN ('seed','ai','admin')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (axis_id, name)
);

-- 커뮤니티. key = 어댑터 연동 자연키 (decisions.md D-004).
CREATE TABLE ct_communities (
    id         SERIAL PRIMARY KEY,
    key        TEXT NOT NULL UNIQUE,
    name       TEXT NOT NULL,
    home_url   TEXT NOT NULL DEFAULT '',
    enabled    BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 메타 부착(영구 성향) = 코호트 차원 (decisions.md D-010).
CREATE TABLE ct_community_tags (
    community_id INT NOT NULL REFERENCES ct_communities(id) ON DELETE CASCADE,
    tag_id       INT NOT NULL REFERENCES ct_tags(id) ON DELETE RESTRICT,
    PRIMARY KEY (community_id, tag_id)
);

-- 일일 주제 태그 카운트 (자동+수동 합류). 신규유입형 (decisions.md D-001).
CREATE TABLE ct_tag_daily (
    community_id INT  NOT NULL REFERENCES ct_communities(id) ON DELETE CASCADE,
    tag_id       INT  NOT NULL REFERENCES ct_tags(id) ON DELETE RESTRICT,
    stat_date    DATE NOT NULL,
    post_count   INT  NOT NULL DEFAULT 0 CHECK (post_count >= 0),
    source       TEXT NOT NULL DEFAULT 'human' CHECK (source IN ('ai','human','hybrid')),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (community_id, tag_id, stat_date)
);

CREATE TABLE ct_community_daily (
    community_id INT  NOT NULL REFERENCES ct_communities(id) ON DELETE CASCADE,
    stat_date    DATE NOT NULL,
    total_posts  INT  NOT NULL DEFAULT 0 CHECK (total_posts >= 0),
    PRIMARY KEY (community_id, stat_date)
);

-- 커뮤니티 × 날짜 작업 상태. 글/제목 미저장.
CREATE TABLE ct_worksheets (
    id           SERIAL PRIMARY KEY,
    community_id INT  NOT NULL REFERENCES ct_communities(id) ON DELETE CASCADE,
    stat_date    DATE NOT NULL,
    mode         TEXT NOT NULL CHECK (mode IN ('auto','manual')),
    status       TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','suggested','confirmed')),
    total_posts  INT,
    confirmed_by UUID REFERENCES users(id),
    confirmed_at TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (community_id, stat_date)
);

-- 합법성 추적
CREATE TABLE ct_robots_status (
    community_id  INT  NOT NULL REFERENCES ct_communities(id) ON DELETE CASCADE,
    checked_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    allowed       BOOLEAN NOT NULL,
    snapshot_hash TEXT NOT NULL DEFAULT '',
    note          TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (community_id, checked_at)
);

CREATE TABLE ct_robots_transitions (
    id           SERIAL PRIMARY KEY,
    community_id INT NOT NULL REFERENCES ct_communities(id) ON DELETE CASCADE,
    from_allowed BOOLEAN,
    to_allowed   BOOLEAN NOT NULL,
    changed_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 중복검정 (역산 불가 해시 지문)
CREATE TABLE ct_seen_posts (
    community_id INT  NOT NULL REFERENCES ct_communities(id) ON DELETE CASCADE,
    fingerprint  TEXT NOT NULL,
    first_seen   DATE NOT NULL,
    PRIMARY KEY (community_id, fingerprint)
);

-- 밈 트랙 (태그와 독립)
CREATE TABLE ct_memes (
    id          SERIAL PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,
    aliases     TEXT[] NOT NULL DEFAULT '{}',
    status      TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','retired')),
    created_via TEXT NOT NULL CHECK (created_via IN ('promote','manual')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE ct_meme_candidates (
    id          SERIAL PRIMARY KEY,
    expression  TEXT NOT NULL UNIQUE,
    hit_count   INT  NOT NULL DEFAULT 1 CHECK (hit_count >= 1),
    first_seen  DATE NOT NULL,
    last_seen   DATE NOT NULL
);

CREATE TABLE ct_meme_blacklist (
    expression TEXT PRIMARY KEY
);

CREATE TABLE ct_meme_daily (
    meme_id      INT  NOT NULL REFERENCES ct_memes(id) ON DELETE CASCADE,
    community_id INT  NOT NULL REFERENCES ct_communities(id) ON DELETE CASCADE,
    stat_date    DATE NOT NULL,
    count        INT  NOT NULL DEFAULT 0 CHECK (count >= 0),
    PRIMARY KEY (meme_id, community_id, stat_date)
);

-- 조회 인덱스
CREATE INDEX idx_ct_tag_daily_date ON ct_tag_daily (stat_date, community_id);
CREATE INDEX idx_ct_tag_daily_tag  ON ct_tag_daily (tag_id, stat_date);
CREATE INDEX idx_ct_worksheets_date ON ct_worksheets (stat_date, status);
CREATE INDEX idx_ct_meme_daily_date ON ct_meme_daily (stat_date, community_id);

-- ── 시드 ─────────────────────────────────────────────
INSERT INTO ct_axes (key, label, display_order) VALUES
    ('leaning',        '성향축',     1),
    ('political',      '정치성향축', 2),
    ('age',            '연령축',     3),
    ('gender_topic',   '젠더논제축', 4),
    ('political_topic','정치논제축', 5);

-- 메타 태그 (성향/연령). created_by='seed'.
INSERT INTO ct_tags (axis_id, name, created_by) VALUES
    ((SELECT id FROM ct_axes WHERE key='leaning'),   '남성향',    'seed'),
    ((SELECT id FROM ct_axes WHERE key='leaning'),   '여성향',    'seed'),
    ((SELECT id FROM ct_axes WHERE key='political'), '진보 성향', 'seed'),
    ((SELECT id FROM ct_axes WHERE key='political'), '보수 성향', 'seed'),
    ((SELECT id FROM ct_axes WHERE key='age'),       '2030',      'seed'),
    ((SELECT id FROM ct_axes WHERE key='age'),       '4050',      'seed');

-- 첫 4개 커뮤니티 (decisions.md D-004)
INSERT INTO ct_communities (key, name, home_url) VALUES
    ('dogdrip', '개드립',     'https://www.dogdrip.net'),
    ('clien',   '클리앙',     'https://www.clien.net'),
    ('fmkorea', '에펨코리아', 'https://www.fmkorea.com'),
    ('theqoo',  '더쿠',       'https://theqoo.net');

-- 메타태그 부착 (관리자가 추후 조정 가능, 시작값)
INSERT INTO ct_community_tags (community_id, tag_id)
SELECT c.id, t.id FROM ct_communities c, ct_tags t
WHERE (c.key='dogdrip' AND t.name='남성향')
   OR (c.key='clien'   AND t.name IN ('남성향','진보 성향'))
   OR (c.key='fmkorea' AND t.name IN ('남성향','보수 성향'))
   OR (c.key='theqoo'  AND t.name='여성향');
