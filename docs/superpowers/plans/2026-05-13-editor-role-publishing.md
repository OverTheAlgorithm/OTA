# Editor Role & Publishing — Implementation Plan

> Spec: `docs/superpowers/specs/2026-05-13-editor-role-publishing-design.md`

**Goal:** Ship an editor role with rich-text publishing, public listing, and sitemap integration.

**Architecture:** Add `editor` role between `user` and `admin` with a single ordered-precedence helper, a generic `RequireRoleMiddleware`, a new `editor` domain (model + repo + service), a TipTap-based React editor, and a sitemap extension that reads from the new table.

**Tech Stack:** Go 1.25 / Gin / pgx / bluemonday HTML sanitiser / React 19 / TipTap v2 / Tailwind 4.

---

## Phase 1 — Backend foundation

### Task 1: Migration 000036 — role hierarchy + audit + editor_posts tables

**Files**
- Create: `server/migrations/000036_editor_role_and_posts.up.sql`
- Create: `server/migrations/000036_editor_role_and_posts.down.sql`

The migration must:
1. Backfill `users.role` (`UPDATE users SET role='user' WHERE role IS NULL OR role=''`).
2. Add `NOT NULL`, `DEFAULT 'user'`, and `CHECK (role IN ('user','editor','admin'))` on `users.role`.
3. Create `role_change_logs` (id, user_id FK, before_role, after_role, actor_id FK nullable, memo, created_at).
4. Create `editor_posts` (full schema from spec) + two indexes.

Down migration reverses cleanly.

### Task 2: Role helper + RequireRoleMiddleware

**Files**
- Modify: `server/domain/user/model.go` — add `HasRoleAtLeast` and role constants.
- Modify: `server/api/middleware.go` — add `RequireRoleMiddleware`, rewrite `AdminMiddleware` in terms of it.
- Create: `server/api/middleware_role_test.go` — unit tests for the helper + middleware.

`HasRoleAtLeast` table:

```go
const (
    RoleUser   = "user"
    RoleEditor = "editor"
    RoleAdmin  = "admin"
)

var roleRank = map[string]int{
    RoleUser:   0,
    RoleEditor: 1,
    RoleAdmin:  2,
}

func HasRoleAtLeast(role, min string) bool {
    return roleRank[role] >= roleRank[min]
}
```

### Task 3: Role change audit repository

**Files**
- Create: `server/domain/user/role_change.go` (model + `RoleChangeRepository` interface)
- Create: `server/storage/role_change_repo.go`
- Modify: `server/domain/user/repository.go` — add `UpdateRole(ctx, userID, newRole) error`.
- Modify: `server/storage/user_repo.go` — implement `UpdateRole`.

`RoleChangeRepository.Log` + `ListByUser`.

### Task 4: Admin user-management handler

**Files**
- Create: `server/api/handler/admin_user_handler.go`
- Create: `server/api/handler/admin_user_handler_test.go`

Endpoints:
- `GET /search?type=id|email&q=` — reuses `userRepo.FindByID/FindByEmail`.
- `POST /role` — body `{user_id, new_role, memo}`. Validates role value, rejects self-change, runs `UpdateRole` + `Log` inside a transaction.
- `GET /:id/role-history` — paginated audit list (limit/offset).

---

## Phase 2 — Editor posts backend

### Task 5: HTML sanitiser package

**Files**
- Create: `server/domain/editor/sanitizer.go`
- Create: `server/domain/editor/sanitizer_test.go`
- Modify: `server/go.mod` — add `github.com/microcosm-cc/bluemonday`.

Single-policy `Sanitize(html string) string` and a helper `Excerpt(html string, n int) string` that strips tags into plain text.

### Task 6: Editor domain model + repository interface

**Files**
- Create: `server/domain/editor/model.go` (Post struct, status constants, validation errors)
- Create: `server/domain/editor/repository.go` (interface)

### Task 7: Editor postgres repository

**Files**
- Create: `server/storage/editor_repo.go`

Methods: `Create`, `Update`, `Delete`, `FindByID`, `ListPublished(limit, offset)`, `ListByAuthor(authorID)`, `CountPublished`.

### Task 8: Editor service

**Files**
- Create: `server/domain/editor/service.go`
- Create: `server/domain/editor/service_test.go`

Service responsibilities:
- Sanitise content before save.
- Derive `content_text` (via `Excerpt`) and `first_image_url` (regex over sanitised HTML).
- Enforce ownership (`callerID == authorID || callerRole == admin`).
- Validate title/content length.
- Set `published_at` when status flips to `published`.

### Task 9: Editor CRUD handler

**Files**
- Create: `server/api/handler/editor_handler.go`
- Create: `server/api/handler/editor_handler_test.go`

Endpoints: POST/GET list/GET id/PUT/DELETE under `/editor/posts`.

### Task 10: Image upload handler

**Files**
- Create: `server/api/handler/editor_upload_handler.go`
- Create: `server/api/handler/editor_upload_handler_test.go`

`POST /editor/upload-image`. Validates magic bytes (use `net/http.DetectContentType`), 5 MB cap, writes to `data/images/editor/YYYY/MM/{uuid}.{ext}`.

---

## Phase 3 — Public + sitemap

### Task 11: Public editor-picks handler

**Files**
- Create: `server/api/handler/editor_pick_handler.go`
- Create: `server/api/handler/editor_pick_handler_test.go`

Two endpoints: `GET /editor-picks` (paginated cards) and `GET /editor-picks/:id` (full post).

### Task 12: Sitemap extension

**Files**
- Modify: `server/storage/sitemap_repository.go` — add `GetAllEditorPostRows`.
- Modify: `server/api/handler/sitemap_handler.go` — extend interface with `GetAllEditorPostEntries`, append URLs, add `/editor-picks` to `staticPages`.
- Modify: `server/main.go` — extend `sitemapRepoAdapter`.
- Modify: `server/api/handler/sitemap_handler_test.go` — cover the new entries.

---

## Phase 4 — Wiring

### Task 13: Wire everything in main.go

**Files**
- Modify: `server/main.go`

Register: `editorRepo`, `editorService`, `editorHandler`, `editorUploadHandler`, `editorPickHandler`, `adminUserHandler`, `roleChangeRepo`. Add route modules with `RequireRoleMiddleware(userRepo, "editor")` for editor routes and `AdminMiddleware` for admin user routes.

---

## Phase 5 — Frontend

### Task 14: Install TipTap and adjacent deps

**Files**
- Modify: `web/package.json`

Add: `@tiptap/react`, `@tiptap/pm`, `@tiptap/starter-kit`, `@tiptap/extension-image`, `@tiptap/extension-link`, `@tiptap/extension-placeholder`, `@tiptap/extension-character-count`.

### Task 15: API client additions

**Files**
- Modify: `web/src/lib/api.ts`

Add `User.role` ∈ `'user' | 'editor' | 'admin'`. Add `hasRoleAtLeast`, plus `createEditorPost`, `updateEditorPost`, `deleteEditorPost`, `getEditorPost`, `listMyEditorPosts`, `uploadEditorImage`, `listEditorPicks`, `getEditorPick`, `adminSearchUser`, `adminUpdateRole`, `adminListRoleHistory`.

### Task 16: Editor component (TipTap wrapper)

**Files**
- Create: `web/src/components/rich-text-editor.tsx`

`<RichTextEditor value={...} onChange={...} onImageUpload={fn} />` with toolbar (bold/italic/underline/strike/H1-H3/bullet/numbered/quote/code-block/link/image/clear).

### Task 17: Editor pages (new / edit)

**Files**
- Create: `web/src/pages/editor-new.tsx`
- Create: `web/src/pages/editor-edit.tsx`

Title input + RichTextEditor + draft/publish actions. Edit page loads existing post, prefills, supports delete.

### Task 18: Public editor-picks pages

**Files**
- Create: `web/src/pages/editor-picks.tsx` (list with "더 보기" pagination)
- Create: `web/src/pages/editor-pick-detail.tsx` (full render)

Add default thumbnail `web/public/wl-default-thumb.png` (copy an existing wl-* asset if present, else placeholder).

### Task 19: Admin user role management page

**Files**
- Create: `web/src/pages/admin-users.tsx`

Reuses the `/admin/coins` search pattern. Search → display current role → dropdown → memo → confirm. Shows recent role-change history.

### Task 20: Header updates + router updates

**Files**
- Modify: `web/src/components/header.tsx` (add "에디터 픽", "발행하기")
- Modify: `web/src/App.tsx` (register new routes; guard editor/admin routes)

---

## Phase 6 — QA

### Task 21: Integration tests

**Files**
- Create: `server/integration/editor_post_test.go`
- Create: `server/integration/role_change_test.go`
- Modify: `server/integration/api_integration_test.go` — extend sitemap test to cover editor posts.

### Task 22: Full backend build + test pass

```bash
cd server && go build ./... && go test ./... -short -race
```

Resolve any regressions. Run integration tests separately:

```bash
cd server && go test ./integration/... -run Editor
```

### Task 23: Frontend type check

```bash
cd web && pnpm install && pnpm build
```

### Task 24: Commit phases

One commit per phase. Conventional commit prefix.
