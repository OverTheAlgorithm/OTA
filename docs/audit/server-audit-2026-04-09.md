# WizLetter 서버 감사 보고서

- **작성일**: 2026-04-09
- **감사 범위**: `server/` 전체 (Go 1.25 + Gin + pgx + PostgreSQL 16)
- **감사 방식**: 3개 analyst 에이전트 병렬 실행 (보안 / 아키텍처 / API 계약)
- **관점**: CTO 레벨 종합 리뷰 (best practice 위반 / 보안 취약성 / 계층 불일치 / 비효율 / 과도한 복잡성)

이 문서는 다른 세션에서 이어받아 수정 작업을 진행할 수 있도록 모든 findings를 파일:라인 단위로 정리한 원본 감사 결과입니다. 심각도 순으로 정렬되어 있고, 각 항목에 fix 방향이 명시되어 있습니다.

### 검증 결과 (2026-04-23)

소스코드 라인 단위 대조 검증 완료. 38건 검증 중 **28건 TRUE, 6건 PARTIALLY TRUE, 4건 FALSE**.

| 오판 항목 | 원래 등급 | 판정 | 사유 |
|-----------|-----------|------|------|
| C5 | CRITICAL | **FALSE** | `ON CONFLICT DO UPDATE`에서 email 미갱신 (nickname/profile_image만) |
| C8 | CRITICAL | **FALSE** | `CanRunToday`가 `status='running'`도 체크함 |
| C10 | CRITICAL | **PARTIALLY TRUE** | 멱등성 키 없음은 사실이나 `FOR UPDATE` 있음 (`withdrawal_repository.go:90`) |
| H7 | HIGH | **PARTIALLY TRUE** | UUID 검증 없음은 사실이나 Go stdlib가 path traversal 차단 |
| H9 | HIGH | **PARTIALLY TRUE** | 4/5곳 사실, `scheduler.collect()`만 `shutdownCtx` 사용 |
| H10 | HIGH | **PARTIALLY TRUE** | admin.go/auth.go 고루틴은 맞음, cron 라이브러리가 scheduler에 recovery 제공 |
| H13 | HIGH | **FALSE** | 실제 delivery 9:30AM (7AM 아님), 3.5시간 버퍼 존재 |
| M2 | MEDIUM | **FALSE** | 실제로 404 반환 (500 아님) |
| M8 | MEDIUM | **PARTIALLY TRUE** | 기본 테이블 인덱스 존재, UNION VIEW 한계는 맞음 |
| M10 | MEDIUM | **PARTIALLY TRUE** | 실제 568줄 (549 아님), 어댑터 인라인은 사실 |

---

## 목차
- [CRITICAL (10건)](#critical--즉시-수정돈데이터보안-직결)
- [HIGH (18건)](#high--다음-스프린트)
- [MEDIUM (20건)](#medium--다다음-스프린트)
- [LOW (선택 정리)](#low-선택적-정리)
- [우선순위 제안](#우선순위-제안)
- [테스트 커버리지 공백](#테스트-커버리지-공백)

---

## CRITICAL — 즉시 수정(돈/데이터/보안 직결)

### C1. `SetTrustedProxies` 미설정 → Rate limit 완전 우회 [FIXED 2026-04-23]
- **파일**: `server/api/router.go:20`, `server/api/middleware.go:268-283`
- **문제**: Gin은 기본적으로 모든 프록시를 신뢰 → `c.ClientIP()`가 `X-Forwarded-For` 첫값을 그대로 사용.
- **공격 시나리오**: `X-Forwarded-For: <랜덤IP>` 헤더 조작만으로 모든 레이트 리밋 무력화. Turnstile의 `remoteip` 검증도 무력화.
- **해결**:
  - 감사 문서 제안(`SetTrustedProxies(["127.0.0.1"])`)은 Docker Compose 환경에서 부적절 — Caddy 컨테이너 IP가 동적이라 관리 부담이 큼.
  - 대신 **TrustedPlatform + 커스텀 헤더** 방식 채택:
    1. `Caddyfile`: `header_up X-Real-Client-IP {remote_host}` 추가. Caddy가 실제 클라이언트 IP를 커스텀 헤더에 **덮어쓰기**(append 아닌 overwrite)로 설정. 공격자가 같은 이름의 헤더를 보내도 Caddy가 실제 IP로 교체.
    2. `server/api/router.go`: `r.SetTrustedProxies(nil)` (X-Forwarded-For 완전 무시) + `r.TrustedPlatform = "X-Real-Client-IP"` (커스텀 헤더에서 IP 읽기).
  - `c.ClientIP()` 호출 시 X-Real-Client-IP 헤더 값을 사용 → rate limit, Turnstile remoteip 모두 실제 클라이언트 IP 기반으로 동작.
  - **외부 동작 변화 없음**: 정상 사용자는 기존과 동일하게 동작. X-Forwarded-For 조작을 통한 rate limit 우회만 차단됨.
  - **배포 시 주의**: Caddy 재시작 필요 (Caddyfile 변경 반영). `docker compose up --build -d`로 자동 반영됨.

### C2. CSRF 미들웨어 우회 가능 [PARTIALLY FIXED 2026-04-23]
- **파일**: `server/api/middleware.go:200-228`, `server/api/handler/auth.go:156,180,195,204`
- **문제**: Origin **과** Referer **둘 다** 없으면 "non-browser client"로 간주하고 통과. 쿠키는 `SameSite=None`.
- **공격 시나리오**: `Referrer-Policy: no-referrer` 헤더를 단 악성 페이지에서 `/auth/delete-account`나 `/withdrawal/request`로 크로스 오리진 POST. 쿠키 자동 첨부, CSRF 통과.
- **해결 (1단계 — SameSite=Lax)**:
  - `auth.go`의 모든 쿠키 설정(setAuthCookies, clearAuthCookies) 4곳에서 `SameSite=None` → `SameSite=Lax`로 변경.
  - **근거**: 프론트엔드(`wizletter.mindhacker.club`)의 모든 API 호출은 Vercel rewrite(`vercel.json`)를 통해 same-origin으로 프록시됨. 브라우저는 `server.mindhacker.club`에 직접 통신하지 않음. 쿠키는 `wizletter.mindhacker.club`에 설정되므로 `SameSite=Lax`로 충분.
  - `SameSite=Lax` 효과: 외부 사이트의 POST/DELETE 요청에 쿠키 미첨부 → CSRF의 핵심 공격 벡터 차단.
  - **외부 동작 변화 없음**: 이메일/카카오톡 링크 클릭(외부 GET)은 쿠키 전송됨. 사이트 내 모든 기능 정상.
- **미해결 (2단계 — 선택)**: Origin+Referer 둘 다 없는 경우의 미들웨어 통과 로직. Lax로 쿠키 자체가 안 붙으므로 실질 위험은 제거됨. 추후 미들웨어 정리 시 함께 처리 가능.

### C3. SSRF — Collector article fetcher에 URL 허용리스트 없음
- **파일**: `server/domain/collector/article_fetcher.go:56-85`, `server/domain/collector/source_validator.go:158-201`
- **문제**: AI가 뱉는 URL에 `http.Get` 무제한. RFC1918/loopback/link-local 거름망 없음. 리다이렉트 홉별 재검증 없음.
- **공격 시나리오**: 프롬프트 인젝션 또는 소스 포이즈닝으로 `http://169.254.169.254/latest/meta-data/iam/security-credentials/` (Oracle Cloud IMDS) 호출 → DB/로그에 크리덴셜 유출. `http://localhost:5432` 같은 내부 포트 스캔도 가능.
- **Fix**: `net.LookupIP` 후 `IsPrivate/IsLoopback/IsLinkLocal/IsUnspecified` 거부. custom `DialContext`로 리다이렉트 홉마다 재검증. scheme은 `http/https`만 허용.

### C4. Turnstile 프로덕션 fallback이 테스트 키 → CAPTCHA 완전 스킵 [FIXED 2026-04-23]
- **파일**: `server/config/config.go:113`, `server/api/handler/level_handler.go:479-488`
- **문제**: `TURNSTILE_SECRET_KEY` 기본값이 Cloudflare 테스트 키(`1x0000000000000000000000000000000AA`). `isTurnstileTestKey()`가 감지하면 검증 자체를 skip.
- **시나리오**: 프로덕션 env 누락 시 5-layer anti-cheat의 L3가 조용히 무력화. 봇이 CAPTCHA 없이 코인 적립.
- **해결**:
  - `server/config/config.go:175-179`에 프로덕션 가드 추가.
  - `AppEnv=production`일 때 `TURNSTILE_SECRET_KEY`가 빈 값, `"dummy-secret-key"`, 또는 `"1x000000000000000000000000000000"` 접두사이면 startup 실패.
  - 개발 환경(`AppEnv=development`)은 기존대로 테스트 키 허용.
  - **외부 동작 변화 없음**: 프로덕션에 이미 정상 키가 설정되어 있으면 영향 0. 키 미설정 시 서버 시작 자체를 거부하여 잘못된 배포 사전 차단.

### ~~C5. Kakao 로그인 시 기존 유저의 email을 OAuth 응답으로 덮어쓰기~~ [FALSE]
- **파일**: `server/api/handler/auth.go:248-277`, `server/storage/user_repo.go:22-29`
- **원래 주장**: 기존 유저 경로에서 `UpsertByKakaoID`가 Kakao가 준 `Account.Email`로 DB users 레코드를 덮어씀.
- **검증 결과**: **거짓**. `user_repo.go:22-29`의 SQL `ON CONFLICT (kakao_id) DO UPDATE SET`에서 `nickname`, `profile_image`, `updated_at`만 갱신. email 컬럼은 업데이트 대상에 포함되지 않음. 이미 의도대로 구현되어 있음.

### C6. CompleteSignup이 트랜잭션이 아님 → 반쪽 가입 상태 발생
- **파일**: `server/api/handler/auth.go:340-375`
- **문제**: `UpsertByKakaoID` → `SaveConsents` → `AddPoints` → `InsertCoinEvent` 네 번의 쓰기가 트랜잭션 밖에서 순차 실행.
- **결과**: 2단계 실패 시 약관 미동의 유저가 DB에 남음(개인정보보호법/GDPR 위반). 3-4단계 실패 시 `user_points`와 `coin_events` 영구 드리프트. `InsertCoinEvent` 에러는 `_ =`로 아예 discard.
- **Fix**: `UserService.CompleteSignup`이 단일 pgx 트랜잭션을 열고 repos에 `WithTx(tx pgx.Tx)` 변형을 넘기도록. Atomic commit.

### C7. `EarnCoin` 일일한도/COIN_CAP 검증이 트랜잭션 밖 → 경쟁 조건으로 한도 초과
- **파일**: `server/domain/level/service.go:106-163`, `server/storage/level_repository.go:40-97`
- **문제**: `GetTodayEarnedCoins` + cap 검증이 tx 밖에서 일어난 후 별도 tx로 `EarnCoin` 수행. `coin_logs`의 UNIQUE(user_id, run_id, context_item_id)는 **같은** 토픽만 막음.
- **공격 시나리오**: 두 브라우저 탭에서 **서로 다른** 토픽에 대해 동시에 `/level/earn` 호출 → 둘 다 검증 통과 → 일일 한도 2배 적립. `COIN_CAP=5000`도 깨짐.
- **영향**: `MIN_WITHDRAWAL_AMOUNT=3000` 실제 출금 시스템과 연결된 돈 문제.
- **Fix**: cap + daily-limit 체크를 `LevelRepository.EarnCoin` 트랜잭션 안으로 이동. `SELECT ... FOR UPDATE` on `user_points`. 또는 `UPDATE ... WHERE today_earned + $1 <= limit` 조건부 업데이트.

### ~~C8. Collector 스케줄러 오버랩 — 이전 run이 돌고있는데 다음 cron 시작~~ [FALSE]
- **파일**: `server/scheduler/scheduler.go:43-48, 75-89`, `server/storage/collector_repo.go:103-118`
- **원래 주장**: `CanRunToday`가 `status='running'`을 보지 않아 병렬 파이프라인 시작 가능.
- **검증 결과**: **거짓**. `collector_repo.go:103-118`의 `CanRunToday` 쿼리가 `WHERE ... AND (status = 'running' OR status = 'success')`로 running 상태도 체크함. 또한 `checkpoint_repo.go:77-92`의 `CreateRunIfIdle`도 `INSERT ... WHERE NOT EXISTS (... status = 'running')`으로 원자적 보호. `FailStaleRuns`는 boot 시 실행(비정상 종료 대응)으로 적절.

### C9. 관리자 `/admin/collect` 중복 실행 가드 없음 + graceful shutdown 붕괴
- **파일**: `server/api/handler/admin.go:59-85`
- **문제**: 백그라운드 goroutine을 `context.Background()`로 detached하게 돌림. dedup 키 없음. 202만 반환.
- **결과**: 두 번 클릭하면 동시 collection 2개 돌아감. 배포 중 SIGTERM 시 좀비 goroutine 발생.
- **Fix**: `sync.Mutex + running bool` 또는 Redis `collect:running` 키로 가드. 409 반환. `shutdownCtx` 주입 + WaitGroup.

### C10. 출금 요청 멱등성 없음 ~~+ row-level lock 없음~~ [PARTIALLY TRUE]
- **파일**: `server/api/handler/withdrawal_handler.go:71-101`, `server/domain/withdrawal/service.go:149-152`, `server/storage/withdrawal_repository.go:80-142`
- **멱등성 키 미지원**: **사실**. `Idempotency-Key` 헤더 미지원. 네트워크 재시도 시 중복 출금 요청 생성 가능.
- **~~row-level lock 없음~~**: **거짓**. `withdrawal_repository.go:90`에서 `SELECT points FROM user_points WHERE user_id = $1 FOR UPDATE` 사용. 단일 트랜잭션 내 잔액 체크 + 차감 + 출금 생성 원자적 수행. 잔액 이중 차감(double-spend)은 방지됨.
- **실제 위험**: 멱등성 키 부재로 중복 pending 레코드 생성 가능하나, 재정적 무결성(잔액)은 `FOR UPDATE`로 보호됨. 심각도 하향 권장 (CRITICAL → HIGH).
- **Fix**: `Idempotency-Key` 헤더 + `withdrawal_idempotency` 테이블(TTL).

---

## HIGH — 다음 스프린트

### 보안

#### H1. Refresh token reuse detection 없음
- **파일**: `server/api/handler/auth.go:419-465`, `server/storage/refresh_token_repo.go`
- **문제**: 도난된 refresh token으로 끝없이 rotation 가능. 현재는 피해자가 재사용 시도할 때만 에러.
- **Fix**: `family_id` 컬럼 추가. 삭제된 hash의 재사용 감지 시 `DeleteAllForUser(userID)` + 보안 이벤트 로그.

#### H2. Logout/DeleteAccount가 access token 무효화 안 함
- **파일**: `server/api/handler/auth.go:467-516`
- **문제**: Logout/DeleteAccount가 refresh token만 삭제. 기존 access token은 최대 15분간 유효 → 세션 하이잭 생존.
- **Fix**: JTI denylist (Redis SET TTL=15min) 또는 `users.tokens_valid_since` 컬럼. 삭제된 유저에 500 반환 대신 401 반환 (H12 참조).

#### H3. JWT_SECRET 최소 길이 검증 없음 [FIXED 2026-04-23]
- **파일**: `server/config/config.go:100, 155-157`
- **문제**: `"dev"` 3글자도 허용됨. HS256 + 3글자 시크릿은 오프라인 brute-force 가능.
- **해결**:
  - `server/config/config.go:159-161`에 프로덕션 가드 추가.
  - `AppEnv=production`일 때 `len(JWTSecret) < 32`이면 startup 실패. 에러 메시지에 실제 길이 표시 (값은 노출하지 않음).
  - 개발 환경은 기존대로 짧은 시크릿 허용.
  - **외부 동작 변화 없음**: 프로덕션에 32자 이상 시크릿이 설정되어 있으면 영향 0.

#### H4. Withdrawal approval에 `FOR UPDATE` 없음
- **파일**: `server/domain/withdrawal/service.go:192-207`
- **문제**: 관리자 동시 승인 시 approved 트랜지션 2개 생성 → 다운스트림 payout 잡이 2번 송금할 수 있음.
- **Fix**: 트랜잭션에 `SELECT ... FROM withdrawals WHERE id=$1 FOR UPDATE` + latest transition FOR UPDATE 후 insert.

#### H5. 어드민 `NewCoins`에 상한 없음
- **파일**: `server/api/handler/admin_coin_handler.go:75-78`
- **문제**: `min=0`만 있음. 악의적/실수 관리자가 `new_coins=2147483647` 설정 가능. `SetCoins`는 `COIN_CAP` 미적용.
- **Fix**: `max=<COIN_CAP>` 검증.

#### H6. CORS 서브도메인 와일드카드 + credentials
- **파일**: `server/api/middleware.go:104-117`
- **문제**: `mindhacker.club`의 어떤 서브도메인이든 허용 + `AllowCredentials=true`. dangling DNS 기반 subdomain takeover 시 full credentialed CORS.
- **Fix**: 명시적 호스트 allowlist (`wizletter.mindhacker.club`만).

#### H7. `/api/v1/images` static 서빙 검증 부족 [PARTIALLY TRUE]
- **파일**: `server/main.go:535`
- **UUID 검증 없음**: **사실**. `r.Static("/api/v1/images", imageBaseDir)`로 파일명 무검증 서빙.
- **~~심볼릭 링크/path traversal~~**: **과대평가**. Go stdlib `http.Dir`가 `..` path traversal 자동 차단. 심볼릭 링크는 `data/images/` 내부에 존재할 때만 위험 (외부 생성 불가).
- **Fix**: `image_path`를 `^[a-f0-9-]{36}\.(png|jpg|webp)$`로 검증 후 DB 저장. `X-Content-Type-Options: nosniff` 추가.

#### H8. PII 로깅
- **파일**: `server/api/handler/auth.go:348-354, 397`
- **문제**: email/nickname/kakao_id 원본을 INFO 로그에 기록 → `logs/ota.log` 30일 보관 → 개인정보보호법 위반.
- **Fix**: email은 SHA-256 해시, nickname은 redact.

### 아키텍처

#### H9. ~~모든~~ 대부분의 백그라운드 goroutine이 `context.Background()` 사용 [PARTIALLY TRUE]
- **파일**: `server/api/handler/auth.go:379` (welcome email), `server/api/handler/admin.go:64` (collect), `server/scheduler/scheduler.go:93, 105` (deliver/retry)
- **사실인 부분**: auth.go:379, admin.go:64, scheduler.go:93(deliver), scheduler.go:105(retryFailed) — 4곳에서 `context.Background()` 사용.
- **거짓인 부분**: `scheduler.go:77`의 `collect()`는 `s.shutdownCtx` 사용. "모든"이라는 표현은 부정확.
- **Fix**: 나머지 4곳에 `shutdownCtx` 주입 + `sync.WaitGroup`으로 shutdown 시 drain.

#### H10. 백그라운드 goroutine에 panic recovery 없음 [PARTIALLY TRUE]
- **파일**: `server/api/handler/admin.go:63-85`, `server/api/handler/auth.go:378-384`
- **사실인 부분**: `admin.go:63`(manual collect)과 `auth.go:378`(welcome email) 고루틴에 `recover()` 없음. panic 시 프로세스 크래시.
- **거짓인 부분**: `scheduler.go:75-89`의 cron 작업은 `robfig/cron` 라이브러리가 `Recover` job wrapper로 recovery 제공. 스케줄러 panic은 프로세스를 죽이지 않음.
- **Fix**: `admin.go:63`, `auth.go:378`의 `go func()` body에 `defer func(){ if r := recover(); r != nil { slog.Error("panic", "r", r, "stack", debug.Stack()) } }()` 래퍼.

#### H11. Gemini 클라이언트 `Timeout: 0` + 서킷 브레이커 없음
- **파일**: `server/platform/gemini/client.go:35-40`
- **문제**: Gemini 장애 시 4AM/5AM/6AM 모두 동일하게 1시간씩 걸려 실패. 좀비 TCP 연결 누적.
- **Fix**: `httpClient.Timeout = 5 * time.Minute`. `sony/gobreaker` 서킷 브레이커 적용. `callAIWithRetry` (`service.go:719`) 확장.

#### H12. Delivery dead-letter 경로 없음
- **파일**: `server/domain/delivery/service.go:156-229`, `server/storage/delivery_repository.go:221`
- **문제**: `retry_count` 초과한 유저는 조용히 failed 상태로 방치. 관찰 불가.
- **Fix**: "abandoned" 상태 추가 + Slack 알림. SMTP 백프레셔 처리.

#### ~~H13. 6AM collect가 아직 running인데 7AM delivery 시작 → 어제 run으로 fallback 발송~~ [FALSE]
- **파일**: `server/scheduler/scheduler.go:42-60`
- **원래 주장**: 7AM delivery가 아직 running 중인 6AM collection과 충돌.
- **검증 결과**: **거짓**. 실제 스케줄: collection 4/5/6AM, delivery **9:30AM/9:45AM** (7AM 아님). 3.5시간 버퍼 존재. 감사 시점의 스케줄 오기.
- **참고**: 모든 collection이 실패할 경우 `GetLatestRun`이 이전 날 데이터를 반환하는 것은 사실이나, 이는 스케줄 충돌이 아닌 별도 설계 판단의 문제.

#### H14. `LevelRepository.EarnCoin`에 프로덕션 디버그 INFO 로그
- **파일**: `server/storage/level_repository.go:66-73`
- **문제**: Hot path에 `slog.Info("[EarnCoin] upsert params", ..., "coins_type", fmt.Sprintf("%T", coins))`. 모든 토픽 조회마다 로깅. 로그 예산 낭비 + 미해결 디버그 흔적.
- **Fix**: 삭제 또는 `slog.Debug`로 강등.

### API

#### H15. 응답 envelope 6가지 혼재
- **파일**: router/handler 전체
- **문제**: `{data}`, `{data, has_more}`, `{data, total}`, `{channels}`, flat (admin.go:166, 145), `{message}`가 혼재.
- **Fix**: 표준 envelope `{"data": T, "meta"?: {has_more, total, ...}, "error"?: string}`로 일괄 마이그레이션 (별도 PR).

#### H16. `/level/earn`이 EXPIRED 상태에서 200 OK 반환
- **파일**: `server/api/handler/level_handler.go:281`
- **문제**: 200으로 실패 전달 (`data: {attempted: true, reason: "EXPIRED"}`). 프론트가 `reason` 분기해야 함.
- **Fix**: `409 Conflict` + `reason: "EXPIRED"`. `DAILY_LIMIT`, `DUPLICATE`도 동일.

#### H17. Admin withdrawals가 `has_more` 대신 `total` 사용
- **파일**: `server/api/handler/withdrawal_admin_handler.go:50`
- **문제**: 다른 페이지네이션 엔드포인트는 `has_more`를 사용. 프론트엔드 `api.ts:537`이 `body.total`, `getWithdrawalHistory`는 `body.has_more` → 드리프트.
- **Fix**: 표준 `has_more`로 통일. `total`은 optional로 유지.

#### H18. `/auth/me` 응답에 `kakao_id` 노출
- **파일**: `server/domain/user/model.go:7` (`KakaoID int64`, `json:"kakao_id"`)
- **문제**: Kakao OAuth provider의 user ID는 프론트에서 쓰이지 않음. fingerprinting/cross-reference 리스크.
- **Fix**: `UserResponse` DTO를 handler 계층에 정의하고 `kakao_id` drop. TS 타입에서도 제거.

---

## MEDIUM — 다다음 스프린트

### 보안/운영

#### M1. Signup key가 URL 쿼리스트링으로 전달
- **파일**: `server/api/handler/auth.go:279-292`
- **문제**: `signup_key=<uuid>`가 Caddy access log, 브라우저 history, Referer에 3분간 노출.
- **Fix**: 단기 HttpOnly 쿠키로 전환.

#### ~~M2. `Me` 핸들러가 삭제된 유저에 500 반환~~ [FALSE]
- **파일**: `server/api/handler/auth.go:409-411`
- **원래 주장**: DB 레코드 없으면 500 반환.
- **검증 결과**: **거짓**. 실제로 `http.StatusNotFound` (404) 반환. 단, DB 인프라 에러(connection timeout 등)도 404로 반환하는 것은 별도 이슈 (500이어야 할 케이스를 404로 처리).

#### M3. `BANK_ACCOUNT_ENCRYPTION_KEY` 프로덕션 검증 없음
- **파일**: `server/config/config.go:145`, `server/storage/withdrawal_repository.go:47-51`
- **문제**: 빈 키면 계좌번호 평문 저장. 프로덕션 가드 없음.
- **Fix**: `AppEnv=production` + key 비어있으면 startup fail.

#### M4. RateLimit fail-open
- **파일**: `server/api/middleware.go:244-249`
- **문제**: Redis 장애 시 전체 통과. Redis DoS로 rate limit 무력화 가능.
- **Fix**: 최소한 admin/withdrawal 엔드포인트는 fail-closed. 또는 in-memory fallback.

#### M5. Email 템플릿 XSS 가능성
- **파일**: `server/platform/email/*`
- **문제**: grep 결과 `html/template` 사용 없음. 닉네임/email 보간 경로 검증 필요.
- **Fix**: 항상 `html/template`. user input은 escape.

#### M6. `withdrawal_handler.go:63`이 `err.Error()`를 그대로 클라이언트에 노출
- **파일**: `server/api/handler/withdrawal_handler.go:63`
- **문제**: 내부 에러 메시지(예: "encrypt account number: ...") leak.
- **Fix**: 상세 로그 + 제네릭 400 반환.

#### M7. SMTP in-function retry 없음
- **파일**: `server/platform/email/sender.go`
- **문제**: 일시 실패 시 바로 failed 마킹. 7:30/8:00/8:30 cron 의존.
- **Fix**: 3회 재시도 + exponential backoff (2-4-8s).

#### M8. `coin_history` 뷰 인덱스 확인 필요 [PARTIALLY TRUE]
- **파일**: migration 000024, 000016, 000020, 000017
- **원래 주장**: 각 테이블에 인덱스 없으면 full scan.
- **검증 결과**: 기본 테이블에 인덱스 **이미 존재** — `coin_logs`(`idx_coin_logs_user_created`, 000016), `coin_events`(`idx_coin_events_user_created`, 000020), `withdrawals`(`idx_withdrawals_user_id` + `idx_withdrawals_created_at`, 000017). 단, UNION ALL 뷰는 PostgreSQL이 단일 인덱스로 최적화 불가하므로 4개 브랜치 merge-sort는 구조적 한계.
- **Fix**: 현재 인덱스로 충분. 대규모 데이터 시 materialized view 검토.

### 아키텍처

#### M9. `collector/service.go` 880+ lines
- **파일**: `server/domain/collector/service.go`
- **문제**: 프로젝트 룰 `coding-style.md` 800 라인 제한 위반. 파이프라인 + AI retry + checkpoint + image + quiz 혼재.
- **Fix**: `pipeline.go` / `ai_retry.go` / `checkpoint.go` / `images.go` / `quizzes.go`로 분할.

#### M10. `main.go` ~~549~~ 568 라인 + adapters 인라인 [PARTIALLY TRUE]
- **파일**: `server/main.go:46-80` (adapters), 전체 (568줄)
- **문제**: DI god-file. `sitemapRepoAdapter`(46-60), `levelServiceAdapter`(62-80)가 main.go에 인라인 정의. 라인수는 549가 아닌 568줄이나 구조적 문제는 동일.
- **Fix**: `buildRouter` / `buildCollectorService` 헬퍼 추출. adapters는 `server/api/adapters/`로.

#### M11. `EarnCoin` 동시성 경쟁 테스트 없음
- **파일**: `server/domain/level/service_test.go`
- **문제**: C7 대응 검증을 위한 `-race` 테스트 없음.
- **Fix**: 테이블 기반 concurrent test + `sync.WaitGroup` + `-race` 옵션.

#### M12. 메트릭스 엔드포인트 없음
- **문제**: `/metrics` 없음. `last_successful_collection_at` 게이지 없음. 실패 감지는 `slog.Error`만.
- **Fix**: Prometheus `/metrics` + 주요 카운터/게이지.

#### M13. `fetchAndValidateSources`가 topic 단위로 직렬 실행
- **파일**: `server/domain/collector/service.go:555-606`
- **문제**: Phase 2는 병렬인데 Phase 3는 직렬. 수집 전체 지연의 주원인 → C8 트리거.
- **Fix**: `phase2Concurrency` 세마포어 패턴 재사용.

#### M14. `rows.Err()` 누락
- **파일**: `server/storage/level_repository.go:292` (OK), 여러 repo
- **문제**: `for rows.Next()` 후 체크 누락 시 네트워크 에러 조용히 truncate.
- **Fix**: 전체 repo 감사 후 일괄 추가.

#### M15. `collectorService.WithQuizRepo` 등 post-construction mutation
- **파일**: `server/main.go:335-337` vs `:233-241`
- **문제**: `With*` 메소드가 `*Service` 뮤테이션. 현재는 안전하지만 immutable 원칙 위반.
- **Fix**: Functional options 패턴으로 `NewService`에 전달.

### API

#### M16. `GetRecentTopics`/`GetLatestRunTopics`가 DB 에러 시 빈 배열 반환
- **파일**: `server/api/handler/context_history.go:107-110, 122-125`
- **문제**: 에러 swallow → outage invisible.
- **Fix**: 500 반환. 랜딩 페이지는 client-side fallback.

#### M17. `BatchEarnStatus` N+1
- **파일**: `server/api/handler/level_handler.go:386-393`
- **문제**: `IsRunCreatedToday`를 unique run ID 별로 루프 호출.
- **Fix**: `AreRunsCreatedToday(ctx, []uuid.UUID) (map[uuid.UUID]bool, error)` 배치 메소드 추가.

#### M18. 타입 기반 센티넬 대신 `strings.Contains`
- **파일**: `server/api/handler/admin_coin_handler.go:50, 100`, `server/api/handler/quiz_handler.go:75-80`
- **문제**: 메시지 변경 시 조용히 깨짐.
- **Fix**: `apperr.NotFoundError` 등 타입 정의 + `errors.Is`.

#### M19. `parsePageParams`가 잘못된 입력 silent clamp
- **파일**: `server/api/handler/helpers.go:18-32`
- **문제**: `?limit=9999`, `?limit=abc`, `?offset=-5` 전부 조용히 default로 fallback.
- **Fix**: 400 반환 또는 최소 warning 로그.

#### M20. Turnstile verify에 context 미전달
- **파일**: `server/api/handler/level_handler.go:501-509`
- **문제**: `http.NewRequest` 사용 → 클라이언트 disconnect 시에도 5s 블로킹.
- **Fix**: `http.NewRequestWithContext(c.Request.Context(), ...)`.

#### M21. Stage 6 (image gen) 파이프라인 꼬리 실행 검증 필요
- **파일**: `server/domain/collector/service.go:401-405` (`runPipelineTail`)
- **문제**: 동기 실행이면 delivery 지연 유발. CLAUDE.md는 "best-effort"로 명시되어 있으나 구현 확인 필요.
- **Fix**: Stage 5 success 후 즉시 리턴, Stage 6+는 별도 goroutine + 자체 timeout.

---

## LOW (선택적 정리)

- **L1.** `domain/user/model.go:7` — `kakao_id`가 JS number로 직렬화. 2^53 초과 시 정밀도 손실. `json:",string"` 태그 권장.
- **L2.** `/context/recent` vs `/context/latest` 중복 엔드포인트 (`context_history.go:204, 207`).
- **L3.** `Me`/`GetLevel`/`GetCoinHistory`에 `Cache-Control: no-store` 헤더 없음.
- **L4.** `PreviewEmail` 관리자 엔드포인트가 raw HTML 응답 (`admin.go:189`). CSP 없음.
- **L5.** POST 리소스 생성이 200/201 혼재. `POST /complete-signup`은 200 (`auth.go:398`), `POST /admin/terms`는 201, `POST /subscriptions`는 200. 일관되게 201로.
- **L6.** DELETE 엔드포인트가 body 반환. `subscription.go:60`, `brain_category.go:73`, `withdrawal_handler.go:138` → 204 권장.
- **L7.** 페이지네이션 inline 로직이 `withdrawal_admin_handler.go:25-38`에서 `parsePageParams` 우회.
- **L8.** `/delivery/trigger`, `/admin/delivery/preview`, `/api/v1/push/*`가 CLAUDE.md route map에 문서화 안 됨.
- **L9.** `LevelRepository.RestoreCoins` 비대칭 — 유저 없으면 조용히 0 row (`DeductCoins`는 에러).
- **L10.** `scheduler.go:93` delivery timeout 10분 고정. 유저 수 무관. *(주: scheduler.go의 deliver/retryFailed는 context.Background() 사용, collect()만 shutdownCtx)*
- **L11.** `defer tx.Rollback(ctx)` 일부에서 `_ = tx.Rollback(ctx)` 사용 안 함 (의도 명시성 부족).
- **L12.** `main.go:524-530` — `os.Exit(1)` in goroutine이 defer 스킵. `log.Fatal` at function scope 권장.
- **L13.** `server/platform/kakao/client.go:52` — `&http.Client{}` timeout 없음.
- **L14.** `cache.NewRedisCache` Redis 실패 시 in-process fallback → multi-replica 배포 시 signupCache/earnCache 일관성 깨짐. 프로덕션은 fail-closed.
- **L15.** `server/platform/gemini/client.go:62` — `baseURL + model + ":generateContent"` URL concat (`url.Parse` 권장).
- **L16.** `GetCategories`가 `categoryRepo.GetAllCategories` + `brainCatRepo.GetAll`을 직렬 호출 (`context_history.go:160-192`). errgroup 병렬화 가능.

---

## 우선순위 제안

### 1단계: 즉시 (하루~이틀)
1. **C1** (`SetTrustedProxies`) — 한 줄, 파급력 큼
2. **C2** (CSRF bypass) — 수시간
3. **C4** (Turnstile test key) — 수시간
4. **H3** (JWT_SECRET 길이 검증) — 수시간
5. **H14** (디버그 로그 제거) — 수분
6. **H18** (`kakao_id` drop) — 수시간

### 2단계: 1주 내 (돈/데이터 직결)
1. **C7** (EarnCoin 경쟁 조건) — 트랜잭션 재설계
2. **C10** (출금 멱등성 키 추가 — FOR UPDATE는 이미 있음)
3. **C6** (CompleteSignup 트랜잭션화)
4. ~~**C8** (스케줄러 오버랩) — FALSE, 이미 보호됨~~
5. **C9** (admin collect 가드)
6. **H4** (withdrawal approval lock)
7. **H5** (admin coin 상한)

### 3단계: 2주 내 (보안/운영)
1. **C3** (SSRF 허용리스트)
2. ~~**C5** (Kakao email overwrite) — FALSE, email 덮어쓰기 없음~~
3. **H1** (refresh token family)
4. **H2** (access token invalidation)
5. **H6** (CORS 명시 allowlist)
6. **H7** (image path 검증)
7. **H8** (PII 로깅 제거)
8. **H9, H10** (graceful shutdown + panic recovery)

### 4단계: 다음 릴리스 (안정성/UX)
1. **H11** (Gemini 타임아웃/서킷 브레이커)
2. **H12** (delivery dead-letter)
3. ~~**H13** (collect/delivery 중첩 감지) — FALSE, 3.5시간 버퍼~~
4. **H15** (응답 envelope 표준화) — 별도 PR
5. **H16** (409 EXPIRED)
6. **H17** (admin has_more 통일)
7. **M11** (EarnCoin race 테스트)
8. **M12** (/metrics)

### 5단계: 백로그
- 나머지 MEDIUM 전부
- M9, M10 (파일 분할 리팩토링)
- LOW 전체

---

## 테스트 커버리지 공백

CTO가 물을 질문들:

1. **`EarnCoin` 동시 호출 race** — `-race` + `sync.WaitGroup`으로 재현 가능한 테스트 없음
2. **`CompleteSignup` 부분 실패 롤백** — 트랜잭션 자체가 없으니 테스트 불가
3. **Scheduler 오버랩 시나리오** — 4AM run이 진행 중일 때 5AM cron 진입 케이스 없음
4. **Gemini 장애/타임아웃** — 서킷 브레이커가 없으니 테스트할 것도 없음
5. **`fetchAndValidateSources` 부분 실패 처리** — 일부 topic 실패 시 전체 행동 검증 없음
6. **Withdrawal 동시 approval** — 레이스 조건 테스트 없음
7. **Rate limit XFF 스푸핑** — 재현 케이스 없음
8. **SSRF 차단** — URL allowlist 테스트 없음

---

## 참조 파일 목록

### 보안 (C1~C10 관련)
- `server/main.go`
- `server/api/router.go`
- `server/api/middleware.go`
- `server/api/handler/auth.go`
- `server/api/handler/level_handler.go`
- `server/api/handler/withdrawal_handler.go`
- `server/api/handler/withdrawal_admin_handler.go`
- `server/api/handler/admin_coin_handler.go`
- `server/api/handler/admin.go`
- `server/auth/jwt.go`
- `server/auth/state_store.go`
- `server/auth/state_redis.go`
- `server/config/config.go`
- `server/domain/withdrawal/service.go`
- `server/domain/level/service.go`
- `server/domain/collector/service.go`
- `server/domain/collector/article_fetcher.go`
- `server/domain/collector/source_validator.go`
- `server/storage/withdrawal_repository.go`
- `server/storage/level_repository.go`
- `server/storage/refresh_token_repo.go`

### 아키텍처 (C6~C9, H9~H14 관련)
- `server/main.go`
- `server/scheduler/scheduler.go`
- `server/domain/collector/service.go`
- `server/domain/level/service.go`
- `server/domain/delivery/service.go`
- `server/storage/level_repository.go`
- `server/storage/collector_repo.go`
- `server/api/handler/auth.go`
- `server/api/handler/admin.go`
- `server/platform/gemini/client.go`
- `server/platform/kakao/client.go`

### API 계약 (H15~H18, M16~M21 관련)
- `server/api/router.go`
- `server/api/delivery_handler.go`
- `server/api/handler/helpers.go`
- `server/api/handler/auth.go`
- `server/api/handler/level_handler.go`
- `server/api/handler/context_history.go`
- `server/api/handler/mypage_handler.go`
- `server/api/handler/withdrawal_handler.go`
- `server/api/handler/withdrawal_admin_handler.go`
- `server/api/handler/admin_coin_handler.go`
- `server/api/handler/admin.go`
- `server/api/handler/subscription.go`
- `server/api/handler/user_delivery_channels.go`
- `server/api/handler/terms_handler.go`
- `server/api/handler/quiz_handler.go`
- `server/api/handler/brain_category.go`
- `server/api/handler/email_verification.go`
- `server/domain/collector/history_repository.go`
- `server/domain/delivery/service.go`
- `server/domain/user/model.go`
- `packages/shared/src/types.ts`
- `packages/shared/src/api.ts`

---

## 진행 상태 체크리스트

다른 세션에서 이 보고서를 이어받을 때, 아래 체크박스로 진행 상황 추적.

### CRITICAL
- [x] C1. SetTrustedProxies [FIXED 2026-04-23 — TrustedPlatform + X-Real-Client-IP 방식]
- [x] C2. CSRF bypass [PARTIALLY FIXED 2026-04-23 — SameSite=Lax로 핵심 벡터 차단]
- [ ] C3. SSRF allowlist
- [x] C4. Turnstile production guard [FIXED 2026-04-23]
- [x] C5. ~~Kakao email overwrite~~ [FALSE — email은 ON CONFLICT에서 갱신 안 됨]
- [ ] C6. CompleteSignup transaction
- [ ] C7. EarnCoin race condition
- [x] C8. ~~Scheduler overlap~~ [FALSE — CanRunToday가 running 상태도 체크함]
- [ ] C9. Admin collect guard
- [ ] C10. Withdrawal idempotency [PARTIALLY TRUE — 멱등성 키 없음은 사실, FOR UPDATE는 있음. HIGH로 하향 권장]

### HIGH
- [ ] H1. Refresh token family
- [ ] H2. Access token invalidation
- [x] H3. JWT_SECRET length [FIXED 2026-04-23]
- [ ] H4. Withdrawal approval lock
- [ ] H5. Admin coin max
- [ ] H6. CORS allowlist
- [ ] H7. Image path validation
- [ ] H8. PII logging
- [ ] H9. Background context
- [ ] H10. Panic recovery
- [ ] H11. Gemini timeout/breaker
- [ ] H12. Delivery dead-letter
- [x] H13. ~~Collect/delivery overlap detection~~ [FALSE — 실제 delivery 9:30AM, 3.5시간 버퍼]
- [ ] H14. Debug log removal
- [ ] H15. Response envelope standardization
- [ ] H16. 409 on EXPIRED
- [ ] H17. Admin has_more
- [ ] H18. kakao_id drop

### MEDIUM
- [ ] M1~M21 (21개)

### LOW
- [ ] L1~L16 (16개)
