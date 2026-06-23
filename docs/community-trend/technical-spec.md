# 커뮤니티 트렌드 — 기술 스펙

> 상태: 초안 (2026-06-24) · 선행 문서: `방향성-기획.html`
> 목적: 한국 대형 커뮤니티들의 **논제(트렌드) 데이터**를 수집·집계한다. 본 피쳐는 **데이터 수집**까지다. 표현(그래프/발행물)은 별도.

---

## 1. 목표 / 비목표

**목표**
- 대상 커뮤니티의 대표 게시판에서 **최근 1일 글**의 *논제*를 파악해 `(커뮤니티 × 태그 × 날짜 = 글 개수)` 집계로 쌓는다.
- 자동(어댑터 크롤) + 수동(사람 입력) 두 경로가 **동일 스키마**로 합류한다.
- 신조어/밈 **후보 발굴 + 확정 밈 카운트**를 보조 트랙으로 제공한다.
- 모든 수집은 합법성 가드레일(robots 추적, 본문 미저장, 정중한 요청) 안에서 동작한다.

**비목표 (이번 범위 밖)**
- 그래프/대시보드/발행물 등 **표현 레이어**.
- 유튜브·트위터 등 비게시판 소스 어댑터 (구조는 수용하되 구현은 향후).
- 본격 밈 추적(밈 라이프사이클 분석 등). v1은 후보 발굴 + 카운트까지.
- 기존 `collector`(WizLetter 뉴스 파이프라인)와의 어떤 로직 공유도 없음.

---

## 2. 격리 원칙

기존 뉴스 수집(`server/domain/collector`)과 **비즈니스 로직 0 공유.** 공유하는 것은 인프라뿐: DB 풀(pgxpool), AI SDK(`platform/gemini`), HTTP 기반 클라이언트.

```
server/domain/communitytrend/      # 비즈니스 로직 (DB import 없음)
  model.go            # Community, Tag, Axis, Meme, TrendItem, Worksheet ...
  repository.go       # 저장소 인터페이스 모음
  adapter.go          # SourceAdapter 인터페이스 + AdapterRegistry
  robots.go           # robots.txt fetch/parse + allow 판정 + 전이 감지
  dedup.go            # 지문 생성 + seen 검정
  worksheet.go        # 일일 워크시트 생성/제안/확정 상태기계
  tagger.go           # AI 1패스 호출 오케스트레이션 (태그+밈)
  tag_prompt.go       # 프롬프트 빌더 (분류체계 주입, 보수적 규칙)
  meme.go             # 밈 후보/확정 상태기계
  aggregate.go        # 증감(델타) + 코호트 롤업 쿼리 로직
  service.go          # 일일 런 오케스트레이션
  *_test.go

server/platform/communities/       # 사이트별 어댑터 (개발자 영역)
  fetch.go            # 정중한 HTTP (UA, crawl-delay, 조건부 GET, 타임아웃)
  dogdrip.go, clien.go, fmkorea.go, theqoo.go ...
  fixtures/           # 테스트용 저장 HTML (CI는 라이브 호출 금지)

server/storage/                    # ct_* 저장소 구현 (pgxpool)
  ct_community_repository.go, ct_tag_repository.go,
  ct_tag_daily_repository.go, ct_robots_repository.go,
  ct_seen_repository.go, ct_worksheet_repository.go, ct_meme_repository.go

server/api/handler/
  community_trend_handler.go        # 관리자 API (커뮤니티/태그/워크시트/밈 CRUD)

server/scheduler/
  community_trend_job.go            # 기존 collect와 별도 일일 잡

server/migrations/
  000043_create_community_trend.up.sql / .down.sql

web/src/pages/
  admin-community-trend.tsx         # 통합 태깅 워크시트 + 커뮤니티/태그/밈 관리 UI
```

Go 모듈명은 `ota` 유지.

---

## 3. 핵심 개념 모델

**태그 = 공통 풀 하나.** 붙는 자리(attachment)로 역할이 갈린다.

```
ct_tags (공통 풀, 각 태그는 axis 1개에 속함)
  예: [성향축]남성향 [성향축]여성향 [연령축]40대이상 [정치성향축]좌성향
      [젠더논제]남성 인권 [정치논제]우파 지지 ...
   │
   ├─[메타 부착] ct_community_tags(community_id, tag_id)
   │     커뮤니티의 영구 성향. "fmkorea = 남성향 + 우파성향".  ← 베이스 작업
   │     ※ 코호트는 이 메타태그에서 파생되는 쿼리. 별도 테이블 없음.
   │
   └─[일일 부착] ct_tag_daily(community_id, tag_id, date, post_count)
         그날 논의된 주제. 어댑터 수집 + 수동 입력 둘 다 여기로.  ← 데일리 작업
```

**코호트 = 별도 엔티티가 아니라 메타태그 필터.**
"남초 통계" = `메타태그 '남성향'이 붙은 커뮤니티들의 일일 카운트 합산`. 정의 변경 = 메타태그 재부착만.

**커뮤니티 ↔ 어댑터 연동 = `key` 문자열 일치(컨벤션).**
커뮤니티는 관리자가 DB로 CRUD(`key="fmkorea"`). 어댑터는 개발자가 코드로 작성(`Key()="fmkorea"`). 런타임에 key로 매칭. 어댑터 있으면 자동 경로, 없으면 수동 경로. 둘 다 결과는 `ct_tag_daily`(community 기준)로 합류.

---

## 4. 데이터 모델 (PostgreSQL, `ct_` 접두, 기존 테이블과 FK 없음 · `users`만 actor 참조)

> **저작권 가드레일: 글 본문·제목은 어떤 테이블에도 저장하지 않는다.** 제목은 워크시트 작업 중 메모리/캐시(TTL)로만 존재하다 폐기. 중복검정은 역산 불가능한 해시 지문만 저장.

### 4.1 커뮤니티 & 태그

```sql
CREATE TABLE ct_axes (
  id            SERIAL PRIMARY KEY,
  key           TEXT NOT NULL UNIQUE,          -- 'leaning', 'age', 'political', 'gender_topic' ...
  label         TEXT NOT NULL,                 -- '성향축', '연령축' ...
  display_order INT  NOT NULL DEFAULT 0
);

CREATE TABLE ct_tags (
  id          SERIAL PRIMARY KEY,
  axis_id     INT  NOT NULL REFERENCES ct_axes(id),
  name        TEXT NOT NULL,                   -- '남성 인권', '우파 지지', '남성향' ...
  description TEXT,
  created_by  TEXT NOT NULL DEFAULT 'admin',   -- 'seed' | 'ai' | 'admin'
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (axis_id, name)
);

CREATE TABLE ct_communities (
  id          SERIAL PRIMARY KEY,
  key         TEXT NOT NULL UNIQUE,            -- 'fmkorea' — 어댑터 연동 자연키
  name        TEXT NOT NULL,                   -- '에펨코리아'
  home_url    TEXT,
  enabled     BOOLEAN NOT NULL DEFAULT TRUE,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 메타 부착(영구 성향) = 코호트 차원
CREATE TABLE ct_community_tags (
  community_id INT NOT NULL REFERENCES ct_communities(id) ON DELETE CASCADE,
  tag_id       INT NOT NULL REFERENCES ct_tags(id) ON DELETE RESTRICT,
  PRIMARY KEY (community_id, tag_id)
);
```

### 4.2 일일 집계 (핵심 팩트)

```sql
-- 일일 주제 태그 카운트 (자동+수동 합류)
CREATE TABLE ct_tag_daily (
  community_id INT  NOT NULL REFERENCES ct_communities(id) ON DELETE CASCADE,
  tag_id       INT  NOT NULL REFERENCES ct_tags(id) ON DELETE RESTRICT,
  stat_date    DATE NOT NULL,
  post_count   INT  NOT NULL DEFAULT 0,
  source       TEXT NOT NULL DEFAULT 'human',  -- 'ai' | 'human' | 'hybrid' (감사용)
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (community_id, tag_id, stat_date)
);

-- 일일 총량 (점유율 계산 분모)
CREATE TABLE ct_community_daily (
  community_id INT  NOT NULL REFERENCES ct_communities(id) ON DELETE CASCADE,
  stat_date    DATE NOT NULL,
  total_posts  INT  NOT NULL DEFAULT 0,        -- 그날 '신규' 관측 항목 수 (§7.2 카운트 의미 참조)
  PRIMARY KEY (community_id, stat_date)
);
```

### 4.3 워크시트 (일일 작업 단위 + 상태)

```sql
-- 커뮤니티 × 날짜 작업 상태. 글/제목은 저장 안 함. 운영 대시보드용 경량 행.
CREATE TABLE ct_worksheets (
  id           SERIAL PRIMARY KEY,
  community_id INT  NOT NULL REFERENCES ct_communities(id) ON DELETE CASCADE,
  stat_date    DATE NOT NULL,
  mode         TEXT NOT NULL,                  -- 'auto' | 'manual'
  status       TEXT NOT NULL DEFAULT 'pending',-- 'pending' | 'suggested' | 'confirmed'
  total_posts  INT,                            -- 확정 시 기록
  confirmed_by UUID REFERENCES users(id),
  confirmed_at TIMESTAMPTZ,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (community_id, stat_date)
);
```

### 4.4 합법성 추적

```sql
CREATE TABLE ct_robots_status (
  community_id  INT  NOT NULL REFERENCES ct_communities(id) ON DELETE CASCADE,
  checked_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  allowed       BOOLEAN NOT NULL,              -- 대표 게시판 경로가 generic UA에 허용되나
  snapshot_hash TEXT,                          -- robots.txt 내용 해시(변경 감지)
  note          TEXT,                          -- '404=규칙없음', 'HTTP 403', 'crawl-delay 10' 등
  PRIMARY KEY (community_id, checked_at)
);

CREATE TABLE ct_robots_transitions (
  id           SERIAL PRIMARY KEY,
  community_id INT NOT NULL REFERENCES ct_communities(id) ON DELETE CASCADE,
  from_allowed BOOLEAN,
  to_allowed   BOOLEAN NOT NULL,
  changed_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### 4.5 중복검정

```sql
-- 한 번 카운트한 항목은 다시 분석/카운트 안 함. 지문은 역산 불가 해시.
CREATE TABLE ct_seen_posts (
  community_id INT  NOT NULL REFERENCES ct_communities(id) ON DELETE CASCADE,
  fingerprint  TEXT NOT NULL,                  -- sha256(community_key + source_item_id)
  first_seen   DATE NOT NULL,
  PRIMARY KEY (community_id, fingerprint)
);
-- 운영: first_seen 오래된 행 주기적 prune (베스트판은 ~1일치만 노출).
```

### 4.6 밈 트랙 (태그와 완전 독립)

```sql
CREATE TABLE ct_memes (
  id          SERIAL PRIMARY KEY,
  name        TEXT NOT NULL UNIQUE,            -- 정식명
  aliases     TEXT[] NOT NULL DEFAULT '{}',    -- 표기 변형: ['ㄹㅇ','레알','레알레알']
  status      TEXT NOT NULL DEFAULT 'active',  -- 'active' | 'retired' (hard delete 아님 = 과거 카운트 보존)
  created_via TEXT NOT NULL,                   -- 'promote'(후보 승격) | 'manual'(직접 입력)
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- AI만 입력. 빈도 누적. 사람이 승격하거나 영구거부.
CREATE TABLE ct_meme_candidates (
  id          SERIAL PRIMARY KEY,
  expression  TEXT NOT NULL UNIQUE,            -- 정규화된 후보 표현
  hit_count   INT  NOT NULL DEFAULT 1,         -- 누적 등장(중복 등록 대신 카운트++)
  first_seen  DATE NOT NULL,
  last_seen   DATE NOT NULL
);

-- 유저가 후보 삭제 시 영구거부. AI 재등록 차단.
CREATE TABLE ct_meme_blacklist (
  expression  TEXT PRIMARY KEY
);

-- 확정 밈 일일 카운트 (태그와 별개 팩트테이블, 구조는 쌍둥이)
CREATE TABLE ct_meme_daily (
  meme_id      INT  NOT NULL REFERENCES ct_memes(id) ON DELETE CASCADE,
  community_id INT  NOT NULL REFERENCES ct_communities(id) ON DELETE CASCADE,
  stat_date    DATE NOT NULL,
  count        INT  NOT NULL DEFAULT 0,
  PRIMARY KEY (meme_id, community_id, stat_date)
);
```

---

## 5. 어댑터 계층 (소스 중립)

수집 허브는 게시판을 가정하지 않는다. 어댑터가 사이트별 → 범용 struct로 정규화.

```go
package communitytrend

// 어댑터가 뱉는 중립 단위. "글"이 아니라 "관측 항목".
type TrendItem struct {
    SourceID   string         // 사이트 내 안정적 식별자 (글번호/영상ID 등). 지문 재료.
    TextUnit   string         // 논제 추출 대상 한 줄(제목 등). **휘발성. 저장 금지.**
    Engagement map[string]int // 범용 지표: {"upvotes":.., "comments":.., "views":..}
    ObservedAt time.Time
}

type SourceAdapter interface {
    Key() string                                  // "fmkorea" — 커뮤니티 key와 일치
    RobotsURL() string                            // "" 이면 robots 게이트 스킵(API 소스 등)
    BestBoardPaths() []string                     // robots 허용 판정 대상 경로들
    FetchRecent(ctx context.Context) ([]TrendItem, error) // 최근 1일 대표 항목
}

type AdapterRegistry interface {
    Get(key string) (SourceAdapter, bool)
    Keys() []string
}
```

- `Engagement`을 map으로 둬 소스마다 다른 지표를 수용(추천/댓글/조회수).
- 게시판 v1 어댑터만 작성. 유튜브/트위터는 향후 같은 인터페이스로 plug.
- `platform/communities/fetch.go` = 정중한 HTTP 공통: 정직한 UA(봇 명시), `Crawl-delay` 준수, 조건부 GET(If-Modified-Since/ETag), 타임아웃, 사이트당 동시성 1.

---

## 6. robots 게이트 (합법성)

매일 모든 enabled 커뮤니티에 대해:
1. 어댑터의 `RobotsURL()` 조회. `""` → 게이트 스킵(allowed=true 취급, note에 사유).
2. robots.txt fetch.
   - HTTP 2xx → 파싱. `BestBoardPaths()`가 generic UA(`*`)에 **허용**이면 `allowed=true`.
   - HTTP 4xx(404 등) → 규칙 없음 = 기본 허용(`allowed=true`, note 기록).
   - HTTP 403/429/타임아웃 → **접근 거부 신호로 간주**(`allowed=false`). anti-bot 벽.
3. `ct_robots_status` 기록. 직전 `allowed`와 다르면 `ct_robots_transitions`에 전이 추가.
4. `allowed=false` → 그날 그 커뮤니티는 **수동 모드**(자동 fetch 안 함).

> robots.txt 자체를 가져오는 것은 보편 허용(읽으라고 존재). 불허 시 우회 없음 — 사람이 대신 입력.

---

## 7. 일일 파이프라인

별도 스케줄러 잡(`community_trend_job.go`). 기존 collect(4-6AM)와 무관. 제안 시각: **매일 새벽**(예 03:00 KST) 자동 단계 실행 → 사람은 낮 동안 확정.

### 7.1 단계

```
단계 0  robots 게이트 (§6) → 커뮤니티별 mode(auto/manual) 결정, 워크시트 생성(status=pending)
단계 1  [auto만] 어댑터 FetchRecent → 지문 생성 → ct_seen_posts와 대조 → 신규 항목만 통과(중복검정)
단계 2  [auto만] AI 1패스(§8): 신규 항목들의 TextUnit 묶음 → 태그제안 + 밈매칭 + 밈후보
          → 워크시트 status=suggested (제안 결과는 캐시/세션에 보관, 제목은 미저장)
단계 3  [auto+manual] 사람이 admin에서 확정:
          - auto: AI 제안 검수·수정 후 확정
          - manual: 사람이 직접 사이트 보고 태그 입력 후 확정
단계 4  확정 시 원자적 쓰기:
          - ct_tag_daily(community, tag, date, count, source) upsert
          - ct_community_daily(community, date, total_posts) upsert
          - 신규 항목 지문 → ct_seen_posts
          - 확정밈 매칭분 → ct_meme_daily
          - 워크시트 status=confirmed, confirmed_by/at 기록
          - 제목/제안 캐시 폐기
```

자동 단계(0~2)는 **사이트별 best-effort** — 한 사이트 어댑터 실패가 런 전체를 죽이지 않음(이미지 단계 패턴과 동일). 실패는 워크시트에 표기되고 사람이 수동으로 메울 수 있음.

### 7.2 카운트 의미 (중요 — §13에서 결정 요청)

**채택안: `post_count` = 그날 "신규로 관측된" 항목 중 해당 태그가 붙은 수.**
- 베스트판에 며칠째 머무는 글은 첫 관측일에 1회만 카운트(중복검정과 일치, 스펙 1번 "중복분석 없도록" 충족).
- `total_posts`도 그날 신규 항목 수 → 점유율 = `tag_new / total_new` 일관.
- 의미: "오늘 이 논제로 **새로** 끓은 글 N건" = 신규 유입(fresh energy) 신호. 재고(carryover) 인플레 없음.
- 트렌드 증감 = 일자별 신규 카운트의 변화. 더 깔끔한 신호라 판단.

---

## 8. AI 태깅 (1패스 통합)

커뮤니티-날짜당 **AI 호출 1회**로 세 결과를 동시에 산출(비용·복잡도 절감):

**입력**: 신규 항목 `TextUnit` 묶음 + 현재 태그 분류체계(축+태그) + 확정밈 목록(alias 포함) + 밈 blacklist.

**출력(구조화 JSON)**:
1. **태그 제안**: 각 논제에 대해 기존 태그 우선 매칭. 정말 없을 때만 신규 태그(+축) 제안. 보수적.
2. **밈 매칭**: 확정밈/alias에 걸리는 표현 → 카운트.
3. **밈 후보**: 기존 태그·일반어·blacklist에 없는 **반복 신조어** → 후보 등록(또는 hit_count++).

**규칙(프롬프트에 명시)**:
- 태그 명명은 정밀하게: "우파"❌ → "우파 지지"⭕.
- 보수적 임계: 같은 논제가 **N건 이상**일 때만 태그 제안(N은 설정, §13).
- 신규 태그·신규 밈후보는 절대 자동 확정 아님 — 사람 게이트 통과 필요.

AI 클라이언트는 신규 도메인 내 자체 인터페이스로 주입(기존 `platform/gemini` 래핑, `collector` 의존 0):
```go
type Tagger interface {
    Analyze(ctx, in TaggerInput) (TaggerOutput, error)
}
```

---

## 9. 밈 상태기계

```
후보(ct_meme_candidates)
  Create : AI만. 이미 있으면 hit_count++/last_seen 갱신. blacklist면 무시.
  Delete : 유저 → ct_meme_blacklist에 등록(영구거부, AI 재등록 차단).
  Promote: 유저 → ct_memes로 승격(created_via='promote').

확정(ct_memes)
  Create : 후보 승격 OR 관리자 직접 입력('manual'). (two-way)
  Read   : 목록/검색
  Update : name·aliases 수정 (alias 관리가 매칭 정확도의 핵심)
  Delete : status='retired'로 끔 (hard delete 아님 → ct_meme_daily 과거 보존)

확정밈 카운트: AI가 매칭만 하면 자동 집계(사람 확정 불필요). 사람 손은 '후보 승격'에만.
```

---

## 10. 관리자 API (`/api/v1/admin/community-trend`, AuthMiddleware + AdminMiddleware)

```
# 커뮤니티 (CRUD)
GET    /communities                 목록(+메타태그, +어댑터 존재여부, +오늘 robots/워크시트 상태)
POST   /communities                 등록 {key, name, home_url}
PATCH  /communities/:id              수정 (enabled 토글 등)
DELETE /communities/:id
PUT    /communities/:id/meta-tags    메타태그 일괄 부착 {tag_ids:[]}

# 태그 / 축 (공통 풀)
GET    /axes ; POST /axes
GET    /tags?axis_id= ; POST /tags ; PATCH /tags/:id ; DELETE /tags/:id

# 워크시트 (일일 작업)
GET    /worksheets?date=            그날 커뮤니티별 상태 보드
GET    /worksheets/:id              AI 제안 + (auto면 신규 항목 제목 — 캐시에서, 미저장)
POST   /worksheets/:id/confirm      {tags:[{tag_id,count}], total_posts} → 단계4 실행

# 밈
GET    /memes ; POST /memes ; PATCH /memes/:id ; DELETE /memes/:id (retire)
GET    /meme-candidates
POST   /meme-candidates/:id/promote
DELETE /meme-candidates/:id         (영구거부)

# robots 추적 노출
GET    /robots-status               커뮤니티별 현재 상태 + 최근 전이
```

응답은 프로젝트 표준 엔벨로프(`success`, `data`, `error`).

---

## 11. 관리자 UI (`admin-community-trend.tsx`)

- **워크시트 보드**: 오늘 날짜, 커뮤니티별 카드(pending/suggested/confirmed, auto/manual 배지).
- **태깅 화면(통합)**: AI 제안 태그 + 카운트가 미리 채워짐(auto). 사람은 추가/수정/삭제 후 [확정·발행]. manual은 빈 화면에서 입력. 같은 컴포넌트.
- **밈 패널**: 후보 리스트(빈도순) → [승격]/[거부]. 확정밈 관리(alias 편집, retire).
- **베이스 관리**: 커뮤니티 CRUD + 메타태그 부착, 태그/축 풀 관리.
- 정/부 2인 당번 운영 가정(이중승인 아님, 단일클릭 확정).

---

## 12. 에러 처리 · 테스트 · 운영

**에러 처리**
- 어댑터/AI 실패는 사이트별 격리(best-effort). 워크시트에 사유 표기 → 수동 폴백.
- robots fetch 실패 → 직전 상태 유지 + stale 표기. 모호하면 보수적으로 `allowed=false`(수동).
- 모든 외부 입력(어댑터 응답·관리자 입력) 경계에서 검증.

**테스트**
- 단위: robots 파서, 지문/dedup, 증감·코호트 집계 수학, 밈 상태기계, 워크시트 상태기계, 태깅 프롬프트 빌더.
- 통합: testcontainers(postgres:16-alpine)로 ct_* 저장소.
- 어댑터: `fixtures/` 저장 HTML 대상. **CI에서 라이브 사이트 호출 금지.**

**운영/스케줄**
- 일일 잡 03:00 KST(설정). robots→fetch→AI제안까지 자동, 확정은 낮 시간 사람.
- `ct_seen_posts` 오래된 행 prune 잡.

---

## 13. 구현 중 발견한 단순화 제안 & 미해결 모호점

> 너의 부탁대로 점검하며 모은 것. **[채택]** = 스펙에 이미 반영(이의 없으면 그대로). **[결정요청]** = 네 판단 필요.

**[채택] ① `ct_collection_runs` 테이블 제거**
원래 "런 메타" 테이블을 두려 했으나, "런"은 결국 **달력 날짜** 한 개다. 커뮤니티별 상태는 `ct_worksheets`가 (community, date)로 이미 들고 있음. 별도 런 테이블은 군더더기 → 삭제. 모든 것을 `stat_date`로 키잉.

**[채택] ② AI 호출 1패스 통합**
태그 제안 + 밈 매칭 + 밈 후보를 **한 번의 AI 호출**로. 따로 돌리면 호출 3배·복잡도↑. 입력(제목 묶음)이 같으니 합치는 게 자연스럽고 싸다.

**[채택] ③ 코호트 테이블 제거 → 메타태그 파생 쿼리**
지난 논의 반영. `ct_cohorts`/`ct_community_cohorts` 없앰. 코호트 = `ct_community_tags` 필터. v1 코호트 = **단일 메타태그**(예 '남성향'). 복합 조건(남성향 AND 2030)은 향후.

**[채택] ④ 워크시트는 상태만, 글 미저장**
제목/제안은 캐시(otter TTL)·세션에만. `ct_worksheets`는 경량 상태행. 저작권 가드레일과 일치.

**[결정됨] ⑤ 카운트 의미 — "신규 관측분만"(신규유입형) (§7.2)**
첫 관측일만 카운트. carryover(베스트판 잔존) 글은 재카운트 안 함. → 중복검정과 정합, "집계만 저장" 유지, "새로 끓는 양" 신호. 누적 존재감은 포기(트레이드오프 수용). 상세 근거: `decisions.md` D-001.

**[결정됨] ⑥ 보수적 임계값 N — 환경변수, 기본 3**
`CT_MIN_TAG_COUNT`(기본 3). **raw 카운트는 1건도 다 저장**, N은 *통계 노출* 시점 필터(저장 단계 아님). 재수집 없이 튜닝. 상세: `decisions.md` D-002.

**[결정됨] ⑦ 메타/주제 태그 한 풀, 축 공유, 겸용 허용**
풀은 하나, 역할은 부착(메타 vs 일일)으로 구분. 한 태그의 메타·주제 겸용 막지 않음. 상세: `decisions.md` D-003.

**[결정됨] ⑧ 첫 4개 사이트**
자동: dogdrip, clien / 수동: fmkorea, 더쿠. 남초2·여초1·진보1. 상세: `decisions.md` D-004.

**[열린 질문 — 팀]** 축 초기 set, 코호트 메타태그 사전정의 목록, 보수적 N 최종값, 운영 당번.

---

## 14. 마이그레이션 / 롤아웃

- 단일 마이그레이션 `000043_create_community_trend.up.sql`로 §4 전 테이블 + 시드(기본 축, 첫 4개 커뮤니티, 초기 태그 일부).
- 어댑터는 점진 추가(없으면 수동 동작하므로 커뮤니티 먼저 등록 가능).
- 기능 플래그/별도 라우트 그룹으로 기존 서비스와 격리.
