package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ulule/limiter/v3/drivers/store/memory"

	"ota/api"
	"ota/api/handler"
	"ota/auth"
	"ota/domain/communitytrend"
	"ota/storage"
)

func TestCommunityTrend_Migration(t *testing.T) {
	db := SetupTestDB(t)
	ctx := context.Background()

	// 14개 ct_ 테이블이 모두 존재하는지
	wantTables := []string{
		"ct_axes", "ct_tags", "ct_communities", "ct_community_tags",
		"ct_tag_daily", "ct_community_daily", "ct_worksheets",
		"ct_robots_status", "ct_robots_transitions", "ct_seen_posts",
		"ct_memes", "ct_meme_candidates", "ct_meme_blacklist", "ct_meme_daily",
	}
	for _, tbl := range wantTables {
		var exists bool
		err := db.Pool.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = $1)`, tbl).Scan(&exists)
		if err != nil {
			t.Fatalf("check table %s: %v", tbl, err)
		}
		if !exists {
			t.Fatalf("expected table %s to exist", tbl)
		}
	}

	// 시드 검증
	var axisCount, commCount, tagCount, attachCount int
	db.Pool.QueryRow(ctx, `SELECT count(*) FROM ct_axes`).Scan(&axisCount)
	db.Pool.QueryRow(ctx, `SELECT count(*) FROM ct_communities`).Scan(&commCount)
	db.Pool.QueryRow(ctx, `SELECT count(*) FROM ct_tags`).Scan(&tagCount)
	db.Pool.QueryRow(ctx, `SELECT count(*) FROM ct_community_tags`).Scan(&attachCount)

	if axisCount != 5 {
		t.Fatalf("expected 5 seeded axes, got %d", axisCount)
	}
	if commCount != 4 {
		t.Fatalf("expected 4 seeded communities, got %d", commCount)
	}
	if tagCount != 6 {
		t.Fatalf("expected 6 seeded meta tags, got %d", tagCount)
	}
	if attachCount < 4 {
		t.Fatalf("expected at least 4 meta-tag attachments, got %d", attachCount)
	}
}

func TestCommunityTrend_CommunityRepo(t *testing.T) {
	db := SetupTestDB(t)
	ctx := context.Background()
	repo := storage.NewCTCommunityRepository(db.Pool)

	// 시드 4개 + 신규 1개 생성
	created, err := repo.Create(ctx, communitytrend.Community{
		Key: "ruliweb", Name: "루리웹", HomeURL: "https://bbs.ruliweb.com", Enabled: true,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.ID == 0 || created.Key != "ruliweb" {
		t.Fatalf("unexpected created community: %+v", created)
	}

	list, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 5 {
		t.Fatalf("expected 5 communities (4 seed + 1), got %d", len(list))
	}

	// 메타태그 부착: 시드 태그 '남성향','진보 성향' id 조회
	var maleID, progID int
	db.Pool.QueryRow(ctx, `SELECT id FROM ct_tags WHERE name='남성향'`).Scan(&maleID)
	db.Pool.QueryRow(ctx, `SELECT id FROM ct_tags WHERE name='진보 성향'`).Scan(&progID)

	if err := repo.SetMetaTags(ctx, created.ID, []int{maleID, progID}); err != nil {
		t.Fatalf("set meta tags: %v", err)
	}
	got, err := repo.GetMetaTags(ctx, created.ID)
	if err != nil {
		t.Fatalf("get meta tags: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 meta tags, got %d", len(got))
	}

	// SetMetaTags는 전체 교체 (1개로 줄임)
	if err := repo.SetMetaTags(ctx, created.ID, []int{maleID}); err != nil {
		t.Fatalf("reset meta tags: %v", err)
	}
	got2, _ := repo.GetMetaTags(ctx, created.ID)
	if len(got2) != 1 {
		t.Fatalf("expected 1 meta tag after replace, got %d", len(got2))
	}

	// Update
	updated, err := repo.Update(ctx, created.ID, "루리웹 커뮤니티", "https://ruliweb.com", false)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Name != "루리웹 커뮤니티" || updated.Enabled != false {
		t.Fatalf("update not applied: %+v", updated)
	}

	// 중복 key 거부
	_, err = repo.Create(ctx, communitytrend.Community{Key: "ruliweb", Name: "dup"})
	if err == nil {
		t.Fatal("expected duplicate key error")
	}

	// Delete + cascade (메타태그도 삭제)
	if err := repo.Delete(ctx, created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	list2, _ := repo.List(ctx)
	if len(list2) != 4 {
		t.Fatalf("expected 4 after delete, got %d", len(list2))
	}
}

func TestCommunityTrend_AxisAndTagRepo(t *testing.T) {
	db := SetupTestDB(t)
	ctx := context.Background()
	axisRepo := storage.NewCTAxisRepository(db.Pool)
	tagRepo := storage.NewCTTagRepository(db.Pool)

	// 시드 축 5개
	axes, err := axisRepo.List(ctx)
	if err != nil {
		t.Fatalf("list axes: %v", err)
	}
	if len(axes) != 5 {
		t.Fatalf("expected 5 seed axes, got %d", len(axes))
	}

	// 신규 축
	newAxis, err := axisRepo.Create(ctx, communitytrend.Axis{Key: "social", Label: "사회논제축", DisplayOrder: 6})
	if err != nil {
		t.Fatalf("create axis: %v", err)
	}

	// 신규 태그 (정밀 명명: '우파 지지' 같은 형태)
	tag, err := tagRepo.Create(ctx, communitytrend.Tag{
		AxisID: newAxis.ID, Name: "지역 격차", Description: "지역 간 불균형 논제", CreatedBy: "admin",
	})
	if err != nil {
		t.Fatalf("create tag: %v", err)
	}
	if tag.ID == 0 {
		t.Fatal("expected non-zero tag id")
	}

	// 같은 축에서 중복 이름 거부
	_, err = tagRepo.Create(ctx, communitytrend.Tag{AxisID: newAxis.ID, Name: "지역 격차", CreatedBy: "admin"})
	if err == nil {
		t.Fatal("expected duplicate (axis,name) error")
	}

	// ListByAxis
	byAxis, err := tagRepo.ListByAxis(ctx, newAxis.ID)
	if err != nil {
		t.Fatalf("list by axis: %v", err)
	}
	if len(byAxis) != 1 {
		t.Fatalf("expected 1 tag in axis, got %d", len(byAxis))
	}

	// List (시드 6 + 신규 1 = 7)
	all, _ := tagRepo.List(ctx)
	if len(all) != 7 {
		t.Fatalf("expected 7 tags, got %d", len(all))
	}

	// Update
	upd, err := tagRepo.Update(ctx, tag.ID, "수도권 집중", "수도권 인구·자원 집중 논제")
	if err != nil {
		t.Fatalf("update tag: %v", err)
	}
	if upd.Name != "수도권 집중" {
		t.Fatalf("update not applied: %+v", upd)
	}

	// Delete
	if err := tagRepo.Delete(ctx, tag.ID); err != nil {
		t.Fatalf("delete tag: %v", err)
	}
	all2, _ := tagRepo.List(ctx)
	if len(all2) != 6 {
		t.Fatalf("expected 6 tags after delete, got %d", len(all2))
	}
}

func TestCommunityTrend_AdminHTTP(t *testing.T) {
	db := SetupTestDB(t)

	svc := communitytrend.NewService(
		storage.NewCTCommunityRepository(db.Pool),
		storage.NewCTTagRepository(db.Pool),
		storage.NewCTAxisRepository(db.Pool),
	)
	wsSvc := communitytrend.NewWorksheetService(storage.NewCTWorksheetRepository(db.Pool))
	adminHandler := handler.NewCommunityTrendAdminHandler(svc, wsSvc)

	gin.SetMode(gin.TestMode)
	jwtManager := auth.NewJWTManager("test-secret")
	router := api.NewRouter("api", "v1", "http://localhost:5173", jwtManager, 10000, memory.NewStore(),
		[]api.RouteModule{
			{GroupName: "admin/community-trend", Handler: adminHandler, Middlewares: []gin.HandlerFunc{}},
		})

	// 커뮤니티 목록 (시드 4)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/admin/community-trend/communities", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("list communities: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var listResp struct {
		Data []communitytrend.Community `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &listResp)
	if len(listResp.Data) != 4 {
		t.Fatalf("expected 4 seed communities, got %d", len(listResp.Data))
	}

	// 커뮤니티 생성
	body, _ := json.Marshal(map[string]any{"key": "mlbpark", "name": "엠팍", "home_url": "https://mlbpark.donga.com"})
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("POST", "/api/v1/admin/community-trend/communities", bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w2, req2)
	if w2.Code != http.StatusCreated {
		t.Fatalf("create community: expected 201, got %d: %s", w2.Code, w2.Body.String())
	}
	var createResp struct {
		Data communitytrend.Community `json:"data"`
	}
	json.Unmarshal(w2.Body.Bytes(), &createResp)
	commID := createResp.Data.ID

	// 잘못된 key 거부
	badBody, _ := json.Marshal(map[string]any{"key": "MLB Park", "name": "x"})
	w3 := httptest.NewRecorder()
	req3, _ := http.NewRequest("POST", "/api/v1/admin/community-trend/communities", bytes.NewReader(badBody))
	req3.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w3, req3)
	if w3.Code != http.StatusBadRequest {
		t.Fatalf("bad key: expected 400, got %d", w3.Code)
	}

	// 태그 목록 (시드 6)
	w4 := httptest.NewRecorder()
	req4, _ := http.NewRequest("GET", "/api/v1/admin/community-trend/tags", nil)
	router.ServeHTTP(w4, req4)
	var tagsResp struct {
		Data []communitytrend.Tag `json:"data"`
	}
	json.Unmarshal(w4.Body.Bytes(), &tagsResp)
	if len(tagsResp.Data) != 6 {
		t.Fatalf("expected 6 seed tags, got %d", len(tagsResp.Data))
	}

	// 메타태그 부착 (첫 2개 태그)
	metaBody, _ := json.Marshal(map[string]any{"tag_ids": []int{tagsResp.Data[0].ID, tagsResp.Data[1].ID}})
	w5 := httptest.NewRecorder()
	req5, _ := http.NewRequest("PUT",
		"/api/v1/admin/community-trend/communities/"+itoa(commID)+"/meta-tags",
		bytes.NewReader(metaBody))
	req5.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w5, req5)
	if w5.Code != http.StatusOK {
		t.Fatalf("set meta tags: expected 200, got %d: %s", w5.Code, w5.Body.String())
	}

	// 목록에서 메타태그 반영 확인
	w6 := httptest.NewRecorder()
	req6, _ := http.NewRequest("GET", "/api/v1/admin/community-trend/communities", nil)
	router.ServeHTTP(w6, req6)
	var listResp2 struct {
		Data []communitytrend.Community `json:"data"`
	}
	json.Unmarshal(w6.Body.Bytes(), &listResp2)
	var found bool
	for _, c := range listResp2.Data {
		if c.ID == commID && len(c.MetaTagIDs) == 2 {
			found = true
		}
	}
	if !found {
		t.Fatal("expected created community to have 2 meta tags")
	}
}

// itoa is a tiny local helper to avoid importing strconv at call sites.
func itoa(n int) string { return fmt.Sprintf("%d", n) }

func TestCommunityTrend_ManualConfirm(t *testing.T) {
	db := SetupTestDB(t)
	ctx := context.Background()
	wsRepo := storage.NewCTWorksheetRepository(db.Pool)
	svc := communitytrend.NewWorksheetService(wsRepo)

	// seed dogdrip community + two seed tags
	var commID, tag1, tag2 int
	db.Pool.QueryRow(ctx, `SELECT id FROM ct_communities WHERE key='dogdrip'`).Scan(&commID)
	db.Pool.QueryRow(ctx, `SELECT id FROM ct_tags WHERE name='남성향'`).Scan(&tag1)
	db.Pool.QueryRow(ctx, `SELECT id FROM ct_tags WHERE name='보수 성향'`).Scan(&tag2)

	date := time.Date(2026, 6, 24, 0, 0, 0, 0, time.UTC)

	// Ensure → pending worksheet
	ws, err := svc.Ensure(ctx, commID, date, "manual")
	if err != nil {
		t.Fatalf("ensure: %v", err)
	}
	if ws.Status != "pending" || ws.Mode != "manual" {
		t.Fatalf("unexpected worksheet: %+v", ws)
	}

	list, err := svc.ListByDate(ctx, date)
	if err != nil {
		t.Fatalf("list by date: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 worksheet, got %d", len(list))
	}

	// Confirm (manual path, no fingerprints)
	err = svc.Confirm(ctx, communitytrend.Confirmation{
		CommunityID: commID,
		StatDate:    date,
		Mode:        "manual",
		Source:      "human",
		TotalPosts:  12,
		Counts:      []communitytrend.TagCount{{TagID: tag1, Count: 5}, {TagID: tag2, Count: 3}},
	})
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}

	// tag_daily has 2 rows with right counts
	var c1, c2, total int
	db.Pool.QueryRow(ctx, `SELECT post_count FROM ct_tag_daily WHERE community_id=$1 AND tag_id=$2 AND stat_date=$3`, commID, tag1, date).Scan(&c1)
	db.Pool.QueryRow(ctx, `SELECT post_count FROM ct_tag_daily WHERE community_id=$1 AND tag_id=$2 AND stat_date=$3`, commID, tag2, date).Scan(&c2)
	db.Pool.QueryRow(ctx, `SELECT total_posts FROM ct_community_daily WHERE community_id=$1 AND stat_date=$2`, commID, date).Scan(&total)
	if c1 != 5 || c2 != 3 || total != 12 {
		t.Fatalf("expected counts 5,3 total 12, got %d,%d total %d", c1, c2, total)
	}

	// worksheet now confirmed
	var status string
	db.Pool.QueryRow(ctx, `SELECT status FROM ct_worksheets WHERE community_id=$1 AND stat_date=$2`, commID, date).Scan(&status)
	if status != "confirmed" {
		t.Fatalf("expected confirmed, got %s", status)
	}

	// Re-confirm updates (idempotent upsert, no duplicate rows)
	err = svc.Confirm(ctx, communitytrend.Confirmation{
		CommunityID: commID, StatDate: date, Mode: "manual", Source: "human", TotalPosts: 15,
		Counts: []communitytrend.TagCount{{TagID: tag1, Count: 7}},
	})
	if err != nil {
		t.Fatalf("re-confirm: %v", err)
	}
	var c1b, rowCount int
	db.Pool.QueryRow(ctx, `SELECT post_count FROM ct_tag_daily WHERE community_id=$1 AND tag_id=$2 AND stat_date=$3`, commID, tag1, date).Scan(&c1b)
	db.Pool.QueryRow(ctx, `SELECT count(*) FROM ct_tag_daily WHERE community_id=$1 AND stat_date=$2`, commID, date).Scan(&rowCount)
	if c1b != 7 {
		t.Fatalf("expected updated count 7, got %d", c1b)
	}
	// tag2 row from first confirm still present (we only upsert provided counts) → 2 rows total
	if rowCount != 2 {
		t.Fatalf("expected 2 tag_daily rows, got %d", rowCount)
	}

	// invalid source rejected
	if err := svc.Confirm(ctx, communitytrend.Confirmation{
		CommunityID: commID, StatDate: date, Mode: "manual", Source: "bogus", TotalPosts: 1,
	}); err == nil {
		t.Fatal("expected error for invalid source")
	}
}

func TestCommunityTrend_WorksheetHTTP(t *testing.T) {
	db := SetupTestDB(t)
	ctx := context.Background()

	svc := communitytrend.NewService(
		storage.NewCTCommunityRepository(db.Pool),
		storage.NewCTTagRepository(db.Pool),
		storage.NewCTAxisRepository(db.Pool),
	)
	wsSvc := communitytrend.NewWorksheetService(storage.NewCTWorksheetRepository(db.Pool))
	adminHandler := handler.NewCommunityTrendAdminHandler(svc, wsSvc)

	gin.SetMode(gin.TestMode)
	jwtManager := auth.NewJWTManager("test-secret")
	router := api.NewRouter("api", "v1", "http://localhost:5173", jwtManager, 10000, memory.NewStore(),
		[]api.RouteModule{
			{GroupName: "admin/community-trend", Handler: adminHandler, Middlewares: []gin.HandlerFunc{}},
		})

	var commID, tag1 int
	db.Pool.QueryRow(ctx, `SELECT id FROM ct_communities WHERE key='clien'`).Scan(&commID)
	db.Pool.QueryRow(ctx, `SELECT id FROM ct_tags WHERE name='진보 성향'`).Scan(&tag1)

	// missing date → 400
	wBad := httptest.NewRecorder()
	reqBad, _ := http.NewRequest("GET", "/api/v1/admin/community-trend/worksheets", nil)
	router.ServeHTTP(wBad, reqBad)
	if wBad.Code != http.StatusBadRequest {
		t.Fatalf("missing date: expected 400, got %d", wBad.Code)
	}

	// confirm via HTTP (manual)
	body, _ := json.Marshal(map[string]any{
		"community_id": commID,
		"stat_date":    "2026-06-24",
		"mode":         "manual",
		"source":       "human",
		"total_posts":  20,
		"counts":       []map[string]int{{"tag_id": tag1, "count": 8}},
	})
	wc := httptest.NewRecorder()
	reqc, _ := http.NewRequest("POST", "/api/v1/admin/community-trend/worksheets/confirm", bytes.NewReader(body))
	reqc.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(wc, reqc)
	if wc.Code != http.StatusOK {
		t.Fatalf("confirm: expected 200, got %d: %s", wc.Code, wc.Body.String())
	}

	// list worksheets for that date → 1 confirmed
	wl := httptest.NewRecorder()
	reql, _ := http.NewRequest("GET", "/api/v1/admin/community-trend/worksheets?date=2026-06-24", nil)
	router.ServeHTTP(wl, reql)
	if wl.Code != http.StatusOK {
		t.Fatalf("list worksheets: expected 200, got %d: %s", wl.Code, wl.Body.String())
	}
	var wsResp struct {
		Data []communitytrend.Worksheet `json:"data"`
	}
	json.Unmarshal(wl.Body.Bytes(), &wsResp)
	if len(wsResp.Data) != 1 || wsResp.Data[0].Status != "confirmed" {
		t.Fatalf("expected 1 confirmed worksheet, got %+v", wsResp.Data)
	}

	// tag_daily persisted
	var cnt, total int
	db.Pool.QueryRow(ctx, `SELECT post_count FROM ct_tag_daily WHERE community_id=$1 AND tag_id=$2`, commID, tag1).Scan(&cnt)
	db.Pool.QueryRow(ctx, `SELECT total_posts FROM ct_community_daily WHERE community_id=$1`, commID).Scan(&total)
	if cnt != 8 || total != 20 {
		t.Fatalf("expected count 8 total 20, got %d / %d", cnt, total)
	}

	// invalid source via HTTP → 400
	badBody, _ := json.Marshal(map[string]any{
		"community_id": commID, "stat_date": "2026-06-24", "mode": "manual", "source": "nope", "total_posts": 1,
	})
	wb := httptest.NewRecorder()
	reqb, _ := http.NewRequest("POST", "/api/v1/admin/community-trend/worksheets/confirm", bytes.NewReader(badBody))
	reqb.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(wb, reqb)
	if wb.Code != http.StatusBadRequest {
		t.Fatalf("invalid source: expected 400, got %d", wb.Code)
	}
}

func TestCommunityTrend_RobotsRepo(t *testing.T) {
	db := SetupTestDB(t)
	ctx := context.Background()
	repo := storage.NewCTRobotsRepository(db.Pool)

	var commID int
	db.Pool.QueryRow(ctx, `SELECT id FROM ct_communities WHERE key='dogdrip'`).Scan(&commID)

	// no status yet
	if _, found, err := repo.LatestAllowed(ctx, commID); err != nil || found {
		t.Fatalf("expected no prior status, found=%v err=%v", found, err)
	}

	// first record → transition recorded
	changed, err := repo.Record(ctx, commID, true, "h1", "allow")
	if err != nil || !changed {
		t.Fatalf("first record: changed=%v err=%v", changed, err)
	}
	allowed, found, _ := repo.LatestAllowed(ctx, commID)
	if !found || !allowed {
		t.Fatalf("expected latest allowed=true found, got %v/%v", allowed, found)
	}

	// same allowance → no transition
	changed, _ = repo.Record(ctx, commID, true, "h1", "allow")
	if changed {
		t.Fatal("expected no transition for unchanged allowance")
	}

	// flip to disallowed → transition
	changed, _ = repo.Record(ctx, commID, false, "h2", "HTTP 429")
	if !changed {
		t.Fatal("expected transition on flip to disallowed")
	}

	var statusCount, transitionCount int
	db.Pool.QueryRow(ctx, `SELECT count(*) FROM ct_robots_status WHERE community_id=$1`, commID).Scan(&statusCount)
	db.Pool.QueryRow(ctx, `SELECT count(*) FROM ct_robots_transitions WHERE community_id=$1`, commID).Scan(&transitionCount)
	if statusCount != 3 {
		t.Fatalf("expected 3 status rows, got %d", statusCount)
	}
	if transitionCount != 2 {
		t.Fatalf("expected 2 transitions, got %d", transitionCount)
	}
}

func TestCommunityTrend_SeenRepo(t *testing.T) {
	db := SetupTestDB(t)
	ctx := context.Background()
	repo := storage.NewCTSeenRepository(db.Pool)

	var commID int
	db.Pool.QueryRow(ctx, `SELECT id FROM ct_communities WHERE key='dogdrip'`).Scan(&commID)

	today := time.Date(2026, 6, 24, 0, 0, 0, 0, time.UTC)
	old := time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)
	for _, row := range []struct {
		fp   string
		when time.Time
	}{{"fp1", today}, {"fp2", today}, {"fp3", old}} {
		if _, err := db.Pool.Exec(ctx,
			`INSERT INTO ct_seen_posts (community_id, fingerprint, first_seen) VALUES ($1,$2,$3)`,
			commID, row.fp, row.when); err != nil {
			t.Fatalf("seed seen: %v", err)
		}
	}

	seen, err := repo.LoadSeen(ctx, commID)
	if err != nil {
		t.Fatalf("load seen: %v", err)
	}
	if len(seen) != 3 || !seen["fp1"] || !seen["fp3"] {
		t.Fatalf("expected 3 seen incl fp1/fp3, got %v", seen)
	}

	// prune rows older than 2026-06-22 → removes fp3
	n, err := repo.Prune(ctx, time.Date(2026, 6, 22, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 pruned, got %d", n)
	}
	seen2, _ := repo.LoadSeen(ctx, commID)
	if len(seen2) != 2 || seen2["fp3"] {
		t.Fatalf("expected fp3 pruned, got %v", seen2)
	}
}
