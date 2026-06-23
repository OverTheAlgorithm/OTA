package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	"ota/domain/editor"
	"ota/storage"
)

// TestListPublishedCards_DecodesLegacyEntities reproduces the bug where the
// editor-pick list preview rendered raw HTML entities (e.g. &#34;) instead of
// their literal characters. Rows persisted before the Excerpt entity-decode
// fix still hold escaped content_text; the repo must decode on read so old
// posts display correctly without a backfill.
func TestListPublishedCards_DecodesLegacyEntities(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, "editor_posts", "users")

	ctx := context.Background()

	userRepo := storage.NewUserRepository(db.Pool)
	author, err := userRepo.UpsertByKakaoID(ctx, 90101, "decode-author@x.com", "decode-author", "")
	if err != nil {
		t.Fatalf("seed author: %v", err)
	}

	repo := storage.NewEditorRepository(db.Pool)
	publishedAt := time.Now().UTC()

	// Simulate a legacy row: content_text written before the fix, with entities
	// left intact exactly as the old Excerpt produced them.
	legacy := `&#34;2026년 역대급 더위&#34; 뉴스 자막엔 벌써 &#34;기습 폭우&#34; &amp; &#34;태풍 비상&#34;`
	if _, err := repo.Create(ctx, editor.Post{
		AuthorID:    author.ID,
		Title:       "엘니뇨 이야기",
		ContentHTML: "<p>" + legacy + "</p>",
		ContentText: legacy,
		Status:      editor.StatusPublished,
		PublishedAt: &publishedAt,
	}); err != nil {
		t.Fatalf("seed legacy post: %v", err)
	}

	cards, err := repo.ListPublishedCards(ctx, 10, 0)
	if err != nil {
		t.Fatalf("list cards: %v", err)
	}
	if len(cards) != 1 {
		t.Fatalf("expected 1 card, got %d", len(cards))
	}

	got := cards[0].Excerpt
	if strings.Contains(got, "&#34;") || strings.Contains(got, "&amp;") {
		t.Errorf("excerpt still contains raw entities: %q", got)
	}
	want := `"2026년 역대급 더위" 뉴스 자막엔 벌써 "기습 폭우" & "태풍 비상"`
	if got != want {
		t.Errorf("excerpt = %q, want %q", got, want)
	}
}
