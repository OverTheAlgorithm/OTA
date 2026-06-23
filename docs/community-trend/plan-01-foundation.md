# 커뮤니티 트렌드 — 구현 계획 #01: 파운데이션 (데이터 계층 + 커뮤니티/태그 CRUD)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 신규 `communitytrend` 도메인의 DB 스키마와 커뮤니티·축·태그·메타태그를 관리자가 CRUD 할 수 있는 백엔드 데이터 계층을 만든다.

**Architecture:** 기존 `collector`와 비즈니스 로직 0 공유. 도메인(`server/domain/communitytrend`)에 모델·저장소 인터페이스·서비스, 구현은 `server/storage`(pgxpool), HTTP는 `server/api/handler`. 라우터는 기존 `RouteModule` 패턴으로 등록. 코호트는 별도 테이블 없이 메타태그(`ct_community_tags`)로 표현.

**Tech Stack:** Go 1.25, Gin, pgx/v5, golang-migrate, testcontainers(postgres:16-alpine).

## Global Constraints

- 모듈명 `ota`. 신규 패키지 `ota/domain/communitytrend`, 구현 `ota/storage`.
- 모든 신규 테이블 `ct_` 접두. 기존 테이블과 FK 없음 (단 actor는 `users(id)` 참조 허용 — 이 플랜에선 미사용).
- **본문/제목 등 원문 절대 미저장** (이 플랜은 메타데이터만 다루므로 자동 충족).
- API 응답 엔벨로프: 성공 `gin.H{"data": ...}` 또는 `gin.H{"message":"ok"}`, 실패 `gin.H{"error": "<한국어 메시지>"}`. (기존 핸들러 관례)
- 에러 래핑: `fmt.Errorf("context: %w", err)`.
- 커뮤니티 `key`는 어댑터 연동 자연키 — 소문자 영숫자+`-`/`_`만, 변경 불가(생성 후 PATCH 대상 아님).
- 테스트: 통합은 `server/integration/`에서 `SetupTestDB(t)` + `db.Truncate(t, ...)`. 단위는 도메인 패키지 내 `_test.go`.
- gofmt/goimports 필수.

---

## File Structure

| 파일 | 책임 |
|------|------|
| `server/migrations/000043_create_community_trend.up.sql` / `.down.sql` | 전 `ct_` 테이블 + 시드 |
| `server/domain/communitytrend/model.go` | `Axis`, `Tag`, `Community` 구조체 |
| `server/domain/communitytrend/repository.go` | `AxisRepository`, `TagRepository`, `CommunityRepository` 인터페이스 |
| `server/domain/communitytrend/service.go` | 검증 + 오케스트레이션 (`Service`) |
| `server/domain/communitytrend/service_test.go` | 서비스 단위 테스트 (fake repo) |
| `server/storage/ct_axis_repo.go` | `AxisRepository` pgx 구현 |
| `server/storage/ct_tag_repo.go` | `TagRepository` pgx 구현 |
| `server/storage/ct_community_repo.go` | `CommunityRepository` pgx 구현 (메타태그 부착 포함) |
| `server/api/handler/community_trend_handler.go` | 관리자 핸들러 + 라우트 |
| `server/integration/community_trend_test.go` | 마이그레이션 + repo + HTTP 통합 테스트 |
| `server/main.go` | 와이어링 (repos→service→handler→RouteModule) |

---

## Task 1: 마이그레이션 + 시드

**Files:**
- Create: `server/migrations/000043_create_community_trend.up.sql`
- Create: `server/migrations/000043_create_community_trend.down.sql`
- Test: `server/integration/community_trend_test.go`

**Interfaces:**
- Consumes: 없음 (기존 `users` 테이블 존재 가정).
- Produces: 테이블 `ct_axes, ct_tags, ct_communities, ct_community_tags, ct_tag_daily, ct_community_daily, ct_worksheets, ct_robots_status, ct_robots_transitions, ct_seen_posts, ct_memes, ct_meme_candidates, ct_meme_blacklist, ct_meme_daily`. 시드: 5 axes, 4 communities, 6 meta tags, 메타 부착.

- [ ] **Step 1: 마이그레이션 통합 테스트 작성 (실패 예정)**

`server/integration/community_trend_test.go` 생성:

```go
package integration

import (
	"context"
	"testing"
)

func TestCommunityTrend_Migration(t *testing.T) {
	db := SetupTestDB(t)
	ctx := context.Background()

	// 14개 ct_ 테이블이 모두 존재하는지
	wantTables := []string{
		"ct_axes", "ct_tags", "ct_communities", "ct_community_tags",
		"ct_tag_daily", "ct_community_daily", "ct_worksheets",
		"ct_robots_status", "ct_robots_transitions", "ct_seen_posts",
		"ct_memes", "ct_meme_candidates", "ct_meme_blacklist", "ct_meme_daily",
	}
	for _, tbl := range wantTables {
		var exists bool
		err := db.Pool.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = $1)`, tbl).Scan(&exists)
		if err != nil {
			t.Fatalf("check table %s: %v", tbl, err)
		}
		if !exists {
			t.Fatalf("expected table %s to exist", tbl)
		}
	}

	// 시드 검증
	var axisCount, commCount, tagCount, attachCount int
	db.Pool.QueryRow(ctx, `SELECT count(*) FROM ct_axes`).Scan(&axisCount)
	db.Pool.QueryRow(ctx, `SELECT count(*) FROM ct_communities`).Scan(&commCount)
	db.Pool.QueryRow(ctx, `SELECT count(*) FROM ct_tags`).Scan(&tagCount)
	db.Pool.QueryRow(ctx, `SELECT count(*) FROM ct_community_tags`).Scan(&attachCount)

	if axisCount != 5 {
		t.Fatalf("expected 5 seeded axes, got %d", axisCount)
	}
	if commCount != 4 {
		t.Fatalf("expected 4 seeded communities, got %d", commCount)
	}
	if tagCount != 6 {
		t.Fatalf("expected 6 seeded meta tags, got %d", tagCount)
	}
	if attachCount < 4 {
		t.Fatalf("expected at least 4 meta-tag attachments, got %d", attachCount)
	}
}
```

- [ ] **Step 2: 테스트 실패 확인**

Run: `cd server && go test ./integration/ -run TestCommunityTrend_Migration -v`
Expected: FAIL — 마이그레이션 파일 없어 테이블 미생성(`expected table ct_axes to exist`).

- [ ] **Step 3: up 마이그레이션 작성**

`server/migrations/000043_create_community_trend.up.sql`:

```sql
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
```

- [ ] **Step 4: down 마이그레이션 작성**

`server/migrations/000043_create_community_trend.down.sql`:

```sql
DROP TABLE IF EXISTS ct_meme_daily;
DROP TABLE IF EXISTS ct_meme_blacklist;
DROP TABLE IF EXISTS ct_meme_candidates;
DROP TABLE IF EXISTS ct_memes;
DROP TABLE IF EXISTS ct_seen_posts;
DROP TABLE IF EXISTS ct_robots_transitions;
DROP TABLE IF EXISTS ct_robots_status;
DROP TABLE IF EXISTS ct_worksheets;
DROP TABLE IF EXISTS ct_community_daily;
DROP TABLE IF EXISTS ct_tag_daily;
DROP TABLE IF EXISTS ct_community_tags;
DROP TABLE IF EXISTS ct_communities;
DROP TABLE IF EXISTS ct_tags;
DROP TABLE IF EXISTS ct_axes;
```

- [ ] **Step 5: 테스트 통과 확인**

Run: `cd server && go test ./integration/ -run TestCommunityTrend_Migration -v`
Expected: PASS.

- [ ] **Step 6: 커밋**

```bash
git add server/migrations/000043_create_community_trend.up.sql server/migrations/000043_create_community_trend.down.sql server/integration/community_trend_test.go
git commit -m "feat(community-trend): add schema migration and seed"
```

---

## Task 2: 도메인 모델 + 저장소 인터페이스

**Files:**
- Create: `server/domain/communitytrend/model.go`
- Create: `server/domain/communitytrend/repository.go`

**Interfaces:**
- Consumes: 없음.
- Produces: 구조체 `Axis{ID int; Key, Label string; DisplayOrder int}`, `Tag{ID, AxisID int; Name, Description, CreatedBy string; CreatedAt time.Time}`, `Community{ID int; Key, Name, HomeURL string; Enabled bool; CreatedAt, UpdatedAt time.Time; MetaTagIDs []int}`. 인터페이스 `AxisRepository`, `TagRepository`, `CommunityRepository` (시그니처는 Step 2 코드 참조).

- [ ] **Step 1: 모델 작성**

`server/domain/communitytrend/model.go`:

```go
// Package communitytrend collects and aggregates the trending "topics" of
// Korean online communities as derived tag counts. It shares no business
// logic with the news collector package.
package communitytrend

import "time"

// Axis groups tags into a dimension (e.g. 성향축, 정치논제축).
type Axis struct {
	ID           int    `json:"id"`
	Key          string `json:"key"`
	Label        string `json:"label"`
	DisplayOrder int    `json:"display_order"`
}

// Tag is a single classification label belonging to one axis.
// The same tag may be attached as community meta (성향) or daily topic (decisions.md D-003).
type Tag struct {
	ID          int       `json:"id"`
	AxisID      int       `json:"axis_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedBy   string    `json:"created_by"` // 'seed' | 'ai' | 'admin'
	CreatedAt   time.Time `json:"created_at"`
}

// Community is a tracked site. Key is the natural identifier linking to a
// code-side SourceAdapter (decisions.md D-004).
type Community struct {
	ID         int       `json:"id"`
	Key        string    `json:"key"`
	Name       string    `json:"name"`
	HomeURL    string    `json:"home_url"`
	Enabled    bool      `json:"enabled"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	MetaTagIDs []int     `json:"meta_tag_ids"` // populated on read; cohort dimension
}
```

- [ ] **Step 2: 저장소 인터페이스 작성**

`server/domain/communitytrend/repository.go`:

```go
package communitytrend

import "context"

// AxisRepository persists tag axes.
type AxisRepository interface {
	Create(ctx context.Context, a Axis) (Axis, error)
	List(ctx context.Context) ([]Axis, error)
}

// TagRepository persists the shared tag pool.
type TagRepository interface {
	Create(ctx context.Context, t Tag) (Tag, error)
	List(ctx context.Context) ([]Tag, error)
	ListByAxis(ctx context.Context, axisID int) ([]Tag, error)
	Update(ctx context.Context, id int, name, description string) (Tag, error)
	Delete(ctx context.Context, id int) error
}

// CommunityRepository persists communities and their meta-tag attachments.
type CommunityRepository interface {
	Create(ctx context.Context, c Community) (Community, error)
	List(ctx context.Context) ([]Community, error)
	Update(ctx context.Context, id int, name, homeURL string, enabled bool) (Community, error)
	Delete(ctx context.Context, id int) error
	// SetMetaTags replaces the full meta-tag set for a community atomically.
	SetMetaTags(ctx context.Context, communityID int, tagIDs []int) error
	GetMetaTags(ctx context.Context, communityID int) ([]int, error)
}
```

- [ ] **Step 3: 컴파일 확인**

Run: `cd server && go build ./domain/communitytrend/`
Expected: 성공 (출력 없음).

- [ ] **Step 4: 커밋**

```bash
git add server/domain/communitytrend/model.go server/domain/communitytrend/repository.go
git commit -m "feat(community-trend): add domain models and repository interfaces"
```

---

## Task 3: 커뮤니티 저장소 구현 (메타태그 부착 포함)

**Files:**
- Create: `server/storage/ct_community_repo.go`
- Test: `server/integration/community_trend_test.go` (함수 추가)

**Interfaces:**
- Consumes: `communitytrend.Community`, `communitytrend.CommunityRepository`.
- Produces: `storage.CTCommunityRepository` (생성자 `NewCTCommunityRepository(pool *pgxpool.Pool) *CTCommunityRepository`), `CommunityRepository` 구현.

- [ ] **Step 1: 통합 테스트 작성 (실패 예정)**

`server/integration/community_trend_test.go`에 추가:

```go
func TestCommunityTrend_CommunityRepo(t *testing.T) {
	db := SetupTestDB(t)
	ctx := context.Background()
	repo := storage.NewCTCommunityRepository(db.Pool)

	// 시드 4개 + 신규 1개 생성
	created, err := repo.Create(ctx, communitytrend.Community{
		Key: "ruliweb", Name: "루리웹", HomeURL: "https://bbs.ruliweb.com", Enabled: true,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.ID == 0 || created.Key != "ruliweb" {
		t.Fatalf("unexpected created community: %+v", created)
	}

	list, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 5 {
		t.Fatalf("expected 5 communities (4 seed + 1), got %d", len(list))
	}

	// 메타태그 부착: 시드 태그 '남성향','진보 성향' id 조회
	var maleID, progID int
	db.Pool.QueryRow(ctx, `SELECT id FROM ct_tags WHERE name='남성향'`).Scan(&maleID)
	db.Pool.QueryRow(ctx, `SELECT id FROM ct_tags WHERE name='진보 성향'`).Scan(&progID)

	if err := repo.SetMetaTags(ctx, created.ID, []int{maleID, progID}); err != nil {
		t.Fatalf("set meta tags: %v", err)
	}
	got, err := repo.GetMetaTags(ctx, created.ID)
	if err != nil {
		t.Fatalf("get meta tags: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 meta tags, got %d", len(got))
	}

	// SetMetaTags는 전체 교체 (1개로 줄임)
	if err := repo.SetMetaTags(ctx, created.ID, []int{maleID}); err != nil {
		t.Fatalf("reset meta tags: %v", err)
	}
	got2, _ := repo.GetMetaTags(ctx, created.ID)
	if len(got2) != 1 {
		t.Fatalf("expected 1 meta tag after replace, got %d", len(got2))
	}

	// Update
	updated, err := repo.Update(ctx, created.ID, "루리웹 커뮤니티", "https://ruliweb.com", false)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Name != "루리웹 커뮤니티" || updated.Enabled != false {
		t.Fatalf("update not applied: %+v", updated)
	}

	// 중복 key 거부
	_, err = repo.Create(ctx, communitytrend.Community{Key: "ruliweb", Name: "dup"})
	if err == nil {
		t.Fatal("expected duplicate key error")
	}

	// Delete + cascade (메타태그도 삭제)
	if err := repo.Delete(ctx, created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	list2, _ := repo.List(ctx)
	if len(list2) != 4 {
		t.Fatalf("expected 4 after delete, got %d", len(list2))
	}
}
```

테스트 파일 import에 `"ota/domain/communitytrend"`, `"ota/storage"` 추가 확인.

- [ ] **Step 2: 테스트 실패 확인**

Run: `cd server && go test ./integration/ -run TestCommunityTrend_CommunityRepo -v`
Expected: FAIL — 컴파일 에러 `undefined: storage.NewCTCommunityRepository`.

- [ ] **Step 3: 저장소 구현**

`server/storage/ct_community_repo.go`:

```go
package storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"ota/domain/communitytrend"
)

// CTCommunityRepository implements communitytrend.CommunityRepository.
type CTCommunityRepository struct {
	pool *pgxpool.Pool
}

func NewCTCommunityRepository(pool *pgxpool.Pool) *CTCommunityRepository {
	return &CTCommunityRepository{pool: pool}
}

func (r *CTCommunityRepository) Create(ctx context.Context, c communitytrend.Community) (communitytrend.Community, error) {
	query := `
		INSERT INTO ct_communities (key, name, home_url, enabled)
		VALUES ($1, $2, $3, $4)
		RETURNING id, key, name, home_url, enabled, created_at, updated_at`
	var out communitytrend.Community
	err := r.pool.QueryRow(ctx, query, c.Key, c.Name, c.HomeURL, c.Enabled).Scan(
		&out.ID, &out.Key, &out.Name, &out.HomeURL, &out.Enabled, &out.CreatedAt, &out.UpdatedAt,
	)
	if err != nil {
		return communitytrend.Community{}, fmt.Errorf("create community: %w", err)
	}
	return out, nil
}

func (r *CTCommunityRepository) List(ctx context.Context) ([]communitytrend.Community, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, key, name, home_url, enabled, created_at, updated_at
		 FROM ct_communities ORDER BY key`)
	if err != nil {
		return nil, fmt.Errorf("list communities: %w", err)
	}
	defer rows.Close()

	var result []communitytrend.Community
	for rows.Next() {
		var c communitytrend.Community
		if err := rows.Scan(&c.ID, &c.Key, &c.Name, &c.HomeURL, &c.Enabled, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan community: %w", err)
		}
		result = append(result, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate communities: %w", err)
	}
	return result, nil
}

func (r *CTCommunityRepository) Update(ctx context.Context, id int, name, homeURL string, enabled bool) (communitytrend.Community, error) {
	query := `
		UPDATE ct_communities SET name=$2, home_url=$3, enabled=$4, updated_at=now()
		WHERE id=$1
		RETURNING id, key, name, home_url, enabled, created_at, updated_at`
	var out communitytrend.Community
	err := r.pool.QueryRow(ctx, query, id, name, homeURL, enabled).Scan(
		&out.ID, &out.Key, &out.Name, &out.HomeURL, &out.Enabled, &out.CreatedAt, &out.UpdatedAt,
	)
	if err != nil {
		return communitytrend.Community{}, fmt.Errorf("update community: %w", err)
	}
	return out, nil
}

func (r *CTCommunityRepository) Delete(ctx context.Context, id int) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM ct_communities WHERE id=$1`, id)
	if err != nil {
		return fmt.Errorf("delete community: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("community not found")
	}
	return nil
}

// SetMetaTags replaces the full meta-tag set atomically.
func (r *CTCommunityRepository) SetMetaTags(ctx context.Context, communityID int, tagIDs []int) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM ct_community_tags WHERE community_id=$1`, communityID); err != nil {
		return fmt.Errorf("clear meta tags: %w", err)
	}
	for _, tagID := range tagIDs {
		if _, err := tx.Exec(ctx,
			`INSERT INTO ct_community_tags (community_id, tag_id) VALUES ($1, $2)`,
			communityID, tagID); err != nil {
			return fmt.Errorf("attach meta tag %d: %w", tagID, err)
		}
	}
	return tx.Commit(ctx)
}

func (r *CTCommunityRepository) GetMetaTags(ctx context.Context, communityID int) ([]int, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT tag_id FROM ct_community_tags WHERE community_id=$1 ORDER BY tag_id`, communityID)
	if err != nil {
		return nil, fmt.Errorf("get meta tags: %w", err)
	}
	defer rows.Close()

	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan meta tag id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate meta tags: %w", err)
	}
	return ids, nil
}
```

- [ ] **Step 4: 테스트 통과 확인**

Run: `cd server && go test ./integration/ -run TestCommunityTrend_CommunityRepo -v`
Expected: PASS.

- [ ] **Step 5: 커밋**

```bash
git add server/storage/ct_community_repo.go server/integration/community_trend_test.go
git commit -m "feat(community-trend): add community repository with meta-tag attach"
```

---

## Task 4: 축 & 태그 저장소 구현

**Files:**
- Create: `server/storage/ct_axis_repo.go`
- Create: `server/storage/ct_tag_repo.go`
- Test: `server/integration/community_trend_test.go` (함수 추가)

**Interfaces:**
- Consumes: `communitytrend.Axis`, `communitytrend.Tag`, 인터페이스 `AxisRepository`, `TagRepository`.
- Produces: `storage.NewCTAxisRepository(pool) *CTAxisRepository`, `storage.NewCTTagRepository(pool) *CTTagRepository`.

- [ ] **Step 1: 통합 테스트 작성 (실패 예정)**

`server/integration/community_trend_test.go`에 추가:

```go
func TestCommunityTrend_AxisAndTagRepo(t *testing.T) {
	db := SetupTestDB(t)
	ctx := context.Background()
	axisRepo := storage.NewCTAxisRepository(db.Pool)
	tagRepo := storage.NewCTTagRepository(db.Pool)

	// 시드 축 5개
	axes, err := axisRepo.List(ctx)
	if err != nil {
		t.Fatalf("list axes: %v", err)
	}
	if len(axes) != 5 {
		t.Fatalf("expected 5 seed axes, got %d", len(axes))
	}

	// 신규 축
	newAxis, err := axisRepo.Create(ctx, communitytrend.Axis{Key: "social", Label: "사회논제축", DisplayOrder: 6})
	if err != nil {
		t.Fatalf("create axis: %v", err)
	}

	// 신규 태그 (정밀 명명: '우파 지지' 같은 형태)
	tag, err := tagRepo.Create(ctx, communitytrend.Tag{
		AxisID: newAxis.ID, Name: "지역 격차", Description: "지역 간 불균형 논제", CreatedBy: "admin",
	})
	if err != nil {
		t.Fatalf("create tag: %v", err)
	}
	if tag.ID == 0 {
		t.Fatal("expected non-zero tag id")
	}

	// 같은 축에서 중복 이름 거부
	_, err = tagRepo.Create(ctx, communitytrend.Tag{AxisID: newAxis.ID, Name: "지역 격차", CreatedBy: "admin"})
	if err == nil {
		t.Fatal("expected duplicate (axis,name) error")
	}

	// ListByAxis
	byAxis, err := tagRepo.ListByAxis(ctx, newAxis.ID)
	if err != nil {
		t.Fatalf("list by axis: %v", err)
	}
	if len(byAxis) != 1 {
		t.Fatalf("expected 1 tag in axis, got %d", len(byAxis))
	}

	// List (시드 6 + 신규 1 = 7)
	all, _ := tagRepo.List(ctx)
	if len(all) != 7 {
		t.Fatalf("expected 7 tags, got %d", len(all))
	}

	// Update
	upd, err := tagRepo.Update(ctx, tag.ID, "수도권 집중", "수도권 인구·자원 집중 논제")
	if err != nil {
		t.Fatalf("update tag: %v", err)
	}
	if upd.Name != "수도권 집중" {
		t.Fatalf("update not applied: %+v", upd)
	}

	// Delete
	if err := tagRepo.Delete(ctx, tag.ID); err != nil {
		t.Fatalf("delete tag: %v", err)
	}
	all2, _ := tagRepo.List(ctx)
	if len(all2) != 6 {
		t.Fatalf("expected 6 tags after delete, got %d", len(all2))
	}
}
```

- [ ] **Step 2: 테스트 실패 확인**

Run: `cd server && go test ./integration/ -run TestCommunityTrend_AxisAndTagRepo -v`
Expected: FAIL — `undefined: storage.NewCTAxisRepository`.

- [ ] **Step 3: 축 저장소 구현**

`server/storage/ct_axis_repo.go`:

```go
package storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"ota/domain/communitytrend"
)

// CTAxisRepository implements communitytrend.AxisRepository.
type CTAxisRepository struct {
	pool *pgxpool.Pool
}

func NewCTAxisRepository(pool *pgxpool.Pool) *CTAxisRepository {
	return &CTAxisRepository{pool: pool}
}

func (r *CTAxisRepository) Create(ctx context.Context, a communitytrend.Axis) (communitytrend.Axis, error) {
	query := `
		INSERT INTO ct_axes (key, label, display_order)
		VALUES ($1, $2, $3)
		RETURNING id, key, label, display_order`
	var out communitytrend.Axis
	err := r.pool.QueryRow(ctx, query, a.Key, a.Label, a.DisplayOrder).Scan(
		&out.ID, &out.Key, &out.Label, &out.DisplayOrder,
	)
	if err != nil {
		return communitytrend.Axis{}, fmt.Errorf("create axis: %w", err)
	}
	return out, nil
}

func (r *CTAxisRepository) List(ctx context.Context) ([]communitytrend.Axis, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, key, label, display_order FROM ct_axes ORDER BY display_order, id`)
	if err != nil {
		return nil, fmt.Errorf("list axes: %w", err)
	}
	defer rows.Close()

	var result []communitytrend.Axis
	for rows.Next() {
		var a communitytrend.Axis
		if err := rows.Scan(&a.ID, &a.Key, &a.Label, &a.DisplayOrder); err != nil {
			return nil, fmt.Errorf("scan axis: %w", err)
		}
		result = append(result, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate axes: %w", err)
	}
	return result, nil
}
```

- [ ] **Step 4: 태그 저장소 구현**

`server/storage/ct_tag_repo.go`:

```go
package storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"ota/domain/communitytrend"
)

// CTTagRepository implements communitytrend.TagRepository.
type CTTagRepository struct {
	pool *pgxpool.Pool
}

func NewCTTagRepository(pool *pgxpool.Pool) *CTTagRepository {
	return &CTTagRepository{pool: pool}
}

const ctTagCols = `id, axis_id, name, description, created_by, created_at`

func (r *CTTagRepository) Create(ctx context.Context, t communitytrend.Tag) (communitytrend.Tag, error) {
	createdBy := t.CreatedBy
	if createdBy == "" {
		createdBy = "admin"
	}
	query := `
		INSERT INTO ct_tags (axis_id, name, description, created_by)
		VALUES ($1, $2, $3, $4)
		RETURNING ` + ctTagCols
	var out communitytrend.Tag
	err := r.pool.QueryRow(ctx, query, t.AxisID, t.Name, t.Description, createdBy).Scan(
		&out.ID, &out.AxisID, &out.Name, &out.Description, &out.CreatedBy, &out.CreatedAt,
	)
	if err != nil {
		return communitytrend.Tag{}, fmt.Errorf("create tag: %w", err)
	}
	return out, nil
}

func (r *CTTagRepository) scanList(ctx context.Context, query string, args ...any) ([]communitytrend.Tag, error) {
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query tags: %w", err)
	}
	defer rows.Close()

	var result []communitytrend.Tag
	for rows.Next() {
		var t communitytrend.Tag
		if err := rows.Scan(&t.ID, &t.AxisID, &t.Name, &t.Description, &t.CreatedBy, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan tag: %w", err)
		}
		result = append(result, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tags: %w", err)
	}
	return result, nil
}

func (r *CTTagRepository) List(ctx context.Context) ([]communitytrend.Tag, error) {
	return r.scanList(ctx, `SELECT `+ctTagCols+` FROM ct_tags ORDER BY axis_id, name`)
}

func (r *CTTagRepository) ListByAxis(ctx context.Context, axisID int) ([]communitytrend.Tag, error) {
	return r.scanList(ctx, `SELECT `+ctTagCols+` FROM ct_tags WHERE axis_id=$1 ORDER BY name`, axisID)
}

func (r *CTTagRepository) Update(ctx context.Context, id int, name, description string) (communitytrend.Tag, error) {
	query := `UPDATE ct_tags SET name=$2, description=$3 WHERE id=$1 RETURNING ` + ctTagCols
	var out communitytrend.Tag
	err := r.pool.QueryRow(ctx, query, id, name, description).Scan(
		&out.ID, &out.AxisID, &out.Name, &out.Description, &out.CreatedBy, &out.CreatedAt,
	)
	if err != nil {
		return communitytrend.Tag{}, fmt.Errorf("update tag: %w", err)
	}
	return out, nil
}

func (r *CTTagRepository) Delete(ctx context.Context, id int) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM ct_tags WHERE id=$1`, id)
	if err != nil {
		return fmt.Errorf("delete tag: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("tag not found")
	}
	return nil
}
```

- [ ] **Step 5: 테스트 통과 확인**

Run: `cd server && go test ./integration/ -run TestCommunityTrend_AxisAndTagRepo -v`
Expected: PASS.

- [ ] **Step 6: 커밋**

```bash
git add server/storage/ct_axis_repo.go server/storage/ct_tag_repo.go server/integration/community_trend_test.go
git commit -m "feat(community-trend): add axis and tag repositories"
```

---

## Task 5: 서비스 계층 (검증)

**Files:**
- Create: `server/domain/communitytrend/service.go`
- Test: `server/domain/communitytrend/service_test.go`

**Interfaces:**
- Consumes: `AxisRepository`, `TagRepository`, `CommunityRepository`.
- Produces: `Service` (생성자 `NewService(communities CommunityRepository, tags TagRepository, axes AxisRepository) *Service`), 메서드:
  - `CreateCommunity(ctx, Community) (Community, error)` — key 형식 검증
  - `ListCommunities(ctx) ([]Community, error)` — 각 커뮤니티에 MetaTagIDs 채움
  - `UpdateCommunity(ctx, id int, name, homeURL string, enabled bool) (Community, error)`
  - `DeleteCommunity(ctx, id int) error`
  - `SetMetaTags(ctx, communityID int, tagIDs []int) error`
  - `ListAxes`, `CreateAxis`, `ListTags`, `ListTagsByAxis`, `CreateTag`, `UpdateTag`, `DeleteTag` (얇은 위임)

- [ ] **Step 1: 서비스 단위 테스트 작성 (실패 예정)**

`server/domain/communitytrend/service_test.go`:

```go
package communitytrend

import (
	"context"
	"testing"
)

// fakeCommunityRepo is an in-memory CommunityRepository for unit tests.
type fakeCommunityRepo struct {
	items    map[int]Community
	metaTags map[int][]int
	nextID   int
	keys     map[string]bool
}

func newFakeCommunityRepo() *fakeCommunityRepo {
	return &fakeCommunityRepo{
		items: map[int]Community{}, metaTags: map[int][]int{}, nextID: 1, keys: map[string]bool{},
	}
}

func (f *fakeCommunityRepo) Create(_ context.Context, c Community) (Community, error) {
	if f.keys[c.Key] {
		return Community{}, errDuplicate
	}
	c.ID = f.nextID
	f.nextID++
	f.items[c.ID] = c
	f.keys[c.Key] = true
	return c, nil
}
func (f *fakeCommunityRepo) List(_ context.Context) ([]Community, error) {
	var out []Community
	for _, c := range f.items {
		out = append(out, c)
	}
	return out, nil
}
func (f *fakeCommunityRepo) Update(_ context.Context, id int, name, homeURL string, enabled bool) (Community, error) {
	c := f.items[id]
	c.Name, c.HomeURL, c.Enabled = name, homeURL, enabled
	f.items[id] = c
	return c, nil
}
func (f *fakeCommunityRepo) Delete(_ context.Context, id int) error { delete(f.items, id); return nil }
func (f *fakeCommunityRepo) SetMetaTags(_ context.Context, id int, tagIDs []int) error {
	f.metaTags[id] = tagIDs
	return nil
}
func (f *fakeCommunityRepo) GetMetaTags(_ context.Context, id int) ([]int, error) {
	return f.metaTags[id], nil
}

// errDuplicate is a sentinel for the fake.
var errDuplicate = &dupErr{}

type dupErr struct{}

func (*dupErr) Error() string { return "duplicate key" }

func TestService_CreateCommunity_ValidatesKey(t *testing.T) {
	svc := NewService(newFakeCommunityRepo(), nil, nil)
	ctx := context.Background()

	tests := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{"valid lower alnum", "fmkorea", false},
		{"valid with dash", "mlb-park", false},
		{"valid with underscore", "the_qoo", false},
		{"empty", "", true},
		{"uppercase", "FmKorea", true},
		{"space", "fm korea", true},
		{"korean", "더쿠", true},
		{"slash", "fm/korea", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.CreateCommunity(ctx, Community{Key: tc.key, Name: "X"})
			if tc.wantErr && err == nil {
				t.Fatalf("key %q: expected error, got nil", tc.key)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("key %q: unexpected error %v", tc.key, err)
			}
		})
	}
}

func TestService_CreateCommunity_RequiresName(t *testing.T) {
	svc := NewService(newFakeCommunityRepo(), nil, nil)
	_, err := svc.CreateCommunity(context.Background(), Community{Key: "valid", Name: ""})
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestService_ListCommunities_PopulatesMetaTags(t *testing.T) {
	repo := newFakeCommunityRepo()
	svc := NewService(repo, nil, nil)
	ctx := context.Background()

	c, _ := svc.CreateCommunity(ctx, Community{Key: "k", Name: "N"})
	_ = svc.SetMetaTags(ctx, c.ID, []int{10, 20})

	list, err := svc.ListCommunities(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 || len(list[0].MetaTagIDs) != 2 {
		t.Fatalf("expected meta tags populated, got %+v", list)
	}
}
```

- [ ] **Step 2: 테스트 실패 확인**

Run: `cd server && go test ./domain/communitytrend/ -v`
Expected: FAIL — `undefined: NewService`.

- [ ] **Step 3: 서비스 구현**

`server/domain/communitytrend/service.go`:

```go
package communitytrend

import (
	"context"
	"fmt"
	"regexp"
)

// keyPattern enforces adapter-linkable community keys: lower alnum, dash, underscore.
var keyPattern = regexp.MustCompile(`^[a-z0-9_-]+$`)

// Service holds validation and orchestration over the repositories.
type Service struct {
	communities CommunityRepository
	tags        TagRepository
	axes        AxisRepository
}

func NewService(communities CommunityRepository, tags TagRepository, axes AxisRepository) *Service {
	return &Service{communities: communities, tags: tags, axes: axes}
}

// --- communities ---

func (s *Service) CreateCommunity(ctx context.Context, c Community) (Community, error) {
	if !keyPattern.MatchString(c.Key) {
		return Community{}, fmt.Errorf("커뮤니티 key는 소문자 영숫자/-/_ 만 허용됩니다")
	}
	if c.Name == "" {
		return Community{}, fmt.Errorf("커뮤니티 이름은 필수입니다")
	}
	return s.communities.Create(ctx, c)
}

func (s *Service) ListCommunities(ctx context.Context) ([]Community, error) {
	list, err := s.communities.List(ctx)
	if err != nil {
		return nil, err
	}
	for i := range list {
		ids, err := s.communities.GetMetaTags(ctx, list[i].ID)
		if err != nil {
			return nil, err
		}
		list[i].MetaTagIDs = ids
	}
	return list, nil
}

func (s *Service) UpdateCommunity(ctx context.Context, id int, name, homeURL string, enabled bool) (Community, error) {
	if name == "" {
		return Community{}, fmt.Errorf("커뮤니티 이름은 필수입니다")
	}
	return s.communities.Update(ctx, id, name, homeURL, enabled)
}

func (s *Service) DeleteCommunity(ctx context.Context, id int) error {
	return s.communities.Delete(ctx, id)
}

func (s *Service) SetMetaTags(ctx context.Context, communityID int, tagIDs []int) error {
	return s.communities.SetMetaTags(ctx, communityID, tagIDs)
}

// --- axes ---

func (s *Service) ListAxes(ctx context.Context) ([]Axis, error) { return s.axes.List(ctx) }

func (s *Service) CreateAxis(ctx context.Context, a Axis) (Axis, error) {
	if a.Key == "" || a.Label == "" {
		return Axis{}, fmt.Errorf("축 key와 label은 필수입니다")
	}
	return s.axes.Create(ctx, a)
}

// --- tags ---

func (s *Service) ListTags(ctx context.Context) ([]Tag, error) { return s.tags.List(ctx) }

func (s *Service) ListTagsByAxis(ctx context.Context, axisID int) ([]Tag, error) {
	return s.tags.ListByAxis(ctx, axisID)
}

func (s *Service) CreateTag(ctx context.Context, t Tag) (Tag, error) {
	if t.AxisID == 0 {
		return Tag{}, fmt.Errorf("축은 필수입니다")
	}
	if t.Name == "" {
		return Tag{}, fmt.Errorf("태그 이름은 필수입니다")
	}
	return s.tags.Create(ctx, t)
}

func (s *Service) UpdateTag(ctx context.Context, id int, name, description string) (Tag, error) {
	if name == "" {
		return Tag{}, fmt.Errorf("태그 이름은 필수입니다")
	}
	return s.tags.Update(ctx, id, name, description)
}

func (s *Service) DeleteTag(ctx context.Context, id int) error { return s.tags.Delete(ctx, id) }
```

- [ ] **Step 4: 테스트 통과 확인**

Run: `cd server && go test ./domain/communitytrend/ -v`
Expected: PASS (모든 서브테스트).

- [ ] **Step 5: 커밋**

```bash
git add server/domain/communitytrend/service.go server/domain/communitytrend/service_test.go
git commit -m "feat(community-trend): add service layer with validation"
```

---

## Task 6: 관리자 핸들러 + 라우트 + 와이어링

**Files:**
- Create: `server/api/handler/community_trend_handler.go`
- Modify: `server/main.go` (repos→service→handler 생성, RouteModule 추가)
- Test: `server/integration/community_trend_test.go` (HTTP 통합 함수 추가)

**Interfaces:**
- Consumes: `communitytrend.Service`.
- Produces: `handler.NewCommunityTrendAdminHandler(svc *communitytrend.Service) *CommunityTrendAdminHandler`, `RegisterRoutes(group)`.

- [ ] **Step 1: HTTP 통합 테스트 작성 (실패 예정)**

`server/integration/community_trend_test.go`에 추가 (import에 `bytes`, `encoding/json`, `net/http`, `net/http/httptest`, `github.com/gin-gonic/gin`, `github.com/ulule/limiter/v3/drivers/store/memory`, `ota/api`, `ota/api/handler`, `ota/auth` 필요):

```go
func TestCommunityTrend_AdminHTTP(t *testing.T) {
	db := SetupTestDB(t)

	svc := communitytrend.NewService(
		storage.NewCTCommunityRepository(db.Pool),
		storage.NewCTTagRepository(db.Pool),
		storage.NewCTAxisRepository(db.Pool),
	)
	adminHandler := handler.NewCommunityTrendAdminHandler(svc)

	gin.SetMode(gin.TestMode)
	jwtManager := auth.NewJWTManager("test-secret")
	router := api.NewRouter("api", "v1", "http://localhost:5173", jwtManager, 10000, memory.NewStore(),
		[]api.RouteModule{
			{GroupName: "admin/community-trend", Handler: adminHandler, Middlewares: []gin.HandlerFunc{}},
		})

	// 커뮤니티 목록 (시드 4)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/admin/community-trend/communities", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("list communities: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var listResp struct {
		Data []communitytrend.Community `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &listResp)
	if len(listResp.Data) != 4 {
		t.Fatalf("expected 4 seed communities, got %d", len(listResp.Data))
	}

	// 커뮤니티 생성
	body, _ := json.Marshal(map[string]any{"key": "mlbpark", "name": "엠팍", "home_url": "https://mlbpark.donga.com"})
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("POST", "/api/v1/admin/community-trend/communities", bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w2, req2)
	if w2.Code != http.StatusCreated {
		t.Fatalf("create community: expected 201, got %d: %s", w2.Code, w2.Body.String())
	}
	var createResp struct {
		Data communitytrend.Community `json:"data"`
	}
	json.Unmarshal(w2.Body.Bytes(), &createResp)
	commID := createResp.Data.ID

	// 잘못된 key 거부
	badBody, _ := json.Marshal(map[string]any{"key": "MLB Park", "name": "x"})
	w3 := httptest.NewRecorder()
	req3, _ := http.NewRequest("POST", "/api/v1/admin/community-trend/communities", bytes.NewReader(badBody))
	req3.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w3, req3)
	if w3.Code != http.StatusBadRequest {
		t.Fatalf("bad key: expected 400, got %d", w3.Code)
	}

	// 태그 목록 (시드 6)
	w4 := httptest.NewRecorder()
	req4, _ := http.NewRequest("GET", "/api/v1/admin/community-trend/tags", nil)
	router.ServeHTTP(w4, req4)
	var tagsResp struct {
		Data []communitytrend.Tag `json:"data"`
	}
	json.Unmarshal(w4.Body.Bytes(), &tagsResp)
	if len(tagsResp.Data) != 6 {
		t.Fatalf("expected 6 seed tags, got %d", len(tagsResp.Data))
	}

	// 메타태그 부착 (첫 2개 태그)
	metaBody, _ := json.Marshal(map[string]any{"tag_ids": []int{tagsResp.Data[0].ID, tagsResp.Data[1].ID}})
	w5 := httptest.NewRecorder()
	req5, _ := http.NewRequest("PUT",
		"/api/v1/admin/community-trend/communities/"+itoa(commID)+"/meta-tags",
		bytes.NewReader(metaBody))
	req5.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w5, req5)
	if w5.Code != http.StatusOK {
		t.Fatalf("set meta tags: expected 200, got %d: %s", w5.Code, w5.Body.String())
	}

	// 목록에서 메타태그 반영 확인
	w6 := httptest.NewRecorder()
	req6, _ := http.NewRequest("GET", "/api/v1/admin/community-trend/communities", nil)
	router.ServeHTTP(w6, req6)
	var listResp2 struct {
		Data []communitytrend.Community `json:"data"`
	}
	json.Unmarshal(w6.Body.Bytes(), &listResp2)
	var found bool
	for _, c := range listResp2.Data {
		if c.ID == commID && len(c.MetaTagIDs) == 2 {
			found = true
		}
	}
	if !found {
		t.Fatal("expected created community to have 2 meta tags")
	}
}

// itoa is a tiny local helper to avoid importing strconv at call sites.
func itoa(n int) string { return fmt.Sprintf("%d", n) }
```

테스트 파일 상단 import에 `"fmt"` 추가 확인.

- [ ] **Step 2: 테스트 실패 확인**

Run: `cd server && go test ./integration/ -run TestCommunityTrend_AdminHTTP -v`
Expected: FAIL — `undefined: handler.NewCommunityTrendAdminHandler`.

- [ ] **Step 3: 핸들러 구현**

`server/api/handler/community_trend_handler.go`:

```go
package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"ota/domain/communitytrend"
)

// CommunityTrendAdminHandler exposes admin CRUD for communities, axes, and tags.
type CommunityTrendAdminHandler struct {
	svc *communitytrend.Service
}

func NewCommunityTrendAdminHandler(svc *communitytrend.Service) *CommunityTrendAdminHandler {
	return &CommunityTrendAdminHandler{svc: svc}
}

// RegisterRoutes registers admin routes under /api/v1/admin/community-trend.
func (h *CommunityTrendAdminHandler) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/communities", h.ListCommunities)
	group.POST("/communities", h.CreateCommunity)
	group.PATCH("/communities/:id", h.UpdateCommunity)
	group.DELETE("/communities/:id", h.DeleteCommunity)
	group.PUT("/communities/:id/meta-tags", h.SetMetaTags)

	group.GET("/axes", h.ListAxes)
	group.POST("/axes", h.CreateAxis)

	group.GET("/tags", h.ListTags) // optional ?axis_id=
	group.POST("/tags", h.CreateTag)
	group.PATCH("/tags/:id", h.UpdateTag)
	group.DELETE("/tags/:id", h.DeleteTag)
}

func parseIDParam(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "잘못된 ID입니다"})
		return 0, false
	}
	return id, true
}

// --- communities ---

func (h *CommunityTrendAdminHandler) ListCommunities(c *gin.Context) {
	list, err := h.svc.ListCommunities(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "커뮤니티 목록을 불러올 수 없습니다"})
		return
	}
	if list == nil {
		list = []communitytrend.Community{}
	}
	c.JSON(http.StatusOK, gin.H{"data": list})
}

type createCommunityRequest struct {
	Key     string `json:"key"`
	Name    string `json:"name"`
	HomeURL string `json:"home_url"`
}

func (h *CommunityTrendAdminHandler) CreateCommunity(c *gin.Context) {
	var req createCommunityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "잘못된 요청 형식입니다"})
		return
	}
	created, err := h.svc.CreateCommunity(c.Request.Context(), communitytrend.Community{
		Key: req.Key, Name: req.Name, HomeURL: req.HomeURL, Enabled: true,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": created})
}

type updateCommunityRequest struct {
	Name    string `json:"name"`
	HomeURL string `json:"home_url"`
	Enabled *bool  `json:"enabled"`
}

func (h *CommunityTrendAdminHandler) UpdateCommunity(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var req updateCommunityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "잘못된 요청 형식입니다"})
		return
	}
	if req.Enabled == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "활성 상태는 필수입니다"})
		return
	}
	updated, err := h.svc.UpdateCommunity(c.Request.Context(), id, req.Name, req.HomeURL, *req.Enabled)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": updated})
}

func (h *CommunityTrendAdminHandler) DeleteCommunity(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	if err := h.svc.DeleteCommunity(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

type setMetaTagsRequest struct {
	TagIDs []int `json:"tag_ids"`
}

func (h *CommunityTrendAdminHandler) SetMetaTags(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var req setMetaTagsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "잘못된 요청 형식입니다"})
		return
	}
	if err := h.svc.SetMetaTags(c.Request.Context(), id, req.TagIDs); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

// --- axes ---

func (h *CommunityTrendAdminHandler) ListAxes(c *gin.Context) {
	list, err := h.svc.ListAxes(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "축 목록을 불러올 수 없습니다"})
		return
	}
	if list == nil {
		list = []communitytrend.Axis{}
	}
	c.JSON(http.StatusOK, gin.H{"data": list})
}

type createAxisRequest struct {
	Key          string `json:"key"`
	Label        string `json:"label"`
	DisplayOrder int    `json:"display_order"`
}

func (h *CommunityTrendAdminHandler) CreateAxis(c *gin.Context) {
	var req createAxisRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "잘못된 요청 형식입니다"})
		return
	}
	created, err := h.svc.CreateAxis(c.Request.Context(), communitytrend.Axis{
		Key: req.Key, Label: req.Label, DisplayOrder: req.DisplayOrder,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": created})
}

// --- tags ---

func (h *CommunityTrendAdminHandler) ListTags(c *gin.Context) {
	ctx := c.Request.Context()
	if raw := c.Query("axis_id"); raw != "" {
		axisID, err := strconv.Atoi(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "잘못된 axis_id입니다"})
			return
		}
		list, err := h.svc.ListTagsByAxis(ctx, axisID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "태그 목록을 불러올 수 없습니다"})
			return
		}
		if list == nil {
			list = []communitytrend.Tag{}
		}
		c.JSON(http.StatusOK, gin.H{"data": list})
		return
	}
	list, err := h.svc.ListTags(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "태그 목록을 불러올 수 없습니다"})
		return
	}
	if list == nil {
		list = []communitytrend.Tag{}
	}
	c.JSON(http.StatusOK, gin.H{"data": list})
}

type createTagRequest struct {
	AxisID      int    `json:"axis_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (h *CommunityTrendAdminHandler) CreateTag(c *gin.Context) {
	var req createTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "잘못된 요청 형식입니다"})
		return
	}
	created, err := h.svc.CreateTag(c.Request.Context(), communitytrend.Tag{
		AxisID: req.AxisID, Name: req.Name, Description: req.Description, CreatedBy: "admin",
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": created})
}

type updateTagRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (h *CommunityTrendAdminHandler) UpdateTag(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var req updateTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "잘못된 요청 형식입니다"})
		return
	}
	updated, err := h.svc.UpdateTag(c.Request.Context(), id, req.Name, req.Description)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": updated})
}

func (h *CommunityTrendAdminHandler) DeleteTag(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	if err := h.svc.DeleteTag(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}
```

- [ ] **Step 4: main.go 와이어링**

`server/main.go`에서 핸들러 생성 블록(예: `termsAdminHandler := ...` 줄 근처, Task 검색어 `termsAdminHandler`)에 추가:

```go
	communityTrendService := communitytrend.NewService(
		storage.NewCTCommunityRepository(pool),
		storage.NewCTTagRepository(pool),
		storage.NewCTAxisRepository(pool),
	)
	communityTrendAdminHandler := handler.NewCommunityTrendAdminHandler(communityTrendService)
```

> `pool` 은 main.go에서 이미 생성된 `*pgxpool.Pool` 변수명. 다를 경우 기존 repo 생성(예 `storage.NewTermsRepository(...)`)에 넘기는 인자명과 동일하게 맞춘다.

`server/main.go` import 블록에 `"ota/domain/communitytrend"` 추가.

`NewRouter(... []api.RouteModule{ ... })` 슬라이스에 admin 모듈 추가 (다른 `admin/...` 모듈과 동일 미들웨어):

```go
		{
			GroupName:   "admin/community-trend",
			Handler:     communityTrendAdminHandler,
			Middlewares: []gin.HandlerFunc{api.AuthMiddleware(jwtManager), api.AdminMiddleware(userRepo)},
		},
```

- [ ] **Step 5: 빌드 + 테스트 통과 확인**

Run: `cd server && go build ./... && go test ./integration/ -run TestCommunityTrend_AdminHTTP -v`
Expected: 빌드 성공, 테스트 PASS.

- [ ] **Step 6: 전체 회귀 확인**

Run: `cd server && go test ./domain/communitytrend/ ./integration/ -run TestCommunityTrend -race`
Expected: 전부 PASS, 레이스 없음.

- [ ] **Step 7: 커밋**

```bash
git add server/api/handler/community_trend_handler.go server/main.go server/integration/community_trend_test.go
git commit -m "feat(community-trend): add admin handler, routes, and wiring"
```

---

## 이후 플랜 로드맵 (각각 독립 테스트 가능, 별도 문서로 작성 예정)

- **#02 수집 파이프라인**: `SourceAdapter` 인터페이스 + 레지스트리, 정중한 HTTP(`platform/communities/fetch.go`), robots 게이트, 지문/dedup, dogdrip·clien 실어댑터(fixtures 테스트), `Tagger` 인터페이스 + gemini 구현 + 프롬프트(1패스), 워크시트 상태기계, 일일 스케줄러 잡(auto), confirm 엔드포인트→`ct_tag_daily` 기록. **산출물**: dogdrip/clien 자동수집→AI제안→사람확정 + fmkorea/더쿠 수동입력 end-to-end.
- **#03 밈 엔진**: 후보/확정 상태기계, blacklist, 승격, retire, AI 1패스 통합, `ct_meme_daily` 카운트, 밈 API.
- **#04 집계/증감 쿼리**: `aggregate.go` — 일자별 증감(델타) + 코호트(메타태그 필터) 롤업 + `CT_MIN_TAG_COUNT`(기본 3) 노출 필터. 조회 API. **이게 "데이터 준비 완료"의 최종 산출물.**
- **#05 관리자 UI**: `admin-community-trend.tsx` — 워크시트 보드, 통합 태깅 화면, 밈 패널, 베이스 관리.

---

## Self-Review

**1. Spec coverage (이 플랜 범위):** spec §2 격리 ✓(Task 2,3,4,6), §3 개념모델 ✓(메타태그=코호트, key=어댑터연동), §4.1 커뮤니티&태그 테이블 ✓(Task 1), §4.2~4.6 나머지 테이블 ✓(Task 1 생성, 사용은 #02~04), §10 커뮤니티/태그/축 API ✓(Task 6). 파이프라인·AI·밈·집계·UI = #02~05로 명시 이월.

**2. Placeholder scan:** "TBD"/"적절히" 없음. 모든 step에 실제 코드/명령/기대출력 포함. main.go 와이어링은 변수명(`pool`) 의존 — 노트로 명시.

**3. Type consistency:** `Community.MetaTagIDs []int` (model) ↔ `GetMetaTags []int` (repo) ↔ `ListCommunities` 채움(service) ↔ HTTP 응답 검증 일치. `NewService(communities, tags, axes)` 인자 순서 = Task5 정의 = Task6 와이어링 호출 일치. 생성자명 `NewCTCommunityRepository`/`NewCTTagRepository`/`NewCTAxisRepository` 전 Task 동일. `NewCommunityTrendAdminHandler` Task6 정의=테스트 호출 일치.
