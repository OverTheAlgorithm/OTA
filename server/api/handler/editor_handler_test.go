package handler_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"

	"ota/api/handler"
	"ota/domain/editor"
	"ota/domain/user"
)

func setupEditorHandler(t *testing.T, callerID, callerRole string) (*gin.Engine, *editor.Service) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	// Re-use the service against an in-memory fake repo. The fake lives in the
	// editor package's tests; we replicate just enough here.
	repo := newInMemoryEditorRepo()
	svc := editor.NewService(repo)
	h := handler.NewEditorHandler(svc)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", callerID)
		c.Set("role", callerRole)
		c.Next()
	})
	group := r.Group("/editor")
	h.RegisterRoutes(group)
	return r, svc
}

func extractEditorPost(t *testing.T, body []byte) editor.Post {
	t.Helper()
	var resp struct {
		Data editor.Post `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("unmarshal: %v (body=%s)", err, string(body))
	}
	return resp.Data
}

func TestEditorHandler_CreateAndGet(t *testing.T) {
	r, _ := setupEditorHandler(t, "u-1", user.RoleEditor)

	w := doRequest(r,http.MethodPost, "/editor/posts", map[string]string{
		"title":        "First Post",
		"content_html": `<p>Hello <strong>world</strong></p>`,
		"status":       editor.StatusPublished,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("create status = %d, body = %s", w.Code, w.Body.String())
	}
	created := extractEditorPost(t, w.Body.Bytes())
	if created.ID == "" {
		t.Fatal("ID should be assigned")
	}

	w2 := doRequest(r,http.MethodGet, "/editor/posts/"+created.ID, nil)
	if w2.Code != http.StatusOK {
		t.Fatalf("get status = %d", w2.Code)
	}
}

func TestEditorHandler_Create_BadRequest(t *testing.T) {
	r, _ := setupEditorHandler(t, "u-1", user.RoleEditor)

	w := doRequest(r,http.MethodPost, "/editor/posts", map[string]string{
		"title": "no content",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestEditorHandler_Update_NonOwnerForbidden(t *testing.T) {
	r, svc := setupEditorHandler(t, "u-2", user.RoleEditor)
	post, err := svc.Create(t.Context(), editor.CreateParams{
		AuthorID: "u-1", Title: "v1", ContentHTML: "<p>v1</p>", Status: editor.StatusDraft,
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	w := doRequest(r,http.MethodPut, "/editor/posts/"+post.ID, map[string]string{
		"title":        "hijack",
		"content_html": "<p>x</p>",
		"status":       editor.StatusDraft,
	})
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", w.Code)
	}
}

func TestEditorHandler_Delete_OwnerSucceeds(t *testing.T) {
	r, svc := setupEditorHandler(t, "u-1", user.RoleEditor)
	post, _ := svc.Create(t.Context(), editor.CreateParams{
		AuthorID: "u-1", Title: "t", ContentHTML: "<p>x</p>", Status: editor.StatusDraft,
	})

	w := doRequest(r,http.MethodDelete, "/editor/posts/"+post.ID, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
}

func TestEditorHandler_List_Editor_SeesOnlyOwn(t *testing.T) {
	r, svc := setupEditorHandler(t, "u-1", user.RoleEditor)
	_, _ = svc.Create(t.Context(), editor.CreateParams{AuthorID: "u-1", Title: "a", ContentHTML: "<p>a</p>", Status: editor.StatusDraft})
	_, _ = svc.Create(t.Context(), editor.CreateParams{AuthorID: "u-2", Title: "b", ContentHTML: "<p>b</p>", Status: editor.StatusDraft})

	w := doRequest(r,http.MethodGet, "/editor/posts", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var resp struct {
		Data []editor.Post `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Data) != 1 || resp.Data[0].AuthorID != "u-1" {
		t.Errorf("expected only own post, got %+v", resp.Data)
	}
}

func TestEditorHandler_List_Admin_SeesAll(t *testing.T) {
	r, svc := setupEditorHandler(t, "admin-1", user.RoleAdmin)
	_, _ = svc.Create(t.Context(), editor.CreateParams{AuthorID: "u-1", Title: "a", ContentHTML: "<p>a</p>", Status: editor.StatusDraft})
	_, _ = svc.Create(t.Context(), editor.CreateParams{AuthorID: "u-2", Title: "b", ContentHTML: "<p>b</p>", Status: editor.StatusDraft})

	w := doRequest(r,http.MethodGet, "/editor/posts", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var resp struct {
		Data []editor.Post `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Data) != 2 {
		t.Errorf("admin should see 2 posts, got %d", len(resp.Data))
	}
}
