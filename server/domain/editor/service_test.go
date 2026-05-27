package editor

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"ota/domain/user"
)

// fakeRepo implements Repository for service tests.
type fakeRepo struct {
	mu    sync.Mutex
	posts map[string]Post
	seq   int
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{posts: make(map[string]Post)}
}

func (r *fakeRepo) Create(_ context.Context, p Post) (Post, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seq++
	p.ID = "post-" + itoa(r.seq)
	p.CreatedAt = time.Now()
	p.UpdatedAt = p.CreatedAt
	r.posts[p.ID] = p
	return p, nil
}
func (r *fakeRepo) Update(_ context.Context, p Post) (Post, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.posts[p.ID]; !ok {
		return Post{}, ErrPostNotFound
	}
	p.UpdatedAt = time.Now()
	r.posts[p.ID] = p
	return p, nil
}
func (r *fakeRepo) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.posts[id]; !ok {
		return ErrPostNotFound
	}
	delete(r.posts, id)
	return nil
}
func (r *fakeRepo) FindByID(_ context.Context, id string) (Post, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if p, ok := r.posts[id]; ok {
		return p, nil
	}
	return Post{}, ErrPostNotFound
}
func (r *fakeRepo) FindDraftByAuthor(_ context.Context, authorID string) (Post, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, p := range r.posts {
		if p.AuthorID == authorID && p.Status == StatusDraft {
			return p, nil
		}
	}
	return Post{}, ErrPostNotFound
}
func (r *fakeRepo) ListByAuthor(_ context.Context, authorID string) ([]Post, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []Post
	for _, p := range r.posts {
		if p.AuthorID == authorID {
			out = append(out, p)
		}
	}
	return out, nil
}
func (r *fakeRepo) ListAllForAdmin(_ context.Context) ([]Post, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Post, 0, len(r.posts))
	for _, p := range r.posts {
		out = append(out, p)
	}
	return out, nil
}
func (r *fakeRepo) ListPublishedCards(context.Context, int, int) ([]PublicCard, error) {
	return nil, errors.New("not used")
}
func (r *fakeRepo) GetPublishedByID(context.Context, string) (PublicPost, error) {
	return PublicPost{}, errors.New("not used")
}
func (r *fakeRepo) CountPublished(context.Context) (int, error) { return 0, nil }
func (r *fakeRepo) SearchPublishedCards(context.Context, string, int, int) ([]PublicCard, bool, error) {
	return nil, false, nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var out []byte
	for n > 0 {
		out = append([]byte{byte('0' + n%10)}, out...)
		n /= 10
	}
	return string(out)
}

// ─── Tests ──────────────────────────────────────────────────────────────────

func TestService_Create_RequiresTitle(t *testing.T) {
	svc := NewService(newFakeRepo())
	_, err := svc.Create(context.Background(), CreateParams{
		AuthorID:    "u-1",
		Title:       "   ",
		ContentHTML: "<p>x</p>",
		Status:      StatusDraft,
	})
	if !errors.Is(err, ErrTitleRequired) {
		t.Fatalf("err = %v, want ErrTitleRequired", err)
	}
}

func TestService_Create_RejectsEmptyContent(t *testing.T) {
	svc := NewService(newFakeRepo())
	_, err := svc.Create(context.Background(), CreateParams{
		AuthorID:    "u-1",
		Title:       "hi",
		ContentHTML: "<p>   </p>",
		Status:      StatusDraft,
	})
	if !errors.Is(err, ErrContentEmpty) {
		t.Fatalf("err = %v, want ErrContentEmpty", err)
	}
}

func TestService_Create_InvalidStatus(t *testing.T) {
	svc := NewService(newFakeRepo())
	_, err := svc.Create(context.Background(), CreateParams{
		AuthorID:    "u-1",
		Title:       "hi",
		ContentHTML: "<p>x</p>",
		Status:      "pending",
	})
	if !errors.Is(err, ErrInvalidStatus) {
		t.Fatalf("err = %v, want ErrInvalidStatus", err)
	}
}

func TestService_Create_PublishedSetsPublishedAt(t *testing.T) {
	repo := newFakeRepo()
	fixed := time.Date(2026, 5, 13, 9, 0, 0, 0, time.UTC)
	svc := NewService(repo).withNow(func() time.Time { return fixed })

	p, err := svc.Create(context.Background(), CreateParams{
		AuthorID:    "u-1",
		Title:       "hello",
		ContentHTML: `<p>hello world</p><img src="/x.png">`,
		Status:      StatusPublished,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if p.PublishedAt == nil || !p.PublishedAt.Equal(fixed) {
		t.Errorf("PublishedAt = %v, want %v", p.PublishedAt, fixed)
	}
	if p.FirstImageURL == nil || *p.FirstImageURL != "/x.png" {
		t.Errorf("FirstImageURL = %v, want /x.png", p.FirstImageURL)
	}
	if !strings.Contains(p.ContentText, "hello world") {
		t.Errorf("ContentText excerpt missing: %q", p.ContentText)
	}
}

func TestService_Create_DraftUpsertsExisting(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	ctx := context.Background()

	first, err := svc.Create(ctx, CreateParams{
		AuthorID: "u-1", Title: "v1", ContentHTML: "<p>v1</p>", Status: StatusDraft,
	})
	if err != nil {
		t.Fatalf("first Create: %v", err)
	}

	second, err := svc.Create(ctx, CreateParams{
		AuthorID: "u-1", Title: "v2", ContentHTML: "<p>v2</p>", Status: StatusDraft,
	})
	if err != nil {
		t.Fatalf("second Create: %v", err)
	}

	if second.ID != first.ID {
		t.Errorf("expected upsert to keep same ID, got %s vs %s", second.ID, first.ID)
	}
	if second.Title != "v2" || !strings.Contains(second.ContentHTML, "v2") {
		t.Errorf("draft not overwritten: %+v", second)
	}

	all, _ := repo.ListByAuthor(ctx, "u-1")
	drafts := 0
	for _, p := range all {
		if p.Status == StatusDraft {
			drafts++
		}
	}
	if drafts != 1 {
		t.Errorf("expected 1 draft after upsert, got %d", drafts)
	}
}

func TestService_Create_DraftDoesNotTouchOtherAuthors(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	ctx := context.Background()

	a, _ := svc.Create(ctx, CreateParams{AuthorID: "u-1", Title: "a", ContentHTML: "<p>a</p>", Status: StatusDraft})
	b, _ := svc.Create(ctx, CreateParams{AuthorID: "u-2", Title: "b", ContentHTML: "<p>b</p>", Status: StatusDraft})

	if a.ID == b.ID {
		t.Fatalf("two authors must have distinct draft rows")
	}
	all, _ := repo.ListAllForAdmin(ctx)
	if len(all) != 2 {
		t.Errorf("expected 2 drafts total, got %d", len(all))
	}
}

func TestService_Create_PublishedDoesNotOverwriteDraft(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	ctx := context.Background()

	draft, _ := svc.Create(ctx, CreateParams{AuthorID: "u-1", Title: "d", ContentHTML: "<p>d</p>", Status: StatusDraft})
	pub, _ := svc.Create(ctx, CreateParams{AuthorID: "u-1", Title: "p", ContentHTML: "<p>p</p>", Status: StatusPublished})

	if pub.ID == draft.ID {
		t.Errorf("published create must not touch the draft row")
	}
	got, err := repo.FindByID(ctx, draft.ID)
	if err != nil || got.Status != StatusDraft || got.Title != "d" {
		t.Errorf("draft should remain intact, got %+v err=%v", got, err)
	}
}

func TestService_Update_OwnerCanEdit(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	created, _ := svc.Create(context.Background(), CreateParams{
		AuthorID: "u-1", Title: "v1", ContentHTML: "<p>v1</p>", Status: StatusDraft,
	})

	updated, err := svc.Update(context.Background(), created.ID, "u-1", user.RoleEditor, UpdateParams{
		Title: "v2", ContentHTML: "<p>v2</p>", Status: StatusPublished,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Title != "v2" || updated.Status != StatusPublished {
		t.Errorf("update did not persist: %+v", updated)
	}
	if updated.PublishedAt == nil {
		t.Error("PublishedAt should be set after publish")
	}
}

func TestService_Update_StrangerBlocked(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	created, _ := svc.Create(context.Background(), CreateParams{
		AuthorID: "u-1", Title: "v1", ContentHTML: "<p>v1</p>", Status: StatusDraft,
	})

	_, err := svc.Update(context.Background(), created.ID, "u-2", user.RoleEditor, UpdateParams{
		Title: "x", ContentHTML: "<p>x</p>", Status: StatusDraft,
	})
	if !errors.Is(err, ErrNotAuthorized) {
		t.Fatalf("err = %v, want ErrNotAuthorized", err)
	}
}

func TestService_Update_AdminCanEditOthers(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	created, _ := svc.Create(context.Background(), CreateParams{
		AuthorID: "u-1", Title: "v1", ContentHTML: "<p>v1</p>", Status: StatusDraft,
	})

	updated, err := svc.Update(context.Background(), created.ID, "admin-1", user.RoleAdmin, UpdateParams{
		Title: "moderated", ContentHTML: "<p>moderated</p>", Status: StatusDraft,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.AuthorID != "u-1" {
		t.Errorf("AuthorID changed unexpectedly: %s", updated.AuthorID)
	}
}

func TestService_Update_DraftClearsPublishedAt(t *testing.T) {
	repo := newFakeRepo()
	fixed := time.Date(2026, 5, 13, 9, 0, 0, 0, time.UTC)
	svc := NewService(repo).withNow(func() time.Time { return fixed })

	created, _ := svc.Create(context.Background(), CreateParams{
		AuthorID: "u-1", Title: "t", ContentHTML: "<p>x</p>", Status: StatusPublished,
	})
	if created.PublishedAt == nil {
		t.Fatal("expected published_at set after publish")
	}

	updated, err := svc.Update(context.Background(), created.ID, "u-1", user.RoleEditor, UpdateParams{
		Title: "t", ContentHTML: "<p>x</p>", Status: StatusDraft,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.PublishedAt != nil {
		t.Errorf("PublishedAt should clear on draft, got %v", updated.PublishedAt)
	}
}

func TestService_Delete_RequiresOwnerOrAdmin(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	created, _ := svc.Create(context.Background(), CreateParams{
		AuthorID: "u-1", Title: "t", ContentHTML: "<p>x</p>", Status: StatusDraft,
	})

	if err := svc.Delete(context.Background(), created.ID, "u-2", user.RoleEditor); !errors.Is(err, ErrNotAuthorized) {
		t.Fatalf("non-owner err = %v, want ErrNotAuthorized", err)
	}
	if err := svc.Delete(context.Background(), created.ID, "u-1", user.RoleEditor); err != nil {
		t.Fatalf("owner delete err = %v", err)
	}
}

func TestService_ListForCaller(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	_, _ = svc.Create(context.Background(), CreateParams{AuthorID: "u-1", Title: "a", ContentHTML: "<p>a</p>", Status: StatusDraft})
	_, _ = svc.Create(context.Background(), CreateParams{AuthorID: "u-2", Title: "b", ContentHTML: "<p>b</p>", Status: StatusDraft})

	editorOwn, err := svc.ListForCaller(context.Background(), "u-1")
	if err != nil {
		t.Fatalf("editor list: %v", err)
	}
	if len(editorOwn) != 1 || editorOwn[0].AuthorID != "u-1" {
		t.Errorf("editor sees only own, got %+v", editorOwn)
	}

	// Admins are intentionally scoped to their own posts here too — the
	// endpoint is "my posts", not "all posts". Cross-author admin views must
	// use a dedicated admin endpoint.
	adminOwn, err := svc.ListForCaller(context.Background(), "admin-1")
	if err != nil {
		t.Fatalf("admin list: %v", err)
	}
	if len(adminOwn) != 0 {
		t.Errorf("admin should only see own posts (0), got %d", len(adminOwn))
	}
}
