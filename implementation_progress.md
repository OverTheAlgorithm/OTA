
# Implementation Progress

### Data Collection Pipeline (2025-02-15)

#### Decisions Made

1. **Collection 방식**: OpenAI Responses API + `web_search` tool 사용. 웹 스크래핑 없이 AI가 직접 웹 검색 후 한국 트렌딩 토픽을 수집 및 분석.
   - `POST /v1/responses` with `{"type": "web_search"}`
   - 프로토타입 단계에서 가장 간단한 접근. 품질이 부족하면 추후 Naver 키워드 스크래핑 + AI 분석 하이브리드 방식으로 전환 검토.

2. **데이터 모델**: `context_items`는 `collection_runs`의 자식 레코드로 정규화.
   - 기존 JSONB blob 방식 대신 개별 row로 저장.
   - 각 item은 `category` 태그를 가짐 ("top", "entertainment", "finance" 등).
   - "top"은 별도 구조가 아니라 하나의 카테고리로 취급.
   - 이유: 카테고리별 쿼리, 유저 구독 매칭, 확장성.

3. **아키텍처 원칙**: 순수 함수 + 인터페이스 주입.
   - `collector.Service.Collect(ctx)` 는 caller-agnostic 순수 함수.
   - DB, 네트워크 등 impure 의존성은 인터페이스로 주입.
   - HTTP handler, cron scheduler, CLI, test 어디서든 호출 가능.
   - config, database 패키지는 협업 개발자가 구현 (유저 기능과 공유).

#### Implemented Files (Data Collection)

```
server/
├── migrations/
│   ├── 000001_create_collections.up.sql    # collection_runs + context_items 테이블
│   └── 000001_create_collections.down.sql  # rollback
├── internal/
│   ├── ai/
│   │   ├── client.go                       # ai.Client 인터페이스 + OpenAIClient (HTTP 직접 호출)
│   │   └── client_test.go                  # httptest 서버로 모킹, good/bad 케이스
│   └── collector/
│       ├── model.go                        # CollectionRun, ContextItem, CollectionResult, RunStatus 타입
│       ├── repository.go                   # Repository 인터페이스만 (DB 구현 없음)
│       ├── prompt.go                       # 한국어 프롬프트 템플릿
│       ├── service.go                      # Collect(ctx) 핵심 함수
│       └── service_test.go                 # mock ai.Client + mock Repository, 6개 테스트
```

- 테스트: 11개 전체 통과, ai 88.9%, collector 84.4% 커버리지
- 의존성: `github.com/google/uuid` (OpenAI SDK 미사용, 직접 HTTP)
- 코딩 규칙: 리터럴 문자열 대신 전용 타입 + 상수 사용 (예: `RunStatus`, `RunStatusFailed`)

### User Features & Infrastructure (협업 개발자 구현, merged)

#### 구현된 인프라

- **config**: `config.Load()` — .env 기반, `godotenv` 사용. `DatabaseURL()` 메서드 제공.
- **database**: `database.NewPool(ctx, url)` — pgx/v5 connection pool. `RunMigrations(url, path)` — golang-migrate/v4, pgx5 드라이버.
- **server**: `server.New(...)` — Gin 라우터 설정. CORS + JWT Auth 미들웨어.
- **docker-compose.yml**: PostgreSQL 16-alpine, port 5432, user `ota`, db `ota`.

#### 구현된 유저 기능

```
server/
├── internal/
│   ├── config/config.go          # Config 구조체, Load(), DatabaseURL()
│   ├── database/postgres.go      # NewPool(), RunMigrations()
│   ├── server/
│   │   ├── server.go             # Gin 라우터 (API v1 그룹)
│   │   └── middleware.go         # CORS, JWT Auth 미들웨어
│   ├── auth/
│   │   ├── handler.go            # Kakao OAuth 핸들러
│   │   ├── jwt.go                # JWT 토큰 발급/검증
│   │   ├── kakao.go              # Kakao OAuth 클라이언트
│   │   └── state_store.go        # CSRF state 관리
│   └── user/
│       ├── model.go              # User 구조체
│       └── repository.go         # Repository 인터페이스 + PostgresRepository (pgx)
├── migrations/
│   ├── 000001_create_users_table.up.sql
│   └── 000001_create_users_table.down.sql
├── main.go                       # DI 와이어링, 서버 시작
└── .env.example                  # 환경변수 템플릿
```

- 패턴: Repository 인터페이스 + pgx 구현체, DI via 생성자
- 인증: Kakao OAuth2 → JWT (httpOnly 쿠키)
- DB: pgx/v5 raw SQL, golang-migrate/v4

#### Next Steps (Data Collection - COMPLETED)

1. ✅ **collector.Repository 구현체**: PostgresRepository (pgx). `user.PostgresRepository` 패턴 따름.
2. ✅ **config에 AI 프로바이더 설정 추가**: `AI_PROVIDER`, `GEMINI_API_KEY`, `OPENAI_API_KEY` 환경변수.
3. ✅ **main.go에 collector 와이어링**: ai.Client → Repository → Service 초기화.
4. ✅ **실제 테스트**: OpenAI/Gemini API 키로 `Collect(ctx)` 실행하여 한국 트렌딩 토픽 수집 검증.
5. ✅ **Retry logic**: 3회 재시도 + 지수 백오프 (1s → 2s → 4s), 에러 분류 (Network/Infrastructure/Format).
6. ✅ **스케줄러 통합**: cron 또는 scheduler에서 `Collect(ctx)` 호출.
7. ✅ **HTTP handler**: 수동 트리거용 admin endpoint 추가.

### Message Delivery System (2025-02-17 - IN PROGRESS)

#### Phase 1: Database Schema ✅ COMPLETED

**Implemented Files:**
```
server/
├── migrations/
│   ├── 000003_create_delivery_tables.up.sql    # user_preferences, user_subscriptions, delivery_logs
│   └── 000003_create_delivery_tables.down.sql  # rollback
└── integration/
    └── delivery_migration_test.go              # Migration 검증 테스트 (2 tests passing)
```

**Tables Created:**
1. **`user_preferences`** - Tracks who receives messages (delivery_enabled toggle)
   - Primary Key: user_id (FK to users)
   - delivery_enabled BOOLEAN (default true)
   - created_at, updated_at timestamps

2. **`user_subscriptions`** - Tracks topic subscriptions per user
   - user_id + category UNIQUE constraint (no duplicate subscriptions)
   - Index on user_id for fast lookups
   - Categories: "entertainment", "economy", "sports", etc.

3. **`delivery_logs`** - Idempotency and audit trail
   - (run_id, user_id, channel) UNIQUE constraint - prevents duplicate sends
   - Tracks: channel ('email'/'kakao'), status ('sent'/'failed'/'skipped'), error_message
   - Indexes on run_id, user_id, created_at for efficient queries

**Tests:** All migration tests passing (table creation + constraint verification)

#### Phase 2: Message Formatter ✅ COMPLETED

**Implemented Files:**
```
server/domain/delivery/
├── formatter.go       # FormatMessage pure function
└── formatter_test.go  # 6 unit tests
```

**Pure Function:**
- Converts `context_items` + `subscriptions` → `FormattedMessage`
- Always includes "top" category
- Personalizes based on subscriptions
- Generates text + HTML versions
- Korean language output

**Tests:** 6 unit tests passing (empty input, subscription filtering, multiple categories)

#### Phase 3: Email Platform Integration ✅ COMPLETED

**Implemented Files:**
```
server/platform/email/
├── sender.go       # Sender interface + SMTPSender
├── sender_test.go  # 2 unit tests
└── mock_sender.go  # MockSender for testing
```

**Email System:**
- Clean abstraction (Sender interface)
- SMTP implementation (net/smtp)
- MIME multipart messages (text + HTML)
- Mock sender for tests

**Tests:** 2 unit tests passing (MIME generation, configuration)

#### Phase 4: Delivery Service Core ✅ COMPLETED

**Implemented Files:**
```
server/domain/delivery/
├── service.go          # DeliverAll orchestration
├── service_test.go     # 6 unit tests with mocks
└── repository.go       # Repository interface
```

**Orchestration Logic:**
- Fetch latest collection run
- Get eligible users (delivery_enabled=true)
- Format personalized messages
- Send via email
- Log delivery attempts
- Idempotency check (no duplicates)
- Graceful partial failure handling

**Tests:** 6 unit tests passing (success, idempotency, partial failure, edge cases)

#### Phase 5: Repository Implementation ✅ COMPLETED

**Implemented Files:**
```
server/storage/delivery_repository.go       # PostgreSQL implementation
server/integration/delivery_repository_test.go  # 5 integration tests
```

**PostgreSQL Queries:**
- `GetEligibleUsers`: Complex JOIN (users ⋈ preferences ⟕ subscriptions)
- `LogDelivery`: Insert with idempotency
- `HasDeliveryLog`: Duplicate check

**Tests:** 5 integration tests passing (testcontainers, real PostgreSQL)

#### Phase 6: API + Scheduler + Config ✅ COMPLETED

**Implemented Files:**
```
server/
├── config/config.go                        # SMTP config added
├── storage/collector_service_adapter.go    # Adapter for collector
├── api/delivery_handler.go                 # POST /api/v1/delivery/trigger
└── main.go                                 # Full system wiring
```

**System Integration:**
- SMTP configuration (host, port, credentials)
- Delivery service wired in main.go
- Scheduled delivery at 7:15 AM KST daily (15 min after collection)
- Manual trigger endpoint for testing
- Build successful ✅

**API Endpoint:**
- `POST /api/v1/delivery/trigger` - Manual delivery trigger
- Returns: total_users, success_count, failure_count, skipped_count, errors

#### Summary: Email Delivery System COMPLETE ✅

**Total Implementation:**
- 6 Phases completed
- 16 new files created
- 11 files modified
- All tests passing (30+ tests)
- System fully integrated and operational

**Next Steps for Production:**
1. Set SMTP credentials in .env
2. Test with real email addresses
3. Monitor delivery logs
4. (Future) Add Kakao Talk integration

### AI Model Decision Log (2026-02-20)

#### Gemini 모델 선택 기준 및 결정

**결정: `gemini-3.1-pro-preview` 사용**

| 모델 ID | 출시일 | 비고 |
|---|---|---|
| `gemini-3.1-pro-preview` | 2026-02-19 | **현재 사용 중** — 최신, 최고 추론 |
| `gemini-3-flash-preview` | 2025-12-17 | Flash급 속도, free tier |
| `gemini-3-pro-preview` | 2025-11-18 | Gemini 3 첫 Pro |
| `gemini-2.5-flash` | 2025-06 | 이전 세대 |
| `gemini-2.5-flash-lite` | 2025-07 | ❌ 더 이상 사용 안 함 |

**thinkingConfig:**
- `thinkingBudget: -1` = dynamic (모델이 프롬프트 복잡도에 따라 자동 결정)
- `0` = thinking 비활성화, 양수 = 고정 토큰 한도
- Gemini 2.5 시리즈부터 thinking 지원 (2.0 시리즈는 미지원)

**모델 변경 시 수정할 파일:**
- `server/config/config.go` → `GeminiModel` 기본값
- 또는 `.env`에 `GEMINI_MODEL=<model-id>` 설정으로 오버라이드

**API 엔드포인트:** `https://generativelanguage.googleapis.com/v1beta/models/`
- Gemini 3 시리즈도 동일한 v1beta 엔드포인트 사용

### Data Collection Pipeline Improvement (2026-02-20)

#### 문제점 및 해결 방안

**기존 문제:**
1. 단일 프롬프트 방식 → AI가 주제를 직접 생성 → 할루시네이션 발생
2. 예시 문구를 AI가 그대로 복사하는 편향

**개선 방안: 2단계 파이프라인**

| 단계 | 역할 | 프롬프트 함수 |
|---|---|---|
| Stage 1 | 실제 웹 검색으로 트렌딩 키워드 추출 (15~20개) | `BuildKeywordExtractionPrompt()` |
| Stage 2 | Stage 1 키워드를 앵커로 삼아 심화 수집 | `BuildEnrichmentPrompt(keywords)` |

**핵심 원리:** Stage 1이 실제 검색으로 실존하는 키워드를 확정하므로, Stage 2는 그 키워드 범위 안에서만 작성 → 할루시네이션 차단

**관련 파일:**
- `server/domain/collector/prompt.go` — 2단계 프롬프트 함수
- `server/domain/collector/service.go` — 파이프라인 오케스트레이션
- `server/platform/gemini/client.go` — Gemini 클라이언트 (thinkingConfig 포함)