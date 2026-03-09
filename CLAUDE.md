# OTA (Over the Algorithm)

AI-curated daily briefing service. Collects trending topics, summarizes with AI, delivers via email, gamifies reading with coins/levels.

## Tech Stack
- **Server**: Go 1.22 + Gin + pgx (PostgreSQL)
- **Frontend**: React + TypeScript + Vite + Tailwind CSS
- **Infra**: Docker (testcontainers for integration tests), Cloudflare Turnstile
- **AI**: Gemini (primary) / OpenAI (fallback) for summarization + image generation

## Architecture

### Server (`server/`)
```
domain/           # Business logic (no DB imports)
  collector/      # Data collection pipeline (sources → AI clustering → images)
  delivery/       # Email delivery orchestration
  level/          # Coin/level gamification
  user/           # User + email verification
  withdrawal/     # Coin withdrawal (cashout)
  terms/          # Terms of service (immutable records, no update/delete)
storage/          # PostgreSQL repository implementations (pgxpool)
api/              # Gin router, middleware (auth, CORS, admin)
  handler/        # HTTP handlers per domain
auth/             # JWT (7d expiry, cookie: ota_token) + OAuth state
cache/            # In-process TTL cache (otter) — earnCache, signupCache
config/           # Env var loading (~30 vars)
platform/         # External integrations (kakao, gemini, openai, email, googlenews, googletrends)
scheduler/        # Cron: collect 4-6AM, deliver 7AM, retry 7:30-8:30AM KST
```

### Frontend (`web/src/`)
```
pages/            # landing, home, topic, withdrawal, admin, admin-withdrawals, admin-terms, terms-consent, email-verification
components/       # interest-section, channel-preferences-section, level-card, history-section, send-briefing-button
lib/api.ts        # All API client functions
contexts/         # auth-context (AuthProvider with JWT cookie)
```

## Database Tables (PostgreSQL)

| Table | Purpose |
|-------|---------|
| users | id, kakao_id, email, nickname, profile_image, role |
| user_points | user_id (PK), points, created_at, updated_at — **current coin balance** |
| coin_logs | Earning log: user_id, run_id, context_item_id, coins_earned — **topic views only** |
| coin_events | *(NEW - pending)* General coin events: signup bonus, promotions, admin adjustments |
| collection_runs | Pipeline run metadata (status, timestamps) |
| context_items | AI-processed topics (topic, summary, detail, buzz_score, brain_category, image_path) |
| brain_categories | Display categories (key, emoji, label, accent_color) |
| user_subscriptions | User interest subscriptions (category) |
| user_delivery_channels | Delivery channel preferences (email enabled/disabled) |
| user_preferences | delivery_enabled flag |
| delivery_logs | Per-user delivery status per run |
| withdrawals | Withdrawal requests (amount, bank info) |
| withdrawal_transitions | Status history (pending→approved/rejected/cancelled, actor, note) |
| user_bank_accounts | Saved bank account info |
| terms | Immutable ToS records (title, url, version, active, required) |
| user_term_consents | user_id + term_id consent records |
| email_verification_codes | Email verification flow |
| trending_items | Cached trending data |

## Critical Design Decisions

### Coin System — Split Ledgers
- **coin_logs**: Topic-view earnings only. Has `context_item_id` (NOT NULL) + `run_id` (NOT NULL) + UNIQUE constraint.
- **withdrawals**: Tracks deductions via `DeductCoins`/`RestoreCoins` on `user_points` directly.
- **coin_events** (pending migration 000020): For non-topic events (signup bonus, etc.).
- `SetCoins()` in `level_repository.go` directly overwrites `user_points.points` — **currently does NOT create any log entry**. Must always pair with a `coin_events` insert.
- Unified history = UNION of coin_logs + coin_events + withdrawals, sorted by created_at DESC.

### Two-Phase Signup (Terms Consent)
- Kakao OAuth callback → if new user → cache PendingSignup in otter cache (TTL 3min, key=UUID) → redirect to /terms-consent
- User agrees to terms → POST /complete-signup with UUID + agreed_term_ids → server validates cache + required terms → creates user + saves consents atomically → busts cache
- `FindByKakaoID` (read-only) used instead of `UpsertByKakaoID` to avoid premature DB insert

### Terms of Service
- Immutable records: no update, no delete (except active toggle via PATCH /:id/active)
- UNIQUE(title, version) — new version = new record
- Required terms must be agreed during signup (server validates authoritatively)

### Withdrawal Flow
- Request → DeductCoins immediately → status=pending
- Admin approve → stays deducted (completed)
- Admin reject or user cancel → RestoreCoins
- Deductions/restorations modify `user_points` directly (no coin_log entry — tracked in withdrawal_transitions)

## API Patterns
- Response envelope: `{"data": T}` or `{"error": "message"}`
- Auth: JWT in `ota_token` cookie (Secure, HttpOnly, SameSite=None, 7d MaxAge)
- Admin routes: AuthMiddleware + AdminMiddleware (checks user.role == "admin")
- Public routes: /terms/active, /level/init-earn, /level/earn, /context/topic/:id

## Testing
- Unit tests: mocks in `_test.go` files (same package for handler tests, internal for domain tests)
- Integration tests: `server/integration/` with testcontainers (postgres:16-alpine)
- Mock naming: use unique suffixes per file to avoid conflicts (e.g., `mockTermsRepoForHandler`, `mockTermsRepoSignup`)

## Env Vars (key ones)
- `SIGNUP_BONUS_COINS` (default: 0) — coins granted on signup via SetCoins
- `DAILY_COIN_LIMIT` (default: 10), `COIN_CAP` (default: 5000), `COINS_PER_LEVEL` (default: 1000)
- `MIN_WITHDRAWAL_AMOUNT` (default: 1000)
- `EARN_MIN_DURATION_SEC` (default: 10) — dwell time before coin earn is confirmed
