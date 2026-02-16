# 카카오 로그인 기능 구현 계획

## Context

프로젝트(Over the Algorithm)가 스켈레톤 상태이므로, 카카오 로그인을 구현하기 위해 Go/Gin 백엔드와 React 프론트엔드의 기본 인프라도 함께 구축해야 합니다. 카카오 OAuth2 인가 코드 플로우를 구현하고, JWT로 세션을 관리하며, 간단한 로그인 페이지를 만듭니다.

---

## 현재 프로젝트 상태 (스켈레톤)

### 존재하는 파일들
- `server/main.go` - 빈 main 함수
- `server/go.mod` - `module ota`, go 1.23.1
- `web/package.json` - 빈 npm 프로젝트 (Vite 미설정)
- `docker-compose.yml` - PostgreSQL 16 (ota/ota_dev_password/ota)
- `server/.env.example` - 환경변수 템플릿 (DB, Kakao, JWT, Server 설정 포함)

---

## 주요 설계 결정

| 결정 | 선택 | 이유 |
|------|------|------|
| DB 드라이버 | pgx/v5 + raw SQL | 쿼리가 단순(upsert, find), ORM 불필요 |
| 마이그레이션 | golang-migrate | 버전 관리, 롤백 가능 |
| JWT 저장 | httpOnly 쿠키 | XSS 방지 |
| CSRF 방지 | state 파라미터 (인메모리) | 프로토타입에 적절 |
| 프론트엔드 | Vite + React + Tailwind + shadcn/ui | 프로젝트 스펙 |
| Dev 프록시 | Vite proxy → localhost:8080 | 쿠키가 same-origin으로 동작 |

---

## Phase 1: 백엔드 인프라

### 1-1. Go 의존성 설치 및 프로젝트 구조 생성

```
server/
├── main.go
├── go.mod / go.sum
├── .env.example (기존)
├── migrations/
│   ├── 000001_create_users_table.up.sql
│   └── 000001_create_users_table.down.sql
└── internal/
    ├── config/
    │   └── config.go          # 환경변수 로드, Config 구조체
    ├── database/
    │   └── postgres.go        # DB 연결 풀 생성
    ├── server/
    │   ├── server.go          # Gin 엔진 생성, 라우트 등록
    │   └── middleware.go      # CORS, 인증 미들웨어
    ├── auth/
    │   ├── handler.go         # /auth/kakao/login, /auth/kakao/callback, /auth/me, /auth/logout
    │   ├── kakao.go           # Kakao API 클라이언트 (토큰 교환, 유저 정보)
    │   ├── jwt.go             # JWT 생성/검증
    │   └── state_store.go     # CSRF state 인메모리 저장소
    └── user/
        ├── model.go           # User 구조체
        └── repository.go      # UserRepository 인터페이스 + PostgreSQL 구현
```

**패키지:**
- `github.com/gin-gonic/gin`
- `github.com/gin-contrib/cors`
- `github.com/jackc/pgx/v5`
- `github.com/golang-jwt/jwt/v5`
- `github.com/golang-migrate/migrate/v4`
- `github.com/joho/godotenv`

### 1-2. DB 마이그레이션 (users 테이블)

```sql
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    kakao_id BIGINT UNIQUE NOT NULL,
    email VARCHAR(255),
    nickname VARCHAR(100),
    profile_image TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_users_kakao_id ON users(kakao_id);
```

### 1-3. 핵심 모듈 구현 순서

1. `config/config.go` - 환경변수 로드
2. `database/postgres.go` - DB 연결
3. `user/model.go` + `user/repository.go` - 유저 모델/저장소
4. `auth/kakao.go` - 카카오 API 클라이언트
5. `auth/jwt.go` - JWT 생성/검증
6. `auth/state_store.go` - CSRF state 관리
7. `auth/handler.go` - HTTP 핸들러
8. `server/middleware.go` - CORS, Auth 미들웨어
9. `server/server.go` - 라우트 등록
10. `main.go` - 엔트리포인트

### 1-4. API 엔드포인트

| Method | Path | 설명 |
|--------|------|------|
| GET | `/api/v1/auth/kakao/login` | 카카오 인가 URL로 리다이렉트 |
| GET | `/api/v1/auth/kakao/callback` | 콜백 처리, JWT 쿠키 설정 후 프론트엔드로 리다이렉트 |
| GET | `/api/v1/auth/me` | 현재 로그인 유저 정보 반환 (인증 필수) |
| POST | `/api/v1/auth/logout` | 쿠키 삭제 |

### 1-5. 카카오 OAuth 플로우

```
[사용자] → GET /auth/kakao/login
         → 302 Redirect to https://kauth.kakao.com/oauth/authorize
         → 카카오 로그인 화면에서 인증
         → 302 Redirect to /auth/kakao/callback?code=xxx&state=yyy
         → 백엔드: code로 access_token 교환
         → 백엔드: access_token으로 유저 정보 조회
         → 백엔드: DB에 유저 upsert
         → 백엔드: JWT 생성, httpOnly 쿠키에 설정
         → 302 Redirect to FRONTEND_URL (로그인 완료)
```

---

## Phase 2: 프론트엔드

### 2-1. Vite + React + TypeScript 프로젝트 초기화

```bash
cd web && npm create vite@latest . -- --template react-ts
npm install
npx tailwindcss init -p  # Tailwind 설정
npx shadcn@latest init   # shadcn/ui 초기화
```

### 2-2. 프로젝트 구조

```
web/src/
├── main.tsx
├── App.tsx
├── index.css             # Tailwind imports
├── lib/
│   └── api.ts            # API 호출 유틸리티
├── contexts/
│   └── auth-context.tsx  # 인증 상태 관리 (Context API)
├── pages/
│   ├── login.tsx         # 로그인 페이지 (카카오 버튼)
│   └── home.tsx          # 로그인 후 홈 페이지
└── components/
    └── kakao-login-button.tsx
```

### 2-3. 로그인 페이지

- 카카오 공식 로그인 버튼 디자인 (노란색 배경, 카카오 로고)
- 클릭 시 `GET /api/v1/auth/kakao/login`으로 이동
- 로그인 후 홈 페이지에서 유저 정보 표시 + 로그아웃 버튼

### 2-4. Vite 프록시 설정

```typescript
// vite.config.ts
server: {
  proxy: {
    '/api': 'http://localhost:8080'
  }
}
```

---

## Phase 3: 통합 테스트 및 검증

1. Docker로 PostgreSQL 실행: `docker compose up -d`
2. 서버 실행: `cd server && go run main.go`
3. 프론트엔드 실행: `cd web && npm run dev`
4. 브라우저에서 `http://localhost:5173` 접속
5. 카카오 로그인 버튼 클릭 → 로그인 플로우 확인

---

## 사용자가 설정해야 할 항목

### 1. 카카오 개발자 설정 (https://developers.kakao.com)

1. **애플리케이션 생성**: [내 애플리케이션] → [애플리케이션 추가하기]
2. **REST API 키 확인**: [앱 설정] → [앱 키] → REST API 키 → `KAKAO_CLIENT_ID`
3. **Client Secret 생성** (권장): [보안] → [Client Secret] → 코드 생성 → `KAKAO_CLIENT_SECRET`
4. **Redirect URI 등록**: [카카오 로그인] → [Redirect URI] → `http://localhost:8080/api/v1/auth/kakao/callback`
5. **카카오 로그인 활성화**: [카카오 로그인] → 활성화 설정 ON
6. **동의 항목 설정**: 닉네임, 프로필 사진 (필수), 이메일 (선택)

### 2. 환경변수

```bash
cp server/.env.example server/.env
# KAKAO_CLIENT_ID, KAKAO_CLIENT_SECRET, JWT_SECRET 수정
```

### 3. Docker

```bash
docker compose up -d
```
