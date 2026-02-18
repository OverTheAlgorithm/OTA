# Home 화면 리디자인 구현 계획

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 홈 화면에 관심사 태그 설정 UI와 수신한 맥락 이력 표시 기능을 추가한다.

**Architecture:** 백엔드에 6개 REST 엔드포인트를 추가하고, 프론트엔드는 InterestSection / HistorySection 두 컴포넌트로 분리해 home.tsx에 조합한다. 상태는 낙관적 업데이트(optimistic update)로 즉각 반응성을 확보한다.

**Tech Stack:** Go 1.22 + Gin, pgx/v5, React 18 + TypeScript, Tailwind CSS

---

## 설계 문서

`docs/plans/2026-02-18-home-redesign-design.md` 참고.

---

## Phase 1: 백엔드 — 구독/설정 리포지토리

### Task 1: 도메인 인터페이스 정의

**Files:**
- Create: `server/domain/user/subscription_repository.go`
- Create: `server/domain/user/preference_repository.go`

**Step 1: SubscriptionRepository 인터페이스 작성**

```go
// server/domain/user/subscription_repository.go
package user

import "context"

type SubscriptionRepository interface {
    GetSubscriptions(ctx context.Context, userID string) ([]string, error)
    AddSubscription(ctx context.Context, userID, category string) error
    DeleteSubscription(ctx context.Context, userID, category string) error
}
```

**Step 2: PreferenceRepository 인터페이스 작성**

```go
// server/domain/user/preference_repository.go
package user

import "context"

type PreferenceRepository interface {
    GetPreference(ctx context.Context, userID string) (deliveryEnabled bool, err error)
    UpsertPreference(ctx context.Context, userID string, deliveryEnabled bool) error
}
```

**Step 3: 커밋**
```bash
git add server/domain/user/
git commit -m "[홈] feat: 구독/설정 리포지토리 인터페이스 추가"
```

---

### Task 2: 구독/설정 스토리지 구현

**Files:**
- Create: `server/storage/subscription_repo.go`
- Create: `server/storage/preference_repo.go`

**Step 1: SubscriptionRepository 구현 작성**

```go
// server/storage/subscription_repo.go
package storage

import (
    "context"
    "github.com/jackc/pgx/v5/pgxpool"
)

type SubscriptionRepository struct {
    pool *pgxpool.Pool
}

func NewSubscriptionRepository(pool *pgxpool.Pool) *SubscriptionRepository {
    return &SubscriptionRepository{pool: pool}
}

func (r *SubscriptionRepository) GetSubscriptions(ctx context.Context, userID string) ([]string, error) {
    rows, err := r.pool.Query(ctx,
        `SELECT category FROM user_subscriptions WHERE user_id = $1 ORDER BY created_at`,
        userID,
    )
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var categories []string
    for rows.Next() {
        var cat string
        if err := rows.Scan(&cat); err != nil {
            return nil, err
        }
        categories = append(categories, cat)
    }
    if categories == nil {
        categories = []string{}
    }
    return categories, nil
}

func (r *SubscriptionRepository) AddSubscription(ctx context.Context, userID, category string) error {
    _, err := r.pool.Exec(ctx,
        `INSERT INTO user_subscriptions (user_id, category)
         VALUES ($1, $2)
         ON CONFLICT (user_id, category) DO NOTHING`,
        userID, category,
    )
    return err
}

func (r *SubscriptionRepository) DeleteSubscription(ctx context.Context, userID, category string) error {
    _, err := r.pool.Exec(ctx,
        `DELETE FROM user_subscriptions WHERE user_id = $1 AND category = $2`,
        userID, category,
    )
    return err
}
```

**Step 2: PreferenceRepository 구현 작성**

```go
// server/storage/preference_repo.go
package storage

import (
    "context"
    "errors"

    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgxpool"
)

type PreferenceRepository struct {
    pool *pgxpool.Pool
}

func NewPreferenceRepository(pool *pgxpool.Pool) *PreferenceRepository {
    return &PreferenceRepository{pool: pool}
}

func (r *PreferenceRepository) GetPreference(ctx context.Context, userID string) (bool, error) {
    var enabled bool
    err := r.pool.QueryRow(ctx,
        `SELECT delivery_enabled FROM user_preferences WHERE user_id = $1`,
        userID,
    ).Scan(&enabled)
    if errors.Is(err, pgx.ErrNoRows) {
        return true, nil // 기본값: 활성화
    }
    return enabled, err
}

func (r *PreferenceRepository) UpsertPreference(ctx context.Context, userID string, deliveryEnabled bool) error {
    _, err := r.pool.Exec(ctx,
        `INSERT INTO user_preferences (user_id, delivery_enabled)
         VALUES ($1, $2)
         ON CONFLICT (user_id) DO UPDATE
         SET delivery_enabled = $2, updated_at = NOW()`,
        userID, deliveryEnabled,
    )
    return err
}
```

**Step 3: 빌드 확인**
```bash
cd server && go build ./...
```
Expected: 에러 없음

**Step 4: 커밋**
```bash
git add server/storage/subscription_repo.go server/storage/preference_repo.go
git commit -m "[홈] feat: 구독/설정 스토리지 구현"
```

---

## Phase 2: 백엔드 — 맥락 이력 리포지토리

### Task 3: 맥락 이력 인터페이스 및 구현

**Files:**
- Create: `server/domain/collector/history_repository.go`
- Create: `server/storage/history_repo.go`

**Step 1: HistoryRepository 인터페이스 작성**

```go
// server/domain/collector/history_repository.go
package collector

import (
    "context"
    "time"
)

type HistoryItem struct {
    Category string `json:"category"`
    Rank     int    `json:"rank"`
    Topic    string `json:"topic"`
    Summary  string `json:"summary"`
}

type HistoryEntry struct {
    Date        string        `json:"date"`         // "2026-02-18"
    DeliveredAt time.Time     `json:"delivered_at"`
    Items       []HistoryItem `json:"items"`
}

type HistoryRepository interface {
    GetHistoryForUser(ctx context.Context, userID string) ([]HistoryEntry, error)
}
```

**Step 2: HistoryRepository 구현 작성**

```go
// server/storage/history_repo.go
package storage

import (
    "context"
    "time"

    "github.com/jackc/pgx/v5/pgxpool"
    "ota/domain/collector"
)

type HistoryRepository struct {
    pool *pgxpool.Pool
}

func NewHistoryRepository(pool *pgxpool.Pool) *HistoryRepository {
    return &HistoryRepository{pool: pool}
}

func (r *HistoryRepository) GetHistoryForUser(ctx context.Context, userID string) ([]collector.HistoryEntry, error) {
    rows, err := r.pool.Query(ctx, `
        SELECT
            dl.created_at,
            ci.category,
            ci.rank,
            ci.topic,
            ci.summary
        FROM delivery_logs dl
        JOIN collection_runs cr ON dl.run_id = cr.id
        JOIN context_items ci   ON ci.collection_run_id = cr.id
        WHERE dl.user_id = $1
          AND dl.status  = 'sent'
        ORDER BY dl.created_at DESC, ci.rank ASC
    `, userID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    entryMap := make(map[string]*collector.HistoryEntry)
    var order []string

    for rows.Next() {
        var deliveredAt time.Time
        var item collector.HistoryItem
        if err := rows.Scan(&deliveredAt, &item.Category, &item.Rank, &item.Topic, &item.Summary); err != nil {
            return nil, err
        }
        date := deliveredAt.UTC().Format("2006-01-02")
        if _, ok := entryMap[date]; !ok {
            entryMap[date] = &collector.HistoryEntry{
                Date:        date,
                DeliveredAt: deliveredAt,
                Items:       []collector.HistoryItem{},
            }
            order = append(order, date)
        }
        entryMap[date].Items = append(entryMap[date].Items, item)
    }

    result := make([]collector.HistoryEntry, 0, len(order))
    for _, date := range order {
        result = append(result, *entryMap[date])
    }
    return result, nil
}
```

**Step 3: 빌드 확인**
```bash
cd server && go build ./...
```

**Step 4: 커밋**
```bash
git add server/domain/collector/history_repository.go server/storage/history_repo.go
git commit -m "[홈] feat: 맥락 이력 리포지토리 구현"
```

---

## Phase 3: 백엔드 — 핸들러 및 라우터 연결

### Task 4: 구독/설정/이력 핸들러 작성

**Files:**
- Create: `server/api/handler/subscription.go`
- Create: `server/api/handler/preference.go`
- Create: `server/api/handler/context_history.go`

**Step 1: SubscriptionHandler 작성**

```go
// server/api/handler/subscription.go
package handler

import (
    "log"
    "net/http"

    "github.com/gin-gonic/gin"
    "ota/domain/user"
)

type SubscriptionHandler struct {
    repo user.SubscriptionRepository
    jwt  jwtValidator
}

type jwtValidator interface {
    authMiddleware() gin.HandlerFunc
}

func NewSubscriptionHandler(repo user.SubscriptionRepository, auth *AuthHandler) *SubscriptionHandler {
    return &SubscriptionHandler{repo: repo, jwt: auth}
}

func (h *SubscriptionHandler) List(c *gin.Context) {
    userID := c.GetString("userID")
    cats, err := h.repo.GetSubscriptions(c.Request.Context(), userID)
    if err != nil {
        log.Printf("get subscriptions error: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
        return
    }
    c.JSON(http.StatusOK, gin.H{"data": cats})
}

func (h *SubscriptionHandler) Add(c *gin.Context) {
    userID := c.GetString("userID")
    var body struct {
        Category string `json:"category" binding:"required"`
    }
    if err := c.ShouldBindJSON(&body); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "category is required"})
        return
    }
    if err := h.repo.AddSubscription(c.Request.Context(), userID, body.Category); err != nil {
        log.Printf("add subscription error: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
        return
    }
    c.JSON(http.StatusOK, gin.H{"data": "ok"})
}

func (h *SubscriptionHandler) Delete(c *gin.Context) {
    userID := c.GetString("userID")
    category := c.Param("category")
    if category == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "category is required"})
        return
    }
    if err := h.repo.DeleteSubscription(c.Request.Context(), userID, category); err != nil {
        log.Printf("delete subscription error: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
        return
    }
    c.JSON(http.StatusOK, gin.H{"data": "ok"})
}

func (h *SubscriptionHandler) RegisterRoutes(group *gin.RouterGroup) {
    group.Use(h.jwt.authMiddleware())
    group.GET("", h.List)
    group.POST("", h.Add)
    group.DELETE("/:category", h.Delete)
}
```

**Step 2: PreferenceHandler 작성**

```go
// server/api/handler/preference.go
package handler

import (
    "log"
    "net/http"

    "github.com/gin-gonic/gin"
    "ota/domain/user"
)

type PreferenceHandler struct {
    repo user.PreferenceRepository
    jwt  jwtValidator
}

func NewPreferenceHandler(repo user.PreferenceRepository, auth *AuthHandler) *PreferenceHandler {
    return &PreferenceHandler{repo: repo, jwt: auth}
}

func (h *PreferenceHandler) Get(c *gin.Context) {
    userID := c.GetString("userID")
    enabled, err := h.repo.GetPreference(c.Request.Context(), userID)
    if err != nil {
        log.Printf("get preference error: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
        return
    }
    c.JSON(http.StatusOK, gin.H{"data": gin.H{"delivery_enabled": enabled}})
}

func (h *PreferenceHandler) Update(c *gin.Context) {
    userID := c.GetString("userID")
    var body struct {
        DeliveryEnabled bool `json:"delivery_enabled"`
    }
    if err := c.ShouldBindJSON(&body); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
        return
    }
    if err := h.repo.UpsertPreference(c.Request.Context(), userID, body.DeliveryEnabled); err != nil {
        log.Printf("update preference error: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
        return
    }
    c.JSON(http.StatusOK, gin.H{"data": "ok"})
}

func (h *PreferenceHandler) RegisterRoutes(group *gin.RouterGroup) {
    group.Use(h.jwt.authMiddleware())
    group.GET("", h.Get)
    group.PUT("", h.Update)
}
```

**Step 3: ContextHistoryHandler 작성**

```go
// server/api/handler/context_history.go
package handler

import (
    "log"
    "net/http"

    "github.com/gin-gonic/gin"
    "ota/domain/collector"
)

type ContextHistoryHandler struct {
    repo collector.HistoryRepository
    jwt  jwtValidator
}

func NewContextHistoryHandler(repo collector.HistoryRepository, auth *AuthHandler) *ContextHistoryHandler {
    return &ContextHistoryHandler{repo: repo, jwt: auth}
}

func (h *ContextHistoryHandler) GetHistory(c *gin.Context) {
    userID := c.GetString("userID")
    entries, err := h.repo.GetHistoryForUser(c.Request.Context(), userID)
    if err != nil {
        log.Printf("get context history error: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
        return
    }
    c.JSON(http.StatusOK, gin.H{"data": entries})
}

func (h *ContextHistoryHandler) RegisterRoutes(group *gin.RouterGroup) {
    group.Use(h.jwt.authMiddleware())
    group.GET("/history", h.GetHistory)
}
```

**Step 4: main.go에 핸들러 및 라우트 등록**

`server/main.go` 의 `// Handlers` 섹션 이후에 추가:

```go
// 추가할 import
"ota/domain/user"  // 이미 있을 수 있음

// Handlers 섹션에 추가
subRepo := storage.NewSubscriptionRepository(pool)
prefRepo := storage.NewPreferenceRepository(pool)
histRepo := storage.NewHistoryRepository(pool)
subscriptionHandler := handler.NewSubscriptionHandler(subRepo, authHandler)
preferenceHandler := handler.NewPreferenceHandler(prefRepo, authHandler)
contextHistHandler := handler.NewContextHistoryHandler(histRepo, authHandler)
```

`NewRouter` 호출의 modules 슬라이스에 추가:

```go
{
    GroupName:   "subscriptions",
    Handler:     subscriptionHandler,
    Middlewares: []gin.HandlerFunc{},
},
{
    GroupName:   "preferences",
    Handler:     preferenceHandler,
    Middlewares: []gin.HandlerFunc{},
},
{
    GroupName:   "context",
    Handler:     contextHistHandler,
    Middlewares: []gin.HandlerFunc{},
},
```

**Step 5: 빌드 확인**
```bash
cd server && go build ./...
```
Expected: 에러 없음

**Step 6: 커밋**
```bash
git add server/api/handler/ server/main.go
git commit -m "[홈] feat: 구독/설정/이력 핸들러 및 라우트 등록"
```

---

## Phase 4: 프론트엔드 — API 레이어

### Task 5: TypeScript API 함수 추가

**Files:**
- Modify: `web/src/lib/api.ts`

**Step 1: 타입 및 함수 추가**

`web/src/lib/api.ts` 파일 하단에 추가:

```typescript
// ── 관심사(구독) ──────────────────────────────────────
export async function getSubscriptions(): Promise<string[]> {
  const res = await fetch("/api/v1/subscriptions", { credentials: "include" });
  if (!res.ok) throw new Error("Failed to fetch subscriptions");
  const body: ApiResponse<string[]> = await res.json();
  return body.data;
}

export async function addSubscription(category: string): Promise<void> {
  const res = await fetch("/api/v1/subscriptions", {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ category }),
  });
  if (!res.ok) throw new Error("Failed to add subscription");
}

export async function deleteSubscription(category: string): Promise<void> {
  const res = await fetch(`/api/v1/subscriptions/${encodeURIComponent(category)}`, {
    method: "DELETE",
    credentials: "include",
  });
  if (!res.ok) throw new Error("Failed to delete subscription");
}

// ── 맥락 이력 ─────────────────────────────────────────
export interface HistoryItem {
  category: string;
  rank: number;
  topic: string;
  summary: string;
}

export interface HistoryEntry {
  date: string;
  delivered_at: string;
  items: HistoryItem[];
}

export async function getContextHistory(): Promise<HistoryEntry[]> {
  const res = await fetch("/api/v1/context/history", { credentials: "include" });
  if (!res.ok) throw new Error("Failed to fetch context history");
  const body: ApiResponse<HistoryEntry[]> = await res.json();
  return body.data;
}
```

**Step 2: 커밋**
```bash
git add web/src/lib/api.ts
git commit -m "[홈] feat: 구독/이력 API 함수 추가"
```

---

## Phase 5: 프론트엔드 — 관심사 섹션 컴포넌트

### Task 6: InterestSection 컴포넌트

**Files:**
- Create: `web/src/components/interest-section.tsx`

```tsx
import { useState } from "react";
import { addSubscription, deleteSubscription } from "@/lib/api";

const PRESET_TAGS = [
  "연예/오락", "경제", "스포츠", "정치", "IT/기술",
  "패션/뷰티", "음식/맛집", "여행", "건강/의학",
  "게임", "사회/이슈", "문화/예술",
];

interface Props {
  selected: string[];
  onChange: (updated: string[]) => void;
}

export function InterestSection({ selected, onChange }: Props) {
  const [customInput, setCustomInput] = useState("");
  const [adding, setAdding] = useState(false);

  const handleAdd = async (category: string) => {
    const trimmed = category.trim();
    if (!trimmed || selected.includes(trimmed)) return;
    setAdding(true);
    onChange([...selected, trimmed]);           // optimistic
    try {
      await addSubscription(trimmed);
    } catch {
      onChange(selected);                        // rollback
    } finally {
      setAdding(false);
      setCustomInput("");
    }
  };

  const handleRemove = async (category: string) => {
    onChange(selected.filter((s) => s !== category)); // optimistic
    try {
      await deleteSubscription(category);
    } catch {
      onChange([...selected, category]);               // rollback
    }
  };

  const unselected = PRESET_TAGS.filter((t) => !selected.includes(t));

  return (
    <section className="rounded-2xl bg-[#1a1229] border border-[#2d1f42] p-6">
      {/* 헤더 */}
      <div className="flex items-center gap-2 mb-5">
        <div className="w-8 h-8 rounded-lg bg-[#5ba4d9]/10 flex items-center justify-center">
          <svg className="w-4 h-4 text-[#5ba4d9]" viewBox="0 0 24 24" fill="none"
            stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <path d="M20.59 13.41l-7.17 7.17a2 2 0 01-2.83 0L2 12V2h10l8.59 8.59a2 2 0 010 2.82z"/>
            <line x1="7" y1="7" x2="7.01" y2="7"/>
          </svg>
        </div>
        <h2 className="font-semibold text-[#f5f0ff]">내 관심사</h2>
        <span className="ml-auto text-xs text-[#9b8bb4]">
          {selected.length}개 선택됨
        </span>
      </div>

      {/* 선택된 태그 */}
      {selected.length > 0 && (
        <div className="flex flex-wrap gap-2 mb-4">
          {selected.map((tag) => (
            <button
              key={tag}
              onClick={() => handleRemove(tag)}
              className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full text-sm font-medium
                         bg-[#5ba4d9]/20 text-[#5ba4d9] border border-[#5ba4d9]/30
                         hover:bg-[#5ba4d9]/30 transition-colors"
            >
              {tag}
              <svg className="w-3 h-3" viewBox="0 0 12 12" fill="currentColor">
                <path d="M9 3L3 9M3 3l6 6" stroke="currentColor" strokeWidth="1.5"
                  strokeLinecap="round" fill="none"/>
              </svg>
            </button>
          ))}
        </div>
      )}

      {/* 구분선 */}
      {unselected.length > 0 && (
        <>
          <p className="text-xs text-[#9b8bb4] mb-3">추가할 수 있는 관심사</p>
          <div className="flex flex-wrap gap-2 mb-4">
            {unselected.map((tag) => (
              <button
                key={tag}
                onClick={() => handleAdd(tag)}
                disabled={adding}
                className="px-3 py-1.5 rounded-full text-sm text-[#9b8bb4]
                           border border-[#2d1f42] hover:border-[#5ba4d9]/50
                           hover:text-[#f5f0ff] transition-colors disabled:opacity-50"
              >
                + {tag}
              </button>
            ))}
          </div>
        </>
      )}

      {/* 직접 입력 */}
      <div className="flex gap-2 mt-2">
        <input
          type="text"
          value={customInput}
          onChange={(e) => setCustomInput(e.target.value)}
          onKeyDown={(e) => e.key === "Enter" && handleAdd(customInput)}
          placeholder="직접 입력..."
          className="flex-1 bg-[#0f0a19] border border-[#2d1f42] rounded-xl px-4 py-2
                     text-sm text-[#f5f0ff] placeholder-[#9b8bb4]/50
                     focus:outline-none focus:border-[#5ba4d9]/50 transition-colors"
        />
        <button
          onClick={() => handleAdd(customInput)}
          disabled={!customInput.trim() || adding}
          className="px-4 py-2 rounded-xl text-sm font-medium bg-[#5ba4d9]/20 text-[#5ba4d9]
                     border border-[#5ba4d9]/30 hover:bg-[#5ba4d9]/30 transition-colors
                     disabled:opacity-40 disabled:cursor-not-allowed"
        >
          추가
        </button>
      </div>
    </section>
  );
}
```

**커밋**
```bash
git add web/src/components/interest-section.tsx
git commit -m "[홈] feat: 관심사 선택 컴포넌트 구현"
```

---

## Phase 6: 프론트엔드 — 이력 섹션 컴포넌트

### Task 7: HistorySection 컴포넌트

**Files:**
- Create: `web/src/components/history-section.tsx`

```tsx
import type { HistoryEntry, HistoryItem } from "@/lib/api";

interface Props {
  entries: HistoryEntry[];
  subscriptions: string[];
  loading: boolean;
}

function formatDate(dateStr: string): string {
  const [y, m, d] = dateStr.split("-");
  return `${y}.${m}.${d}`;
}

function TopicRow({ item }: { item: HistoryItem }) {
  return (
    <li className="flex gap-3 py-2.5 border-b border-[#2d1f42]/60 last:border-0">
      <span className="mt-0.5 w-1.5 h-1.5 rounded-full bg-[#9b8bb4]/60 shrink-0 translate-y-1.5" />
      <div className="min-w-0">
        <p className="text-sm text-[#f5f0ff] leading-relaxed">{item.summary}</p>
        <p className="text-xs text-[#9b8bb4] mt-0.5 truncate">{item.topic}</p>
      </div>
    </li>
  );
}

function HistoryCard({ entry, subscriptions }: { entry: HistoryEntry; subscriptions: string[] }) {
  const topItems = entry.items.filter((i) => i.category === "top");
  const interestItems = entry.items.filter(
    (i) => i.category !== "top" && subscriptions.includes(i.category),
  );

  return (
    <div className="rounded-2xl bg-[#1a1229] border border-[#2d1f42] overflow-hidden">
      {/* 카드 헤더 */}
      <div className="px-6 py-4 border-b border-[#2d1f42] flex items-center justify-between">
        <span className="font-semibold text-[#f5f0ff]">{formatDate(entry.date)}</span>
        <span className="text-xs text-[#9b8bb4] bg-[#0f0a19] px-2.5 py-1 rounded-full border border-[#2d1f42]">
          {entry.items.length}개 토픽
        </span>
      </div>

      <div className="p-6 space-y-5">
        {/* 전체 맥락 */}
        {topItems.length > 0 && (
          <div>
            <div className="flex items-center gap-2 mb-3">
              <div className="w-6 h-6 rounded-md bg-[#e84d3d]/10 flex items-center justify-center">
                <svg className="w-3.5 h-3.5 text-[#e84d3d]" viewBox="0 0 24 24" fill="none"
                  stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <circle cx="12" cy="12" r="10"/><path d="M2 12h20"/>
                  <path d="M12 2a15 15 0 014 10 15 15 0 01-4 10 15 15 0 01-4-10 15 15 0 014-10z"/>
                </svg>
              </div>
              <span className="text-xs font-semibold text-[#e84d3d] uppercase tracking-wider">
                전체 맥락
              </span>
            </div>
            <ul>
              {topItems.map((item, i) => <TopicRow key={i} item={item} />)}
            </ul>
          </div>
        )}

        {/* 내 관심사 맥락 */}
        {interestItems.length > 0 && (
          <div>
            <div className="flex items-center gap-2 mb-3">
              <div className="w-6 h-6 rounded-md bg-[#5ba4d9]/10 flex items-center justify-center">
                <svg className="w-3.5 h-3.5 text-[#5ba4d9]" viewBox="0 0 24 24" fill="none"
                  stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <polygon points="12 2 15.09 8.26 22 9.27 17 14.14 18.18 21.02 12 17.77 5.82 21.02 7 14.14 2 9.27 8.91 8.26 12 2"/>
                </svg>
              </div>
              <span className="text-xs font-semibold text-[#5ba4d9] uppercase tracking-wider">
                내 관심사
              </span>
            </div>
            <ul>
              {interestItems.map((item, i) => (
                <li key={i} className="flex gap-3 py-2.5 border-b border-[#2d1f42]/60 last:border-0">
                  <span className="mt-0.5 w-1.5 h-1.5 rounded-full bg-[#5ba4d9]/60 shrink-0 translate-y-1.5" />
                  <div className="min-w-0">
                    <div className="flex items-center gap-2 mb-0.5">
                      <span className="text-xs px-1.5 py-0.5 rounded bg-[#5ba4d9]/10 text-[#5ba4d9] border border-[#5ba4d9]/20">
                        {item.category}
                      </span>
                    </div>
                    <p className="text-sm text-[#f5f0ff] leading-relaxed">{item.summary}</p>
                    <p className="text-xs text-[#9b8bb4] mt-0.5 truncate">{item.topic}</p>
                  </div>
                </li>
              ))}
            </ul>
          </div>
        )}
      </div>
    </div>
  );
}

export function HistorySection({ entries, subscriptions, loading }: Props) {
  return (
    <section>
      <div className="flex items-center gap-2 mb-4">
        <div className="w-8 h-8 rounded-lg bg-[#7bc67e]/10 flex items-center justify-center">
          <svg className="w-4 h-4 text-[#7bc67e]" viewBox="0 0 24 24" fill="none"
            stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8z"/>
            <polyline points="14 2 14 8 20 8"/>
            <line x1="16" y1="13" x2="8" y2="13"/>
            <line x1="16" y1="17" x2="8" y2="17"/>
            <polyline points="10 9 9 9 8 9"/>
          </svg>
        </div>
        <h2 className="font-semibold text-[#f5f0ff]">받아본 맥락</h2>
      </div>

      {loading ? (
        <div className="space-y-4">
          {[1, 2].map((i) => (
            <div key={i} className="rounded-2xl bg-[#1a1229] border border-[#2d1f42] p-6 animate-pulse">
              <div className="h-4 bg-[#2d1f42] rounded w-24 mb-4" />
              <div className="space-y-2">
                <div className="h-3 bg-[#2d1f42] rounded w-full" />
                <div className="h-3 bg-[#2d1f42] rounded w-3/4" />
              </div>
            </div>
          ))}
        </div>
      ) : entries.length === 0 ? (
        <div className="rounded-2xl bg-[#1a1229] border border-[#2d1f42] p-12 text-center">
          <p className="text-[#9b8bb4] text-sm">아직 받은 맥락이 없습니다.</p>
          <p className="text-[#9b8bb4]/60 text-xs mt-1">매일 아침 7시에 첫 브리핑이 전달됩니다.</p>
        </div>
      ) : (
        <div className="space-y-4">
          {entries.map((entry) => (
            <HistoryCard key={entry.date} entry={entry} subscriptions={subscriptions} />
          ))}
        </div>
      )}
    </section>
  );
}
```

**커밋**
```bash
git add web/src/components/history-section.tsx
git commit -m "[홈] feat: 맥락 이력 컴포넌트 구현"
```

---

## Phase 7: 프론트엔드 — Home 페이지 조립

### Task 8: home.tsx 업데이트

**Files:**
- Modify: `web/src/pages/home.tsx`

전체 파일을 아래 내용으로 교체:

```tsx
import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useAuth } from "@/contexts/auth-context";
import { InterestSection } from "@/components/interest-section";
import { HistorySection } from "@/components/history-section";
import { getSubscriptions, getContextHistory, type HistoryEntry } from "@/lib/api";

export function HomePage() {
  const { user, loading, logout } = useAuth();
  const navigate = useNavigate();

  const [subscriptions, setSubscriptions] = useState<string[]>([]);
  const [history, setHistory] = useState<HistoryEntry[]>([]);
  const [historyLoading, setHistoryLoading] = useState(true);

  useEffect(() => {
    if (!loading && !user) navigate("/", { replace: true });
  }, [user, loading, navigate]);

  useEffect(() => {
    if (!user) return;
    getSubscriptions().then(setSubscriptions).catch(() => {});
    getContextHistory()
      .then(setHistory)
      .catch(() => {})
      .finally(() => setHistoryLoading(false));
  }, [user]);

  if (loading || !user) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-[#0f0a19]">
        <p className="text-[#9b8bb4]">로딩 중...</p>
      </div>
    );
  }

  const displayName = user.nickname || user.email || "사용자";

  const handleLogout = async () => {
    await logout();
    navigate("/", { replace: true });
  };

  // 다음 브리핑 시간 계산 (매일 07:00 KST)
  const nextBriefing = () => {
    const now = new Date();
    const kst = new Date(now.toLocaleString("en-US", { timeZone: "Asia/Seoul" }));
    const next = new Date(kst);
    next.setHours(7, 0, 0, 0);
    if (kst >= next) next.setDate(next.getDate() + 1);
    const diff = next.getTime() - kst.getTime();
    const h = Math.floor(diff / 3600000);
    const m = Math.floor((diff % 3600000) / 60000);
    return h > 0 ? `${h}시간 ${m}분 후` : `${m}분 후`;
  };

  return (
    <div className="min-h-screen bg-[#0f0a19] text-[#f5f0ff]">
      {/* Header */}
      <header className="sticky top-0 z-10 border-b border-[#2d1f42] bg-[#0f0a19]/90 backdrop-blur-lg">
        <div className="max-w-2xl mx-auto px-6 h-16 flex items-center justify-between">
          <img src="/OTA_logo.png" alt="OTA" className="h-7" />
          <div className="flex items-center gap-3">
            <div className="flex items-center gap-2">
              {user.profile_image ? (
                <img src={user.profile_image} alt="" className="w-8 h-8 rounded-full ring-1 ring-[#2d1f42]" />
              ) : (
                <div className="w-8 h-8 rounded-full bg-[#2d1f42] flex items-center justify-center text-xs text-[#9b8bb4]">
                  {displayName[0]}
                </div>
              )}
              <span className="text-sm text-[#9b8bb4] hidden sm:block">{displayName}</span>
            </div>
            <button
              onClick={handleLogout}
              className="text-sm text-[#9b8bb4] hover:text-[#f5f0ff] transition-colors"
            >
              로그아웃
            </button>
          </div>
        </div>
      </header>

      <main className="max-w-2xl mx-auto px-6 py-8 space-y-6">
        {/* Welcome Banner */}
        <div className="rounded-2xl bg-gradient-to-br from-[#1a1229] to-[#1e1530] border border-[#2d1f42] px-6 py-5 flex items-center justify-between">
          <div>
            <p className="text-sm text-[#9b8bb4]">안녕하세요</p>
            <h1 className="text-lg font-bold text-[#f5f0ff] mt-0.5">{displayName}님</h1>
          </div>
          <div className="text-right">
            <p className="text-xs text-[#9b8bb4]">다음 브리핑</p>
            <p className="text-sm font-semibold text-[#e84d3d] mt-0.5">{nextBriefing()}</p>
          </div>
        </div>

        {/* 관심사 섹션 */}
        <InterestSection selected={subscriptions} onChange={setSubscriptions} />

        {/* 이력 섹션 */}
        <HistorySection entries={history} subscriptions={subscriptions} loading={historyLoading} />
      </main>
    </div>
  );
}
```

**커밋**
```bash
git add web/src/pages/home.tsx
git commit -m "[홈] feat: 홈 페이지 관심사/이력 섹션 조립"
```

---

## Phase 8: 최종 확인

### Task 9: 빌드 및 통합 확인

**Step 1: 백엔드 빌드**
```bash
cd server && go build ./...
```

**Step 2: 프론트엔드 빌드**
```bash
cd web && npm run build
```

**Step 3: 최종 커밋 & 푸시**
```bash
git push origin main
```
