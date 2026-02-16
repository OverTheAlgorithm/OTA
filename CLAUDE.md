# Over the Algorithm

## Overview

### Tools

1. Language: Go, TypeScript
2. Frameworks: Gin, React
3. DB: PostgreSQL(via Docker)

### **Description**

이 프로젝트는 유저에게 최근의 사회적 "맥락"을 제공하는 서비스 입니다.
프로젝트의 이름 Over the Algorithm은 유튜브, 틱톡 등의 개인화 된 알고리즘을 넘어, 개인화 된 맥락에 갇히지 않고 사회에서 일어나는 가장 뜨거운 맥락을 제공하는 것을 의미합니다.

예를 들어, 많은 남성들은 연애 프로그램에 관심이 없지만 사회에서 가장 화제인 주제일 수 있습니다. 그리고 그 직장에서 동료 여직원들이 연애 프로그램 대화를 나눌 때 참여하고 싶을 수 있습니다.
이 때, 모든 프로그램을 시청하는 것은 불가능하지만, 가장 뜨거운 주제를 일목요연하게 정리해서 화제에 참여할 수 있는 맥락은 제공할 수 있습니다.

### Features

#### Core

1. 유저들에게 가장 최근의 맥락을 요약해서 제공합니다.

* 맥락은 **매일 아침 7시**에 제공됩니다. 현재는 프로토타입이므로, 한국 시각으로만 제공합니다. 다만, 확장성을 위해 시간과 관련된 정보는 항상 UTC로 처리합니다.
* 맥락의 주제는 정해져있지 않습니다. 분야를 막론하고 **가장 많이 화제로 선정되고 있는 주제**를 선정합니다. 분야를 막론한 맥락이 항상 최상위 맥락으로 제공 됩니다.
* 맥락이 제공되는 경로는 **카카오톡** 과 **이메일** 입니다.
* 유저가 특정 주제를 구독한 경우, 그 주제에 대한 맥락이 요약되어 하위 주제로 덧붙여져 제공 됩니다.
* 맥락은 단순히 "정치", "경제" 등이 아닙니다. 올바른 형태의 예는, "환승 연애의 시즌 3 출연자가 전 남자친구를 두명이나 데리고 출연했으며 둘을 모두 좋아하고 있다." 등과 같이 구체적인 맥락 및 주제입니다.
* 요약은 짧고 명료해야 합니다. 맥락 메세지 하나에 여러 주제가 모두 표현되어야 합니다. 각 주제에 대한 요약은 한 문장으로 표현되어야 하며, 어쩔 수 없는 경우에만 최대 2문장으로 표현되어야 합니다.

2. 맥락을 수집하고 분석합니다.

* 맥락을 수집하는 방법은 트위터, RSS 서비스 등 사람들이 어떤 주제로 대화하고 있는지 실시간으로 파악할 수 있는 곳들입니다.
  * 현재는 프로토타입이므로, 단순한 수집만 구현합니다. 방법은 웹 스크래핑입니다.
  * 단순히 웹페이지 전체를 통째로 넘기는 것은 엄청난 토큰 소모와 더불어 AI의 정보 분석을 힘들게 할 것입니다. HTML은 구조화 된 데이터이므로, 불필요한 부분을 알고리즘적으로 제거한 후 적절한 부분만 넘기도록 합니다.
    * 데이터 정제 과정은 각 웹 사이트별 데이터 형태에 따라 다를 수 있습니다. 최대한 공통적인 정제 과정을 지향해야하며, 정보 수집 소스가 늘어날 경우 구체적으로 데이터 정제 과정을 수정합니다.
* 수집된 데이터를 OpenAI의 API를 통해 정리 및 요약합니다. 이 작업은 빠른 응답속도를 목표로 진행 될 필요가 없고 정확한 분석이 필요하므로 deep analysis가 적절합니다. 요약된 정보는 매일 아침 7시 전까지만 준비되면 되기 때문입니다.
* **중요**: 데이터를 수집하는 과정은 웹 스크래핑에 얽매일 필요는 없습니다. 예를 들어, OpenAI Agent에게 현재 가장 뜨거운 주제들을 수집하라고 지시했을 때, 결과물이 적절한 수준의 품질을 보장한다면 이 작업은 간단화 할 수 있습니다.
  * 이 과정은 여러차례의 실험이 필요합니다. 예를 들어, 단순히 요청을 하는 것보다 특정 웹사이트를 꼭 참조할 레퍼런스로 제공했을 때 더 결과물이 좋을 수 있습니다. 또 다른 예로는, 키워드 추출과 그 키워드에 해당하는 내용 수집을 각각 다른 출처에서 진행하는 것이 효율적일 수도 있습니다.

#### Others

위의 Core features를 구동시키기 위한 모든 기능이 포함 됩니다. 아래에 명시되어있지 않더라도 필요한 기능을 구현해야 하며, 기능 추가가 필요할 땐 사용자에게 먼저 보고 한 후, 구현 세부 사항이 결정된 후 이 문서에 기능 명세를 추가해야합니다.

1. 유저 관련 기능

   * 회원가입
    * Google을 통한 회원가입과 자체 회원가입을 제공합니다. Google 회원가입은 우선순위가 아니므로, 자체 회원가입부터 진행해야 합니다.
    * 이메일과 모바일 번호를 수집해야 합니다. 메세지를 제공하기 위함입니다.
    * 이 서비스는 다른 사용자와의 소통을 제공하지 않으므로, 닉네임 등 유저가 자신을 표현하기 위한 수단은 필요하지 않습니다.
   * 로그인
    * Google을 통한 로그인과 자체 로그인을 제공합니다.
   * 로그아웃
   * 회원탈퇴

2. 메세지 관련 기능
   * **맥락 제공받기 토글**. 이 토글은 특정 주제에 대한 것이 아니고, 메세지 자체를 수령할지를 결정하는 것입니다.
   * **특정 주제 구독 토글**. 메세지 자체를 수령하는 것과 관계 없이, 특정 주제에 대한 하위 맥락을 의미합니다.
   * 최상위 맥락은 특정 주제로 취급되지 않으며, 맥락을 제공받는 유저는 어떤 주제를 구독 했는지에 관계없이 항상 최상위 맥락을 제공 받습니다.

### Monorepo

1. 백엔드와 프론트엔드는 같은 저장소를 사용합니다. 이 프로젝트에서도 함께 구현되어야 합니다.

---

## Implementation Progress

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

#### Next Steps

1. **collector.Repository 구현체**: PostgresRepository (pgx). `user.PostgresRepository` 패턴 따름.
2. **config에 OpenAI 설정 추가**: `OPENAI_API_KEY`, `OPENAI_MODEL` 환경변수.
3. **main.go에 collector 와이어링**: ai.Client → Repository → Service 초기화.
4. **실제 테스트**: OpenAI API 키로 `Collect(ctx)` 실행하여 한국 트렌딩 토픽 품질 확인.
5. **프롬프트 튜닝**: 실제 결과물 기반으로 프롬프트 개선.
6. **스케줄러 통합**: cron 또는 scheduler에서 `Collect(ctx)` 호출.
7. **HTTP handler**: 수동 트리거용 admin endpoint 추가.