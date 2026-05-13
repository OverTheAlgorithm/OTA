# Editor Role & Publishing — Design

## Goal

Add an **editor** role between `user` and `admin` so trusted writers can publish rich-text articles ("에디터 픽") that appear in the public navigation, render with TipTap-produced HTML, and are included in the dynamic sitemap.

## Role Hierarchy

`user` < `editor` < `admin`. Admin implicitly has every editor permission.

- Column: `users.role TEXT NOT NULL DEFAULT 'user' CHECK (role IN ('user','editor','admin'))`
- Migration backfills any `NULL` / empty role to `'user'`.
- Helper `user.HasRoleAtLeast(role, min string) bool` centralises the precedence map.
- New middleware `api.RequireRoleMiddleware(userRepo, minRole)` mirrors `AdminMiddleware` — always hits the DB so role changes apply without re-login.
- `AdminMiddleware` stays (semantics unchanged) but is rewritten to delegate to `RequireRoleMiddleware(userRepo, "admin")` to keep one source of truth.

## Role Management UI (Admin)

A new section on `/admin` lets admins promote/demote users.

| Endpoint | Auth | Purpose |
|---|---|---|
| `GET /api/v1/admin/users/search?type=id\|email&q=` | admin | Look up a user |
| `POST /api/v1/admin/users/role` | admin | Change role with memo |
| `GET /api/v1/admin/users/:id/role-history` | admin | Audit trail |

- Self-demotion blocked at handler level.
- Audit table `role_change_logs (id, user_id, before_role, after_role, actor_id, memo, created_at)`.
- Reuses the existing user-search pattern from `AdminCoinHandler`.

## Editor Posts

### Data Model

```sql
CREATE TABLE editor_posts (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  author_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  title           TEXT NOT NULL,
  content_html    TEXT NOT NULL,           -- sanitised TipTap HTML
  content_text    TEXT NOT NULL DEFAULT '', -- plain text for excerpt
  first_image_url TEXT,                     -- cached thumbnail (NULL fallback default)
  status          TEXT NOT NULL DEFAULT 'draft'
                  CHECK (status IN ('draft','published')),
  published_at    TIMESTAMPTZ,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX editor_posts_published_at_idx
  ON editor_posts (status, published_at DESC) WHERE status = 'published';
CREATE INDEX editor_posts_author_idx ON editor_posts (author_id);
```

`first_image_url` and `content_text` are derived from `content_html` on write so list/excerpt queries are cheap.

### Security

- TipTap HTML is sanitised server-side with **bluemonday** before persistence.
- Allowed tags: `p, h1-h6, strong, em, u, s, a(href), ul, ol, li, blockquote, code, pre, img(src,alt), br, span(style:text-align)`.
- `<a>` rels coerced to `nofollow noopener`, target `_blank` allowed.
- `<img src>` whitelisted to relative paths or our own host.
- Field caps: title 1-200, content_html ≤ 100 KB, content_text trimmed to 500 chars for excerpts.

### Image Upload

- `POST /api/v1/editor/upload-image` (editor+, multipart `file`)
- MIME whitelist: `image/jpeg|png|webp|gif`. Magic-byte verification. ≤ 5 MB.
- Stored at `data/images/editor/YYYY/MM/{uuid}.{ext}`, served by the existing `/api/v1/images` static route.
- Returns `{ url: "/api/v1/images/editor/2026/05/uuid.webp" }`.

### CRUD Endpoints

| Method | Path | Auth | Notes |
|---|---|---|---|
| POST | `/api/v1/editor/posts` | editor+ | create (`status` decides draft vs publish) |
| GET | `/api/v1/editor/posts` | editor+ | own posts (admin: all) |
| GET | `/api/v1/editor/posts/:id` | editor+, owner-or-admin | fetch for edit |
| PUT | `/api/v1/editor/posts/:id` | editor+, owner-or-admin | |
| DELETE | `/api/v1/editor/posts/:id` | editor+, owner-or-admin | hard delete (image files left in place) |
| POST | `/api/v1/editor/upload-image` | editor+ | |

### Public Endpoints

| Method | Path | Auth | Notes |
|---|---|---|---|
| GET | `/api/v1/editor-picks?limit=10&offset=0` | public | published only, newest first, returns excerpt+thumbnail |
| GET | `/api/v1/editor-picks/:id` | public | published only, full HTML |

`limit` clamped to `[1, 50]`, default 10.

## Sitemap

`SitemapRepository` gains `GetAllEditorPostEntries`. Handler appends:

- `/editor-picks` static page (already in `staticPages` slice)
- One `/editor-picks/:id` entry per published post, `lastmod = updated_at`.

## Frontend

### Library

- **TipTap v2** with `@tiptap/react`, `@tiptap/starter-kit`, `@tiptap/extension-image`, `@tiptap/extension-link`, `@tiptap/extension-placeholder`, `@tiptap/extension-character-count`.
- Output is HTML; rendered into a div from the already server-sanitised output (no further client-side sanitisation needed — the server is the trust boundary).

### Navigation

- All users + anonymous: "에디터 픽" link in header (desktop and mobile, all routes except `/`).
- Editor or admin: extra "발행하기" button (accent colour, links to `/editor/new`).
- Admin: existing "관리자" link unchanged.

### Pages

| Route | Auth | Purpose |
|---|---|---|
| `/editor-picks` | public | paginated list, 10 per page, "더 보기" button |
| `/editor-picks/:id` | public | full article render |
| `/editor/new` | editor+ | TipTap editor |
| `/editor/edit/:id` | editor+ (owner/admin) | TipTap editor with prefilled content |
| `/admin/users` | admin | role management UI |

### Card Layout (List)

- 96×96 thumbnail on the left (`first_image_url` or `/wl-default-thumb.png`).
- Right side: title (16px bold), excerpt (14px gray, 2 lines clamp), date.
- Border `2px #231815`, rounded, hover opacity.

## YAGNI Decisions

- **Hard delete** (no soft-delete table). Simpler, sitemap auto-clears.
- **No slugs** — UUID URLs. Posts are short-lived "picks", not evergreen SEO.
- **No coin earning** on editor pick views. Topic items remain the only coin source.
- **No drafts list pagination** — editor's own posts query is unpaginated for now (<100 posts/editor expected).
- **No comments / reactions / view counts**.

## Testing

- **Unit** (Go): role helper, sanitiser, body validation, owner-or-admin check.
- **Integration** (testcontainers): editor_posts CRUD, role change + audit insert, sitemap inclusion, image upload path validation.
- **Handler tests** (Go): one per new handler, table-driven, mocked repos.
- **Frontend**: type checks suffice (`tsc -b`). UI flow validated manually.
