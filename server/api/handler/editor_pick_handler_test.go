package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"ota/api/handler"
	"ota/domain/editor"
)

func newGET(path string) *http.Request {
	return httptest.NewRequest(http.MethodGet, path, nil)
}

func serve(r http.Handler, req *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func setupPickHandler(t *testing.T) (*gin.Engine, *inMemoryEditorRepo) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	repo := newInMemoryEditorRepo()
	r := gin.New()
	h := handler.NewEditorPickHandler(repo)
	h.RegisterRoutes(r.Group("/editor-picks"))
	return r, repo
}

func seedPublished(t *testing.T, repo *inMemoryEditorRepo, n int) {
	t.Helper()
	for i := range n {
		pub := time.Date(2026, 5, 1+i, 9, 0, 0, 0, time.UTC)
		_, _ = repo.Create(t.Context(), editor.Post{
			AuthorID:    "u-1",
			Title:       "post",
			ContentHTML: "<p>x</p>",
			ContentText: "x",
			Status:      editor.StatusPublished,
			PublishedAt: &pub,
		})
	}
}

func TestEditorPickHandler_List_DefaultLimit(t *testing.T) {
	r, repo := setupPickHandler(t)
	seedPublished(t, repo, 15)

	req := newGET("/editor-picks")
	w := serve(r, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var resp struct {
		Data struct {
			Items []editor.PublicCard `json:"items"`
			Total int                 `json:"total"`
			Limit int                 `json:"limit"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Data.Limit != 10 {
		t.Errorf("limit = %d, want 10", resp.Data.Limit)
	}
	if resp.Data.Total != 15 {
		t.Errorf("total = %d, want 15", resp.Data.Total)
	}
	if len(resp.Data.Items) != 10 {
		t.Errorf("items = %d, want 10", len(resp.Data.Items))
	}
}

func TestEditorPickHandler_List_OffsetPagination(t *testing.T) {
	r, repo := setupPickHandler(t)
	seedPublished(t, repo, 15)

	w := serve(r, newGET("/editor-picks?limit=10&offset=10"))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var resp struct {
		Data struct {
			Items []editor.PublicCard `json:"items"`
			Total int                 `json:"total"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Data.Items) != 5 {
		t.Errorf("second page items = %d, want 5", len(resp.Data.Items))
	}
}

func TestEditorPickHandler_Get_OnlyPublished(t *testing.T) {
	r, repo := setupPickHandler(t)
	draft, _ := repo.Create(t.Context(), editor.Post{
		AuthorID: "u-1", Title: "d", ContentHTML: "<p>x</p>", Status: editor.StatusDraft,
	})

	w := serve(r, newGET("/editor-picks/"+draft.ID))
	if w.Code != http.StatusNotFound {
		t.Fatalf("draft should 404, got %d", w.Code)
	}
}

func TestEditorPickHandler_Get_Published_OK(t *testing.T) {
	r, repo := setupPickHandler(t)
	pub := time.Now()
	post, _ := repo.Create(t.Context(), editor.Post{
		AuthorID: "u-1", Title: "p", ContentHTML: "<p>hi</p>", Status: editor.StatusPublished, PublishedAt: &pub,
	})

	w := serve(r, newGET("/editor-picks/"+post.ID))
	if w.Code != http.StatusOK {
		t.Fatalf("published should 200, got %d (body: %s)", w.Code, w.Body.String())
	}
}

// ─── Search ─────────────────────────────────────────────────────────────────

func TestEditorPickHandler_Search_MissingQuery(t *testing.T) {
	r, _ := setupPickHandler(t)
	w := serve(r, newGET("/editor-picks/search"))
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestEditorPickHandler_Search_BlankQuery(t *testing.T) {
	r, _ := setupPickHandler(t)
	w := serve(r, newGET("/editor-picks/search?q=%20%20%20"))
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestEditorPickHandler_Search_TooLongQuery(t *testing.T) {
	r, _ := setupPickHandler(t)
	long := ""
	for range 101 {
		long += "가"
	}
	w := serve(r, newGET("/editor-picks/search?q="+long))
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestEditorPickHandler_Search_RanksTitleAboveBody(t *testing.T) {
	r, repo := setupPickHandler(t)
	pub := time.Now()
	// Body match created first (so it would win on recency in a tie).
	bodyHit, _ := repo.Create(t.Context(), editor.Post{
		AuthorID: "u", Title: "다른 글", ContentHTML: "<p>x</p>",
		ContentText: "본문에 삼성전자 언급",
		Status:      editor.StatusPublished, PublishedAt: &pub,
	})
	titlePub := pub.Add(-time.Hour) // older, but still wins via title rank
	titleHit, _ := repo.Create(t.Context(), editor.Post{
		AuthorID: "u", Title: "삼성전자 분석", ContentHTML: "<p>y</p>",
		ContentText: "관련 없는 내용",
		Status:      editor.StatusPublished, PublishedAt: &titlePub,
	})

	w := serve(r, newGET("/editor-picks/search?q=삼성전자"))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d (body: %s)", w.Code, w.Body.String())
	}
	var resp struct {
		Data struct {
			Items   []editor.PublicCard `json:"items"`
			HasMore bool                `json:"has_more"`
			Query   string              `json:"query"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Data.Items) != 2 {
		t.Fatalf("expected 2 hits, got %d", len(resp.Data.Items))
	}
	if resp.Data.Items[0].ID != titleHit.ID {
		t.Errorf("title match should come first, got %q", resp.Data.Items[0].Title)
	}
	if resp.Data.Items[1].ID != bodyHit.ID {
		t.Errorf("body match should come second, got %q", resp.Data.Items[1].Title)
	}
	if resp.Data.Query != "삼성전자" {
		t.Errorf("query = %q, want 삼성전자", resp.Data.Query)
	}
}

func TestEditorPickHandler_Search_SkipsDrafts(t *testing.T) {
	r, repo := setupPickHandler(t)
	_, _ = repo.Create(t.Context(), editor.Post{
		AuthorID: "u", Title: "삼성전자 초안", ContentHTML: "<p>x</p>",
		ContentText: "draft", Status: editor.StatusDraft,
	})

	w := serve(r, newGET("/editor-picks/search?q=삼성전자"))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var resp struct {
		Data struct {
			Items []editor.PublicCard `json:"items"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Data.Items) != 0 {
		t.Errorf("drafts should not appear, got %d hits", len(resp.Data.Items))
	}
}

func TestEditorPickHandler_Search_HasMorePagination(t *testing.T) {
	r, repo := setupPickHandler(t)
	// Three hits — paginate with limit=2.
	for i := range 3 {
		pub := time.Now().Add(-time.Duration(i) * time.Hour)
		_, _ = repo.Create(t.Context(), editor.Post{
			AuthorID: "u", Title: "테스트", ContentHTML: "<p>x</p>",
			ContentText: "삼성전자 본문 " + string(rune('A'+i)),
			Status:      editor.StatusPublished, PublishedAt: &pub,
		})
	}

	w := serve(r, newGET("/editor-picks/search?q=삼성전자&limit=2&offset=0"))
	var page1 struct {
		Data struct {
			Items   []editor.PublicCard `json:"items"`
			HasMore bool                `json:"has_more"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &page1)
	if len(page1.Data.Items) != 2 || !page1.Data.HasMore {
		t.Errorf("page 1: items=%d hasMore=%v, want 2/true", len(page1.Data.Items), page1.Data.HasMore)
	}

	w2 := serve(r, newGET("/editor-picks/search?q=삼성전자&limit=2&offset=2"))
	var page2 struct {
		Data struct {
			Items   []editor.PublicCard `json:"items"`
			HasMore bool                `json:"has_more"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w2.Body.Bytes(), &page2)
	if len(page2.Data.Items) != 1 || page2.Data.HasMore {
		t.Errorf("page 2: items=%d hasMore=%v, want 1/false", len(page2.Data.Items), page2.Data.HasMore)
	}
}
