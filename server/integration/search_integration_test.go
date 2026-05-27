package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"ota/domain/collector"
	"ota/storage"
)

// seedSearchData inserts a successful collection run plus context_items covering
// Korean title, English title, and body-only matches. Returns the run ID for
// caller assertions and item IDs by their primary search keyword.
func seedSearchData(t *testing.T, db *TestDB) map[string]uuid.UUID {
	t.Helper()
	ctx := context.Background()

	runID := uuid.New()
	if err := storage.NewCollectorRepository(db.Pool).CreateRun(ctx, collector.CollectionRun{
		ID:        runID,
		StartedAt: time.Now().UTC(),
		Status:    collector.RunStatusRunning,
	}); err != nil {
		t.Fatalf("create run: %v", err)
	}
	if err := storage.NewCollectorRepository(db.Pool).CompleteRun(ctx, runID, collector.RunStatusSuccess, nil, nil); err != nil {
		t.Fatalf("complete run: %v", err)
	}

	ids := map[string]uuid.UUID{
		"title-ko":   uuid.New(),
		"title-en":   uuid.New(),
		"summary":    uuid.New(),
		"detail":     uuid.New(),
		"unrelated":  uuid.New(),
		"older-hit":  uuid.New(),
		"newest-hit": uuid.New(),
	}

	// Rows have to be unique on topic for predictable ordering.
	items := []struct {
		id      uuid.UUID
		topic   string
		summary string
		detail  string
		rank    int
	}{
		{ids["title-ko"], "삼성전자 반도체 호황", "관련 요약", "본문 무관", 1},
		{ids["title-en"], "AI breakthrough launch", "no match here", "no match", 2},
		{ids["summary"], "전혀 다른 주제", "삼성전자 언급이 요약에 있음", "본문 무관", 3},
		{ids["detail"], "딴 얘기", "요약도 딴 얘기", "이 본문 안에는 삼성전자 키워드가 있음", 4},
		{ids["unrelated"], "강아지 산책 후기", "강아지 요약", "강아지 본문", 5},
		{ids["older-hit"], "삼성전자 옛 기사", "옛 요약", "옛 본문", 6},
		{ids["newest-hit"], "삼성전자 최신 헤드라인", "최신 요약", "최신 본문", 7},
	}

	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback(ctx)

	// Insert with explicit created_at so we can verify newest-first ordering.
	base := time.Now().UTC()
	for i, it := range items {
		createdAt := base.Add(time.Duration(i) * time.Minute)
		if it.id == ids["older-hit"] {
			createdAt = base.Add(-24 * time.Hour)
		}
		if it.id == ids["newest-hit"] {
			createdAt = base.Add(2 * time.Hour)
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO context_items
			    (id, collection_run_id, category, brain_category, rank,
			     topic, summary, detail, details, buzz_score, sources, priority, created_at)
			VALUES ($1, $2, $3, NULL, $4, $5, $6, $7, '[]', 0, '[]', 'none', $8)
		`, it.id, runID, "general", it.rank, it.topic, it.summary, it.detail, createdAt); err != nil {
			t.Fatalf("insert item %s: %v", it.topic, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}

	return ids
}

func TestSearch_FindsKoreanTitleMatch(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, "context_items", "collection_runs")

	ids := seedSearchData(t, db)
	repo := storage.NewHistoryRepository(db.Pool)

	got, hasMore, err := repo.SearchContextItems(context.Background(), "삼성전자", 10, 0)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if hasMore {
		t.Errorf("expected hasMore=false, got true")
	}
	// 5 rows mention 삼성전자: 3 in title, 1 in summary, 1 in detail.
	if len(got) != 5 {
		t.Fatalf("expected 5 hits, got %d (%v)", len(got), summarize(got))
	}

	// Title matches must precede body-only matches. Verify the first 3 are
	// title rows (newest-hit, title-ko, older-hit) and the last 2 are body.
	titleIDs := map[uuid.UUID]bool{
		ids["title-ko"]:   true,
		ids["newest-hit"]: true,
		ids["older-hit"]:  true,
	}
	for i, item := range got[:3] {
		if !titleIDs[item.ID] {
			t.Errorf("rank %d should be title match, got %q (id=%s)", i, item.Topic, item.ID)
		}
	}
	// Within title tier, newest-hit (now+2h) > title-ko (now) > older-hit (-24h).
	if got[0].ID != ids["newest-hit"] {
		t.Errorf("expected newest-hit first, got %q", got[0].Topic)
	}
	if got[2].ID != ids["older-hit"] {
		t.Errorf("expected older-hit last among title matches, got %q", got[2].Topic)
	}
}

func TestSearch_TitleRanksAboveSummaryRanksAboveDetail(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, "context_items", "collection_runs")

	ids := seedSearchData(t, db)
	repo := storage.NewHistoryRepository(db.Pool)

	// All three rows mention 삼성전자 in different fields. Limit so we capture
	// the ordering deterministically.
	got, _, err := repo.SearchContextItems(context.Background(), "삼성전자", 50, 0)
	if err != nil {
		t.Fatalf("search: %v", err)
	}

	// Locate the title-only, summary-only, and detail-only rows in result order.
	posTitle := indexOf(got, ids["title-ko"])
	posSummary := indexOf(got, ids["summary"])
	posDetail := indexOf(got, ids["detail"])

	if posTitle == -1 || posSummary == -1 || posDetail == -1 {
		t.Fatalf("missing expected hits: title=%d summary=%d detail=%d",
			posTitle, posSummary, posDetail)
	}
	if !(posTitle < posSummary && posSummary < posDetail) {
		t.Errorf("expected title < summary < detail ordering, got %d, %d, %d",
			posTitle, posSummary, posDetail)
	}
}

func TestSearch_EnglishQuery(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, "context_items", "collection_runs")

	ids := seedSearchData(t, db)
	repo := storage.NewHistoryRepository(db.Pool)

	got, _, err := repo.SearchContextItems(context.Background(), "AI", 10, 0)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 hit for 'AI', got %d", len(got))
	}
	if got[0].ID != ids["title-en"] {
		t.Errorf("expected title-en, got %q", got[0].Topic)
	}
}

func TestSearch_CaseInsensitive(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, "context_items", "collection_runs")

	seedSearchData(t, db)
	repo := storage.NewHistoryRepository(db.Pool)

	gotLower, _, err := repo.SearchContextItems(context.Background(), "ai", 10, 0)
	if err != nil {
		t.Fatalf("search lower: %v", err)
	}
	gotUpper, _, err := repo.SearchContextItems(context.Background(), "AI", 10, 0)
	if err != nil {
		t.Fatalf("search upper: %v", err)
	}
	if len(gotLower) != len(gotUpper) || len(gotLower) == 0 {
		t.Errorf("case-insensitive search should match: lower=%d upper=%d",
			len(gotLower), len(gotUpper))
	}
}

func TestSearch_Pagination(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, "context_items", "collection_runs")

	seedSearchData(t, db)
	repo := storage.NewHistoryRepository(db.Pool)

	page1, hasMore1, err := repo.SearchContextItems(context.Background(), "삼성전자", 2, 0)
	if err != nil {
		t.Fatalf("page1: %v", err)
	}
	if !hasMore1 {
		t.Errorf("page1 should have more")
	}
	if len(page1) != 2 {
		t.Fatalf("page1 size = %d, want 2", len(page1))
	}

	page2, hasMore2, err := repo.SearchContextItems(context.Background(), "삼성전자", 2, 2)
	if err != nil {
		t.Fatalf("page2: %v", err)
	}
	if !hasMore2 {
		t.Errorf("page2 should have more (5 total, page size 2, offset 2 → 1 remaining)")
	}
	if len(page2) != 2 {
		t.Fatalf("page2 size = %d, want 2", len(page2))
	}

	page3, hasMore3, err := repo.SearchContextItems(context.Background(), "삼성전자", 2, 4)
	if err != nil {
		t.Fatalf("page3: %v", err)
	}
	if hasMore3 {
		t.Errorf("page3 should have no more (only 1 row left)")
	}
	if len(page3) != 1 {
		t.Fatalf("page3 size = %d, want 1", len(page3))
	}

	page4, hasMore4, err := repo.SearchContextItems(context.Background(), "삼성전자", 2, 5)
	if err != nil {
		t.Fatalf("page4: %v", err)
	}
	if hasMore4 {
		t.Errorf("page4 should have no more")
	}
	if len(page4) != 0 {
		t.Fatalf("page4 size = %d, want 0", len(page4))
	}

	// Verify no overlap across consecutive pages.
	seen := make(map[uuid.UUID]bool)
	for _, p := range [][]collector.TopicPreview{page1, page2, page3} {
		for _, item := range p {
			if seen[item.ID] {
				t.Errorf("duplicate id across pages: %s", item.ID)
			}
			seen[item.ID] = true
		}
	}
}

func TestSearch_NoMatches(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, "context_items", "collection_runs")

	seedSearchData(t, db)
	repo := storage.NewHistoryRepository(db.Pool)

	got, hasMore, err := repo.SearchContextItems(context.Background(), "존재하지않는키워드xyz", 10, 0)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if hasMore {
		t.Errorf("expected hasMore=false")
	}
	if len(got) != 0 {
		t.Errorf("expected 0 hits, got %d", len(got))
	}
}

func TestSearch_ExcludesNonSuccessfulRuns(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, "context_items", "collection_runs")

	ctx := context.Background()

	// Successful run with a 삼성전자 row.
	successRunID := uuid.New()
	repo := storage.NewCollectorRepository(db.Pool)
	if err := repo.CreateRun(ctx, collector.CollectionRun{ID: successRunID, StartedAt: time.Now().UTC(), Status: collector.RunStatusRunning}); err != nil {
		t.Fatal(err)
	}
	if err := repo.CompleteRun(ctx, successRunID, collector.RunStatusSuccess, nil, nil); err != nil {
		t.Fatal(err)
	}
	successItemID := uuid.New()
	if _, err := db.Pool.Exec(ctx, `
		INSERT INTO context_items (id, collection_run_id, category, rank, topic, summary, detail, details, buzz_score, sources, priority)
		VALUES ($1, $2, 'general', 1, '삼성전자 정상', '', '', '[]', 0, '[]', 'none')
	`, successItemID, successRunID); err != nil {
		t.Fatal(err)
	}

	// Failed run with a 삼성전자 row that must NOT appear in search.
	failedRunID := uuid.New()
	failMsg := "boom"
	if err := repo.CreateRun(ctx, collector.CollectionRun{ID: failedRunID, StartedAt: time.Now().UTC(), Status: collector.RunStatusRunning}); err != nil {
		t.Fatal(err)
	}
	if err := repo.CompleteRun(ctx, failedRunID, collector.RunStatusFailed, &failMsg, nil); err != nil {
		t.Fatal(err)
	}
	failedItemID := uuid.New()
	if _, err := db.Pool.Exec(ctx, `
		INSERT INTO context_items (id, collection_run_id, category, rank, topic, summary, detail, details, buzz_score, sources, priority)
		VALUES ($1, $2, 'general', 1, '삼성전자 실패', '', '', '[]', 0, '[]', 'none')
	`, failedItemID, failedRunID); err != nil {
		t.Fatal(err)
	}

	histRepo := storage.NewHistoryRepository(db.Pool)
	got, _, err := histRepo.SearchContextItems(ctx, "삼성전자", 10, 0)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 hit (success run only), got %d", len(got))
	}
	if got[0].ID != successItemID {
		t.Errorf("expected success run item, got %q", got[0].Topic)
	}
}

func indexOf(items []collector.TopicPreview, id uuid.UUID) int {
	for i, it := range items {
		if it.ID == id {
			return i
		}
	}
	return -1
}

func summarize(items []collector.TopicPreview) string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.Topic
	}
	return strings.Join(out, " | ")
}
