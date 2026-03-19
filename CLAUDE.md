# WizLetter (위즈레터) — formerly OTA

AI-curated daily briefing service. Collects trending topics, summarizes with AI, delivers via email, gamifies reading with coins/levels.

**Branding**: Public-facing brand is "WizLetter" (위즈레터). Go module name is still `ota`. Domain: overthealgorithm.mindhacker.club (frontend, Vercel), server.mindhacker.club (backend, Oracle Cloud + Caddy).

## Tech Stack
- **Server**: Go 1.25 + Gin + pgx (PostgreSQL 16)
- **Frontend**: React 19 + TypeScript 5.9 + Vite 7 + Tailwind CSS 4.1
- **Infra**: Docker Compose (Caddy + Go + Postgres), Vercel (frontend), Oracle Cloud (backend), GitHub Actions CI/CD
- **AI**: Gemini (primary, with model fallback) / OpenAI (fallback) for summarization + Gemini image generation
- **Security**: Cloudflare Turnstile, rate limiting (ulule/limiter), 5-layer anti-cheat

## Architecture

### Server (`server/`)
```
domain/           # Business logic (no DB imports)
  collector/      # Data collection pipeline (sources -> AI clustering -> images)
  delivery/       # Email delivery orchestration
  level/          # Coin/level gamification (BaseCoinPreferred=5, BaseCoinNonPreferred=10)
  user/           # User + email verification
  withdrawal/     # Coin withdrawal (cashout)
  terms/          # Terms of service
storage/          # PostgreSQL repository implementations (pgxpool)
api/              # Gin router, middleware (auth, CORS, admin, rate-limit)
  handler/        # HTTP handlers per domain
auth/             # JWT (7d expiry, cookie: ota_token) + OAuth state
cache/            # In-process TTL cache (otter) -- earnCache, signupCache
config/           # Env var loading
platform/         # External integrations (kakao, gemini, openai, email, googlenews, googletrends)
scheduler/        # Cron: collect 4-6AM, deliver 7AM, retry 7:30-8:30AM KST
migrations/       # 23 SQL migrations (golang-migrate)
integration/      # Integration tests (testcontainers)
```

### Frontend (`web/src/`)
```
pages/            # landing, home, topic, allnews, mypage, withdrawal, email-verification,
                  # terms-consent, admin, admin-coins, admin-withdrawals, admin-terms
components/       # kakao-login-button, footer, level-card, interest-section,
                  # channel-preferences-section, history-section, send-briefing-button
lib/api.ts        # All API client functions
lib/adblock.ts    # Adblock detection (bait method)
contexts/         # auth-context (AuthProvider with JWT cookie)
```

## Database Tables (PostgreSQL, 23 migrations)

| Table | Purpose |
|-------|---------|
| users | id (UUID), kakao_id, email (UNIQUE), nickname, profile_image, role, email_verified |
| user_points | user_id (PK), points, created_at, updated_at -- **current coin balance** |
| coin_logs | Topic-view earnings: user_id, run_id, context_item_id, coins_earned. UNIQUE(user_id, run_id, context_item_id) |
| coin_events | General coin events: id, user_id, amount, type, memo, actor_id, created_at (migration 000020+000021) |
| collection_runs | Pipeline run metadata: id, started_at, completed_at, status, error_message |
| context_items | AI-processed topics: id, collection_run_id, category, priority, brain_category, rank, topic, summary, detail, details (JSON), buzz_score, sources (JSON), image_path |
| categories | News categories: key (PK), label, display_order. Seed: general, entertainment, business, sports, technology, science, health |
| news_sources | RSS feed URLs: id, category_key (FK), provider, url, enabled |
| brain_categories | Display categories: key (PK), emoji, label, accent_color, display_order, instruction |
| user_subscriptions | User interest subscriptions (user_id, category) |
| user_delivery_channels | Channel prefs: user_id, channel (email/kakao/telegram/sms/push), enabled |
| user_preferences | delivery_enabled flag |
| delivery_logs | Per-user delivery status: run_id, user_id, channel, status, error_message, retry_count |
| withdrawals | Withdrawal requests: id, user_id, amount, bank_name, account_number, account_holder |
| withdrawal_transitions | Status history: withdrawal_id, status, note, actor_id, created_at |
| user_bank_accounts | Saved bank account info |
| terms | ToS records: id, title, description, url, version, active, required. UNIQUE(title, version) |
| user_term_consents | user_id + term_id consent records |
| email_verification_codes | OTP: user_id, code, email, expires_at |
| trending_items | Cached trending data: item_id, category, data (JSON), expires_at |

## API Endpoints (prefix: /api/v1)

### Public
- `GET /terms/active` -- active terms for consent screen
- `GET /context/topic/:id` -- single topic detail
- `GET /context/recent` -- recent topics (landing)
- `GET /context/topics?filter_type=&filter_value=&limit=&offset=` -- paginated topics
- `GET /context/categories` -- categories + brain_categories for filter UI
- `GET /brain-categories` -- all brain categories
- `POST /level/init-earn` -- initiate earn (needs auth)
- `POST /level/earn` -- confirm earn (needs auth)

### Auth Required
- `GET /auth/me`, `POST /auth/logout`, `DELETE /auth/delete-account`
- `POST /auth/complete-signup` -- two-phase signup with terms consent
- `GET,POST,DELETE /subscriptions` -- category interest management
- `GET,PUT /user/delivery-channels`, `GET /user/delivery-status`
- `GET /level` -- user level info
- `POST /level/batch-earn-status` -- batch query earn states
- `POST /delivery/send` -- send briefing on-demand
- `GET /context/history?limit=&offset=` -- personal reading history
- `GET /mypage/coin-history?limit=&offset=` -- coin transaction history
- `POST /email-verification/send-code`, `POST /email-verification/verify-code`
- `GET /withdrawal/info`, `GET,PUT /withdrawal/bank-account`
- `POST /withdrawal/request`, `GET /withdrawal/history`, `POST /withdrawal/:id/cancel`

### Admin (AuthMiddleware + AdminMiddleware)
- `POST /admin/collect` -- trigger collection (202, async + Slack notification)
- `POST /admin/delivery/send-test` -- send test email
- `POST /admin/level/set-coins` -- manually set user coins
- `POST /admin/level/create-mock-item` -- create mock context item
- `GET /admin/coins/search?type=email|id&q=` -- search user
- `POST /admin/coins/adjust` -- adjust coins + create coin_event with memo
- `GET,POST /admin/brain-categories`, `PUT,DELETE /admin/brain-categories/:key`
- `GET /admin/withdrawals`, `GET /admin/withdrawals/:id`
- `POST /admin/withdrawals/:id/approve`, `POST /admin/withdrawals/:id/reject`
- `PUT /admin/withdrawals/transitions/:id/note`
- `GET,POST /admin/terms`, `PATCH /admin/terms/:id/active`, `PATCH /admin/terms/:id`

## Critical Design Decisions

### Coin System -- Split Ledgers
- **coin_logs**: Topic-view earnings only. UNIQUE(user_id, run_id, context_item_id). BaseCoinPreferred=5 (subscribed), BaseCoinNonPreferred=10 (unsubscribed).
- **coin_events**: Non-topic events (signup bonus, admin adjustments). Has actor_id for audit.
- **withdrawals**: Deductions via DeductCoins/RestoreCoins on user_points directly.
- `SetCoins()` in `level_repository.go` directly overwrites user_points.points -- must pair with coin_events insert.
- Unified history (mypage/coin-history) = UNION of coin_logs + coin_events + withdrawals, sorted by created_at DESC.

### Two-Phase Signup (Terms Consent)
- Kakao OAuth callback -> if new user -> cache PendingSignup in otter (TTL 3min, key=UUID) -> redirect to /terms-consent
- POST /complete-signup with UUID + agreed_term_ids -> validates cache + required terms -> creates user + saves consents atomically -> busts cache
- FindByKakaoID (read-only) used instead of UpsertByKakaoID to avoid premature DB insert

### Terms of Service
- UNIQUE(title, version) -- new version = new record
- PATCH /:id/active toggles active status
- PATCH /:id updates term fields (title, url, description, etc.)
- Required terms must be agreed during signup (server validates authoritatively)

### Withdrawal Flow
- Request -> DeductCoins immediately -> status=pending
- Admin approve -> stays deducted
- Admin reject or user cancel -> RestoreCoins
- Tracked in withdrawal_transitions (not coin_logs)

### Rate Limiting
- ulule/limiter/v3 sliding-window, in-memory store
- Key: authenticated users by user ID (`user:<id>`), anonymous by IP (`ip:<addr>`)
- Default: 100 req/min. Env: RATE_LIMIT_PER_MIN. Fail-open on limiter errors.

### Data Collection Pipeline
- Stage 0: Google Trends + Google News (RSS by category from news_sources table)
- Stage 1: AI clustering (Phase 1 prompt)
- Stage 2: URL decoding (Google News redirect unwrapping)
- Stage 3: Article fetch + host validation
- Stage 4: AI writing (Phase 2, parallel per topic). Primary model with fallback on 5xx.
- Stage 5: Save to DB (collection_runs + context_items)
- Stage 6: Image generation (best-effort, Gemini)

## Middleware Stack (order)
1. Recovery (gin built-in)
2. LoggerMiddleware (user ID extraction from JWT for log context)
3. CORSMiddleware (single frontend origin, credentials)
4. RateLimitMiddleware (global, per-user/IP sliding window)
5. Per-route: AuthMiddleware, AdminMiddleware

## Deployment
- **Frontend**: Vercel. vercel.json rewrites /api/* to server.mindhacker.club.
- **Backend**: Oracle Cloud Ubuntu. Docker Compose: Caddy (SSL, 80/443) -> Go server (8080) -> Postgres (5432, internal only).
- **CI/CD**: GitHub Actions on push to main. Build check -> SSH deploy -> docker compose up --build.
- **Domains**: overthealgorithm.mindhacker.club (frontend), server.mindhacker.club (backend API).

## Testing
- Unit tests: mocks in `_test.go` files (same package for handler tests, internal for domain tests)
- Integration tests: `server/integration/` with testcontainers (postgres:16-alpine)
- Mock naming: use unique suffixes per file to avoid conflicts (e.g., `mockTermsRepoForHandler`)

## Env Vars (key ones)
- `RATE_LIMIT_PER_MIN` (default: 100) -- per-user/IP rate limit
- `SIGNUP_BONUS_COINS` (default: 0), `DAILY_COIN_LIMIT` (default: 10)
- `COIN_CAP` (default: 5000), `COINS_PER_LEVEL` (default: 1000)
- `EXTRA_COIN_LIMIT_PER_LEVEL` (default: 0) -- additional daily coins per level
- `MIN_WITHDRAWAL_AMOUNT` (default: 1000)
- `EARN_MIN_DURATION_SEC` (default: 10) -- dwell time before earn confirmed
- `AI_PROVIDER` (gemini|openai), `GEMINI_API_KEY`, `GEMINI_MODEL` (default: gemini-3.1-pro-preview)
- `GEMINI_MODEL_FALLBACK` (default: gemini-3-flash-preview) -- used on primary 5xx
- `IMAGE_GENERATION_MODEL` (required) -- Gemini model for thumbnail generation
- `TURNSTILE_SECRET_KEY` (default: test key)
- `SLACK_WEBHOOK_URL` (optional) -- async admin notifications on collect
- `FRONTEND_URL` (default: http://localhost:5173), `SERVER_PORT` (default: 8080)
- `APP_ENV` (development|production)
- Frontend: `VITE_API_URL`, `VITE_TURNSTILE_SITE_KEY`
