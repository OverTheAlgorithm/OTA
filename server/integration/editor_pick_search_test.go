package integration

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"ota/domain/editor"
	"ota/storage"
)

// seedEditorPostsForSearch inserts a small mix of published / draft editor
// posts spanning title-only, body-only, and unrelated content so the search
// repo can be exercised for ranking, recency, and visibility rules.
func seedEditorPostsForSearch(t *testing.T, db *TestDB) map[string]string {
	t.Helper()
	ctx := context.Background()

	// We need an author row; users table has FKs from editor_posts.author_id.
	userRepo := storage.NewUserRepository(db.Pool)
	author, err := userRepo.UpsertByKakaoID(ctx, 90001, "search-author@x.com", "search-author", "")
	if err != nil {
		t.Fatalf("seed author: %v", err)
	}

	repo := storage.NewEditorRepository(db.Pool)
	ids := map[string]string{}

	mkPub := func(label, title, body string, publishedAt time.Time) {
		p, err := repo.Create(ctx, editor.Post{
			AuthorID:    author.ID,
			Title:       title,
			ContentHTML: "<p>" + body + "</p>",
			ContentText: body,
			Status:      editor.StatusPublished,
			PublishedAt: &publishedAt,
		})
		if err != nil {
			t.Fatalf("seed %s: %v", label, err)
		}
		ids[label] = p.ID
	}

	now := time.Now().UTC()
	mkPub("title-newest", "삼성전자 최신 분석", "본문은 무관해요", now)
	mkPub("title-old", "삼성전자 옛 글", "옛 본문", now.Add(-48*time.Hour))
	mkPub("body-only", "전혀 다른 제목", "본문 안에 삼성전자 키워드가 있음", now.Add(-time.Hour))
	mkPub("unrelated", "강아지 산책", "강아지 본문", now.Add(-30*time.Minute))

	// Draft with matching content — must never appear.
	if _, err := repo.Create(ctx, editor.Post{
		AuthorID:    author.ID,
		Title:       "삼성전자 초안",
		ContentHTML: "<p>draft</p>",
		ContentText: "draft body",
		Status:      editor.StatusDraft,
	}); err != nil {
		t.Fatalf("seed draft: %v", err)
	}

	return ids
}

func TestEditorSearch_RanksTitleAboveBody(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, "editor_posts", "users")

	ids := seedEditorPostsForSearch(t, db)
	repo := storage.NewEditorRepository(db.Pool)

	got, hasMore, err := repo.SearchPublishedCards(context.Background(), "삼성전자", 10, 0)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if hasMore {
		t.Errorf("expected hasMore=false")
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 hits (2 title + 1 body), got %d", len(got))
	}

	wantOrder := []string{ids["title-newest"], ids["title-old"], ids["body-only"]}
	for i, want := range wantOrder {
		if got[i].ID != want {
			t.Errorf("position %d: got %q (%s), want %q", i, got[i].Title, got[i].ID, want)
		}
	}
}

func TestEditorSearch_SkipsDrafts(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, "editor_posts", "users")

	seedEditorPostsForSearch(t, db)
	repo := storage.NewEditorRepository(db.Pool)

	got, _, err := repo.SearchPublishedCards(context.Background(), "초안", 10, 0)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("draft titled '삼성전자 초안' should not surface; got %d hits", len(got))
	}
}

func TestEditorSearch_Pagination(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, "editor_posts", "users")

	seedEditorPostsForSearch(t, db)
	repo := storage.NewEditorRepository(db.Pool)

	page1, hasMore1, err := repo.SearchPublishedCards(context.Background(), "삼성전자", 2, 0)
	if err != nil {
		t.Fatalf("page1: %v", err)
	}
	if !hasMore1 || len(page1) != 2 {
		t.Errorf("page1: len=%d hasMore=%v, want 2/true", len(page1), hasMore1)
	}

	page2, hasMore2, err := repo.SearchPublishedCards(context.Background(), "삼성전자", 2, 2)
	if err != nil {
		t.Fatalf("page2: %v", err)
	}
	if hasMore2 || len(page2) != 1 {
		t.Errorf("page2: len=%d hasMore=%v, want 1/false", len(page2), hasMore2)
	}

	// No overlap.
	if page1[0].ID == page2[0].ID || page1[1].ID == page2[0].ID {
		t.Errorf("page1/page2 IDs overlap")
	}
}

func TestEditorSearch_NoMatches(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, "editor_posts", "users")

	seedEditorPostsForSearch(t, db)
	repo := storage.NewEditorRepository(db.Pool)

	got, hasMore, err := repo.SearchPublishedCards(context.Background(), "존재하지않는키워드_"+uuid.NewString(), 10, 0)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if hasMore || len(got) != 0 {
		t.Errorf("expected empty result, got len=%d hasMore=%v", len(got), hasMore)
	}
}

func TestEditorSearch_CaseInsensitive(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, "editor_posts", "users")

	ctx := context.Background()
	userRepo := storage.NewUserRepository(db.Pool)
	author, err := userRepo.UpsertByKakaoID(ctx, 91001, "case-author@x.com", "case-author", "")
	if err != nil {
		t.Fatalf("seed author: %v", err)
	}

	repo := storage.NewEditorRepository(db.Pool)
	pub := time.Now().UTC()
	if _, err := repo.Create(ctx, editor.Post{
		AuthorID:    author.ID,
		Title:       "ChatGPT 분석",
		ContentText: "OpenAI keeps shipping",
		ContentHTML: "<p>x</p>",
		Status:      editor.StatusPublished,
		PublishedAt: &pub,
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	lower, _, _ := repo.SearchPublishedCards(ctx, "chatgpt", 10, 0)
	upper, _, _ := repo.SearchPublishedCards(ctx, "CHATGPT", 10, 0)
	if len(lower) != 1 || len(upper) != 1 {
		t.Errorf("case-insensitive match expected: lower=%d upper=%d", len(lower), len(upper))
	}
}
