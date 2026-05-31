package integration

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"ota/domain/collector"
	"ota/storage"
)

// MIN_REFERENCES filter coverage. The same source-count gate must apply
// consistently across user-facing list endpoints, email delivery, and the
// sitemap, while explicitly NOT filtering:
//   - GetContextItemByID (direct-link / old-email-link access)
//   - GetHistoryForUser  (already-delivered items)
//
// Fixture: a single successful collection run with four topics whose source
// arrays have lengths 0 (NULL), 0 (empty), 1, and 3. The pipeline guarantees
// 0-source topics never reach the DB in production, but the filter must
// still treat them as below the threshold for any minRefs >= 1.

type minRefsFixture struct {
	runID         uuid.UUID
	itemNullSrc   uuid.UUID // sources column is SQL NULL
	itemZeroSrc   uuid.UUID // sources = '[]'
	itemOneSrc    uuid.UUID // sources = ['a']
	itemThreeSrc  uuid.UUID // sources = ['a','b','c']
}

// seedMinRefsData inserts the run + 4 items. created_at increases left-to-right
// so newest-first ordering is deterministic for list assertions.
func seedMinRefsData(t *testing.T, db *TestDB) minRefsFixture {
	t.Helper()
	ctx := context.Background()

	runID := uuid.New()
	repo := storage.NewCollectorRepository(db.Pool)
	if err := repo.CreateRun(ctx, collector.CollectionRun{
		ID:        runID,
		StartedAt: time.Now().UTC(),
		Status:    collector.RunStatusRunning,
	}); err != nil {
		t.Fatalf("create run: %v", err)
	}
	if err := repo.CompleteRun(ctx, runID, collector.RunStatusSuccess, nil, nil); err != nil {
		t.Fatalf("complete run: %v", err)
	}

	f := minRefsFixture{
		runID:        runID,
		itemNullSrc:  uuid.New(),
		itemZeroSrc:  uuid.New(),
		itemOneSrc:   uuid.New(),
		itemThreeSrc: uuid.New(),
	}

	base := time.Now().UTC()
	rows := []struct {
		id        uuid.UUID
		rank      int
		topic     string
		sourcesQ  string // raw SQL fragment for the sources column
		createdAt time.Time
	}{
		{f.itemNullSrc, 1, "삼성전자 NULL 출처", "NULL", base},
		{f.itemZeroSrc, 2, "삼성전자 빈 출처", "'[]'::jsonb", base.Add(1 * time.Minute)},
		{f.itemOneSrc, 3, "삼성전자 단일 출처", `'["https://a.example/1"]'::jsonb`, base.Add(2 * time.Minute)},
		{f.itemThreeSrc, 4, "삼성전자 다중 출처", `'["https://a.example/1","https://b.example/2","https://c.example/3"]'::jsonb`, base.Add(3 * time.Minute)},
	}

	for _, r := range rows {
		// We build the SQL inline because the JSONB literal varies per row. All
		// parameter values are still bound; only the sources column expression
		// is interpolated from the closed set above.
		q := `
			INSERT INTO context_items
			    (id, collection_run_id, category, brain_category, rank,
			     topic, summary, detail, details, buzz_score, sources, priority, created_at)
			VALUES ($1, $2, 'general', NULL, $3, $4, '요약', '본문', '[]', 10, ` + r.sourcesQ + `, 'none', $5)
		`
		if _, err := db.Pool.Exec(ctx, q, r.id, f.runID, r.rank, r.topic, r.createdAt); err != nil {
			t.Fatalf("insert %q: %v", r.topic, err)
		}
	}
	return f
}

func ids(items []collector.TopicPreview) map[uuid.UUID]bool {
	m := make(map[uuid.UUID]bool, len(items))
	for _, it := range items {
		m[it.ID] = true
	}
	return m
}

// ---------- HistoryRepository ----------

func TestMinRefs_HistoryRepo_GetRecentTopics(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, "context_items", "collection_runs")
	f := seedMinRefsData(t, db)

	cases := []struct {
		minRefs       int
		wantContains  []uuid.UUID
		wantExcludes  []uuid.UUID
	}{
		{0, []uuid.UUID{f.itemNullSrc, f.itemZeroSrc, f.itemOneSrc, f.itemThreeSrc}, nil},
		{1, []uuid.UUID{f.itemOneSrc, f.itemThreeSrc}, []uuid.UUID{f.itemNullSrc, f.itemZeroSrc}},
		{2, []uuid.UUID{f.itemThreeSrc}, []uuid.UUID{f.itemNullSrc, f.itemZeroSrc, f.itemOneSrc}},
		{99, nil, []uuid.UUID{f.itemNullSrc, f.itemZeroSrc, f.itemOneSrc, f.itemThreeSrc}},
	}
	for _, tc := range cases {
		repo := storage.NewHistoryRepository(db.Pool, tc.minRefs)
		got, err := repo.GetRecentTopics(context.Background(), 50)
		if err != nil {
			t.Fatalf("minRefs=%d: %v", tc.minRefs, err)
		}
		gotIDs := ids(got)
		for _, want := range tc.wantContains {
			if !gotIDs[want] {
				t.Errorf("minRefs=%d: missing expected id %s", tc.minRefs, want)
			}
		}
		for _, want := range tc.wantExcludes {
			if gotIDs[want] {
				t.Errorf("minRefs=%d: unexpected id %s present", tc.minRefs, want)
			}
		}
	}
}

func TestMinRefs_HistoryRepo_GetLatestRunTopics(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, "context_items", "collection_runs")
	f := seedMinRefsData(t, db)

	repo := storage.NewHistoryRepository(db.Pool, 2)
	got, err := repo.GetLatestRunTopics(context.Background())
	if err != nil {
		t.Fatalf("get latest: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("minRefs=2: expected 1 item, got %d", len(got))
	}
	if got[0].ID != f.itemThreeSrc {
		t.Errorf("expected three-source item, got %s", got[0].ID)
	}
}

func TestMinRefs_HistoryRepo_GetAllTopics(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, "context_items", "collection_runs")
	f := seedMinRefsData(t, db)
	ctx := context.Background()

	// Unfiltered (minRefs=2)
	repo := storage.NewHistoryRepository(db.Pool, 2)
	got, _, err := repo.GetAllTopics(ctx, "", "", 50, 0)
	if err != nil {
		t.Fatalf("get all: %v", err)
	}
	if len(got) != 1 || got[0].ID != f.itemThreeSrc {
		t.Errorf("minRefs=2 no filter: expected only three-source item, got %d (%v)", len(got), ids(got))
	}

	// Filtered by category ("general") still applies minRefs.
	got2, _, err := repo.GetAllTopics(ctx, "category", "general", 50, 0)
	if err != nil {
		t.Fatalf("get all category: %v", err)
	}
	if len(got2) != 1 || got2[0].ID != f.itemThreeSrc {
		t.Errorf("minRefs=2 + category filter: expected only three-source, got %d", len(got2))
	}

	// Category that doesn't match any rows -> empty.
	got3, _, err := repo.GetAllTopics(ctx, "category", "nonexistent", 50, 0)
	if err != nil {
		t.Fatalf("get all nonexistent category: %v", err)
	}
	if len(got3) != 0 {
		t.Errorf("nonexistent category: expected 0 rows, got %d", len(got3))
	}
}

func TestMinRefs_HistoryRepo_SearchContextItems(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, "context_items", "collection_runs")
	f := seedMinRefsData(t, db)

	// All 4 fixture rows match "삼성전자" by title. minRefs=2 drops three.
	repo := storage.NewHistoryRepository(db.Pool, 2)
	got, _, err := repo.SearchContextItems(context.Background(), "삼성전자", 50, 0)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(got) != 1 || got[0].ID != f.itemThreeSrc {
		t.Errorf("search with minRefs=2: expected only three-source, got %d (%v)", len(got), ids(got))
	}
}

// GetContextItemByID must NEVER apply minRefs — old email links / direct URLs
// to single-source topics still resolve. This is the intentional escape hatch
// (the filter prevents new exposure, not historical access).
func TestMinRefs_HistoryRepo_GetContextItemByID_BypassesFilter(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, "context_items", "collection_runs")
	f := seedMinRefsData(t, db)
	ctx := context.Background()

	for _, minRefs := range []int{0, 1, 2, 99} {
		repo := storage.NewHistoryRepository(db.Pool, minRefs)

		// Single-source: should always resolve regardless of minRefs.
		got, err := repo.GetContextItemByID(ctx, f.itemOneSrc)
		if err != nil {
			t.Fatalf("minRefs=%d single-source detail: %v", minRefs, err)
		}
		if got == nil {
			t.Errorf("minRefs=%d: single-source topic must remain reachable via direct ID", minRefs)
		}

		// Zero-source: also reachable (we don't retroactively hide).
		got2, err := repo.GetContextItemByID(ctx, f.itemZeroSrc)
		if err != nil {
			t.Fatalf("minRefs=%d zero-source detail: %v", minRefs, err)
		}
		if got2 == nil {
			t.Errorf("minRefs=%d: zero-source topic must remain reachable via direct ID", minRefs)
		}
	}

	// Unknown ID still returns nil, nil.
	repo := storage.NewHistoryRepository(db.Pool, 1)
	got, err := repo.GetContextItemByID(ctx, uuid.New())
	if err != nil {
		t.Fatalf("unknown id: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for unknown id, got %+v", got)
	}
}

// Personal history (delivery_logs join) must include items the user already
// received by email even if their sources fall below the threshold. We never
// "un-deliver" content from a user's own history.
func TestMinRefs_HistoryRepo_GetHistoryForUser_BypassesFilter(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, "context_items", "collection_runs", "users", "delivery_logs")
	f := seedMinRefsData(t, db)
	ctx := context.Background()

	// Minimal user + one delivery_log for the run. The history JOIN expands
	// one log into N rows = the number of context_items in that run, so the
	// single insert below covers all four fixture items.
	userID := uuid.New()
	if _, err := db.Pool.Exec(ctx, `
		INSERT INTO users (id, kakao_id, nickname, role)
		VALUES ($1, $2, 'tester', 'user')
	`, userID, int64(9_000_000_001)); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if _, err := db.Pool.Exec(ctx, `
		INSERT INTO delivery_logs (id, run_id, user_id, channel, status, created_at, retry_count)
		VALUES ($1, $2, $3, 'email', 'sent', NOW(), 0)
	`, uuid.New(), f.runID, userID); err != nil {
		t.Fatalf("insert delivery log: %v", err)
	}

	// Even with the highest threshold, all 4 items must appear in user history.
	repo := storage.NewHistoryRepository(db.Pool, 99)
	entries, _, err := repo.GetHistoryForUser(ctx, userID.String(), 10, 0)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	count := 0
	for _, e := range entries {
		count += len(e.Items)
	}
	if count != 4 {
		t.Errorf("personal history: expected 4 items regardless of minRefs, got %d", count)
	}
}

// ---------- CollectorServiceAdapter (email delivery) ----------

func TestMinRefs_CollectorAdapter_GetContextItems(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, "context_items", "collection_runs")
	f := seedMinRefsData(t, db)
	ctx := context.Background()

	// minRefs=0 → all four.
	adapter0 := storage.NewCollectorServiceAdapter(db.Pool, 0)
	all, err := adapter0.GetContextItems(ctx, f.runID)
	if err != nil {
		t.Fatalf("get items minRefs=0: %v", err)
	}
	if len(all) != 4 {
		t.Errorf("minRefs=0: expected 4 items, got %d", len(all))
	}

	// minRefs=2 → only three-source survives, so the email body never includes
	// the others.
	adapter2 := storage.NewCollectorServiceAdapter(db.Pool, 2)
	gated, err := adapter2.GetContextItems(ctx, f.runID)
	if err != nil {
		t.Fatalf("get items minRefs=2: %v", err)
	}
	if len(gated) != 1 || gated[0].ID != f.itemThreeSrc {
		t.Errorf("minRefs=2: expected only three-source item, got %d (ids=%v)", len(gated), itemIDs(gated))
	}

	// Negative input clamps to 0 (defensive).
	adapterNeg := storage.NewCollectorServiceAdapter(db.Pool, -5)
	negAll, err := adapterNeg.GetContextItems(ctx, f.runID)
	if err != nil {
		t.Fatalf("get items minRefs=-5: %v", err)
	}
	if len(negAll) != 4 {
		t.Errorf("minRefs=-5 should clamp to 0 and return all 4, got %d", len(negAll))
	}
}

func itemIDs(items []collector.ContextItem) []uuid.UUID {
	out := make([]uuid.UUID, len(items))
	for i, it := range items {
		out[i] = it.ID
	}
	return out
}

// ---------- SitemapRepository ----------

func TestMinRefs_SitemapRepo_GetAllTopicRows(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, "context_items", "collection_runs")
	f := seedMinRefsData(t, db)
	ctx := context.Background()

	cases := []struct {
		minRefs int
		want    int
	}{
		{0, 4},
		{1, 2},
		{2, 1},
		{99, 0},
	}
	for _, tc := range cases {
		repo := storage.NewSitemapRepository(db.Pool, tc.minRefs)
		got, err := repo.GetAllTopicRows(ctx)
		if err != nil {
			t.Fatalf("minRefs=%d: %v", tc.minRefs, err)
		}
		if len(got) != tc.want {
			t.Errorf("sitemap minRefs=%d: expected %d rows, got %d", tc.minRefs, tc.want, len(got))
		}
	}

	// When the filter is active, the surviving rows must be the multi-source
	// one — never the single-source topic that prompted this feature.
	repo := storage.NewSitemapRepository(db.Pool, 2)
	got, err := repo.GetAllTopicRows(ctx)
	if err != nil {
		t.Fatalf("sitemap minRefs=2: %v", err)
	}
	if len(got) != 1 || got[0].ID != f.itemThreeSrc.String() {
		t.Errorf("sitemap minRefs=2: expected only three-source row, got %v", got)
	}
}
