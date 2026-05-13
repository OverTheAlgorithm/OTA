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
	for i := 0; i < n; i++ {
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
