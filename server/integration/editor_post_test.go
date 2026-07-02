package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/ulule/limiter/v3/drivers/store/memory"

	"ota/api"
	"ota/api/handler"
	"ota/auth"
	"ota/domain/editor"
	"ota/domain/user"
	"ota/storage"
)

// editorTestEnv bundles everything an editor integration test needs.
type editorTestEnv struct {
	db         *TestDB
	router     *gin.Engine
	jwtManager *auth.JWTManager
	userRepo   *storage.UserRepository
	editorRepo *storage.EditorRepository
}

func setupEditorEnv(t *testing.T) *editorTestEnv {
	t.Helper()
	db := SetupTestDB(t)

	userRepo := storage.NewUserRepository(db.Pool)
	editorRepo := storage.NewEditorRepository(db.Pool)
	editorSvc := editor.NewService(editorRepo)
	roleChangeRepo := storage.NewRoleChangeRepository(db.Pool)
	jwtManager := auth.NewJWTManager("test-secret-editor")

	editorH := handler.NewEditorHandler(editorSvc)
	pickH := handler.NewEditorPickHandler(editorRepo)
	adminUserH := handler.NewAdminUserHandler(userRepo, roleChangeRepo)

	gin.SetMode(gin.TestMode)
	router := api.NewRouter("api", "v1", "http://localhost:5173", jwtManager, 10000, memory.NewStore(), []api.RouteModule{
		{GroupName: "editor", Handler: editorH, Middlewares: []gin.HandlerFunc{api.AuthMiddleware(jwtManager), api.EditorMiddleware(userRepo)}},
		{GroupName: "editor-picks", Handler: pickH, Middlewares: []gin.HandlerFunc{}},
		{GroupName: "admin/users", Handler: adminUserH, Middlewares: []gin.HandlerFunc{api.AuthMiddleware(jwtManager), api.AdminMiddleware(userRepo)}},
	})

	t.Cleanup(func() {
		db.Truncate(t, "editor_posts", "role_change_logs", "users")
	})

	return &editorTestEnv{
		db:         db,
		router:     router,
		jwtManager: jwtManager,
		userRepo:   userRepo,
		editorRepo: editorRepo,
	}
}

func (e *editorTestEnv) createUser(t *testing.T, kakaoID int64, email, role string) (user.User, string) {
	t.Helper()
	u, err := e.userRepo.UpsertByKakaoID(context.Background(), kakaoID, email, "u"+email, "")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if role != "" && role != user.RoleUser {
		if err := e.userRepo.UpdateRole(context.Background(), u.ID, role); err != nil {
			t.Fatalf("update role: %v", err)
		}
		u.Role = role
	}
	token, err := e.jwtManager.Generate(u.ID, u.Role)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	return u, token
}

func (e *editorTestEnv) doRequest(t *testing.T, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	e.router.ServeHTTP(w, req)
	return w
}

func TestEditorPosts_E2E_CRUD(t *testing.T) {
	env := setupEditorEnv(t)

	_, editorToken := env.createUser(t, 10001, "editor@x.com", user.RoleEditor)
	_, regularToken := env.createUser(t, 10002, "reg@x.com", user.RoleUser)

	// Regular user blocked from creating a post.
	w := env.doRequest(t, http.MethodPost, "/api/v1/editor/posts", regularToken, map[string]string{
		"title": "x", "content_html": "<p>x</p>", "status": "draft",
	})
	if w.Code != http.StatusForbidden {
		t.Fatalf("regular user POST should be forbidden, got %d", w.Code)
	}

	// Editor creates a draft.
	w = env.doRequest(t, http.MethodPost, "/api/v1/editor/posts", editorToken, map[string]string{
		"title":        "Draft Title",
		"content_html": "<p>draft content</p>",
		"status":       "draft",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("create status = %d, body = %s", w.Code, w.Body.String())
	}
	var createResp struct {
		Data editor.Post `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &createResp)
	post := createResp.Data

	if post.ID == "" {
		t.Fatal("post ID missing")
	}
	if post.PublishedAt != nil {
		t.Errorf("draft should not have published_at, got %v", post.PublishedAt)
	}

	// Public list does not show drafts.
	w = env.doRequest(t, http.MethodGet, "/api/v1/editor-picks", "", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("public list status = %d", w.Code)
	}
	var listResp struct {
		Data struct {
			Items []editor.PublicCard `json:"items"`
			Total int                 `json:"total"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &listResp)
	if listResp.Data.Total != 0 {
		t.Errorf("expected zero published, got %d", listResp.Data.Total)
	}

	// Editor publishes.
	w = env.doRequest(t, http.MethodPut, "/api/v1/editor/posts/"+post.ID, editorToken, map[string]string{
		"title":        "Published Title",
		"content_html": `<p>hello <img src="/api/v1/images/editor/2026/05/x.png" alt="x"></p>`,
		"status":       "published",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("publish status = %d, body = %s", w.Code, w.Body.String())
	}

	// Public list now sees one.
	w = env.doRequest(t, http.MethodGet, "/api/v1/editor-picks?limit=10&offset=0", "", nil)
	_ = json.Unmarshal(w.Body.Bytes(), &listResp)
	if listResp.Data.Total != 1 {
		t.Errorf("expected 1 published, got %d", listResp.Data.Total)
	}
	if len(listResp.Data.Items) != 1 || listResp.Data.Items[0].Title != "Published Title" {
		t.Errorf("unexpected items: %+v", listResp.Data.Items)
	}
	if listResp.Data.Items[0].FirstImageURL == nil || *listResp.Data.Items[0].FirstImageURL == "" {
		t.Errorf("first_image_url should be derived, got %v", listResp.Data.Items[0].FirstImageURL)
	}

	// Detail returns full HTML.
	w = env.doRequest(t, http.MethodGet, "/api/v1/editor-picks/"+post.ID, "", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("detail status = %d", w.Code)
	}

	// Another editor cannot modify someone else's post.
	_, otherEditorToken := env.createUser(t, 10003, "ed2@x.com", user.RoleEditor)
	w = env.doRequest(t, http.MethodPut, "/api/v1/editor/posts/"+post.ID, otherEditorToken, map[string]string{
		"title": "hijack", "content_html": "<p>x</p>", "status": "draft",
	})
	if w.Code != http.StatusForbidden {
		t.Errorf("non-owner editor should be forbidden, got %d", w.Code)
	}

	// Author can delete.
	w = env.doRequest(t, http.MethodDelete, "/api/v1/editor/posts/"+post.ID, editorToken, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("delete status = %d", w.Code)
	}

	// Detail now 404.
	w = env.doRequest(t, http.MethodGet, "/api/v1/editor-picks/"+post.ID, "", nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("deleted post detail should 404, got %d", w.Code)
	}
}

func TestAdminUsers_RoleChange_E2E(t *testing.T) {
	env := setupEditorEnv(t)

	admin, adminToken := env.createUser(t, 20001, "admin@x.com", user.RoleAdmin)
	target, _ := env.createUser(t, 20002, "target@x.com", user.RoleUser)

	// Promote.
	w := env.doRequest(t, http.MethodPost, "/api/v1/admin/users/role", adminToken, map[string]string{
		"user_id":  target.ID,
		"new_role": user.RoleEditor,
		"memo":     "promote",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("promote status = %d, body = %s", w.Code, w.Body.String())
	}

	// History exposes one entry.
	w = env.doRequest(t, http.MethodGet, "/api/v1/admin/users/"+target.ID+"/role-history", adminToken, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("history status = %d", w.Code)
	}
	var histResp struct {
		Data []user.RoleChangeLog `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &histResp)
	if len(histResp.Data) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(histResp.Data))
	}
	if histResp.Data[0].BeforeRole != user.RoleUser || histResp.Data[0].AfterRole != user.RoleEditor {
		t.Errorf("history mismatch: %+v", histResp.Data[0])
	}

	// Self-change blocked.
	w = env.doRequest(t, http.MethodPost, "/api/v1/admin/users/role", adminToken, map[string]string{
		"user_id":  admin.ID,
		"new_role": user.RoleEditor,
	})
	if w.Code != http.StatusBadRequest {
		t.Errorf("self-change status = %d, want 400", w.Code)
	}

	// Non-admin denied.
	_, regularToken := env.createUser(t, 20003, "reg@x.com", user.RoleUser)
	w = env.doRequest(t, http.MethodPost, "/api/v1/admin/users/role", regularToken, map[string]string{
		"user_id":  target.ID,
		"new_role": user.RoleAdmin,
	})
	if w.Code != http.StatusForbidden {
		t.Errorf("non-admin status = %d, want 403", w.Code)
	}
}

func TestSitemap_IncludesEditorPosts_E2E(t *testing.T) {
	env := setupEditorEnv(t)
	_, editorToken := env.createUser(t, 30001, "editor@x.com", user.RoleEditor)

	// Publish a post via the API.
	w := env.doRequest(t, http.MethodPost, "/api/v1/editor/posts", editorToken, map[string]string{
		"title":        "sitemap test",
		"content_html": "<p>hello</p>",
		"status":       "published",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("create status = %d", w.Code)
	}
	var createResp struct {
		Data editor.Post `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &createResp)
	publishedID := createResp.Data.ID

	// Wire the sitemap handler against the same pool.
	sitemapRepo := storage.NewSitemapRepository(env.db.Pool, 0)
	type adapter struct {
		repo *storage.SitemapRepository
	}
	// Inline adapter to avoid importing main.
	a := &editorSitemapAdapter{repo: sitemapRepo}
	sitemapHandler := handler.NewSitemapHandler(a, "https://example.test", 0)

	r := gin.New()
	sitemapHandler.RegisterRoutes(r.Group("/api/v1"))

	w = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sitemap.xml", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("sitemap status = %d", w.Code)
	}
	body := w.Body.String()
	for _, want := range []string{
		"https://example.test/editor-picks",
		"https://example.test/editor-picks/" + publishedID,
	} {
		if !bytes.Contains([]byte(body), []byte(want)) {
			t.Errorf("sitemap missing %q", want)
		}
	}
}

type editorSitemapAdapter struct {
	repo *storage.SitemapRepository
}

func (a *editorSitemapAdapter) GetAllTopicIDs(ctx context.Context) ([]handler.TopicEntry, error) {
	rows, err := a.repo.GetAllTopicRows(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]handler.TopicEntry, len(rows))
	for i, r := range rows {
		out[i] = handler.TopicEntry{ID: r.ID, CreatedAt: r.CreatedAt}
	}
	return out, nil
}

func (a *editorSitemapAdapter) GetAllEditorPostEntries(ctx context.Context) ([]handler.EditorPostEntry, error) {
	rows, err := a.repo.GetAllEditorPostRows(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]handler.EditorPostEntry, len(rows))
	for i, r := range rows {
		out[i] = handler.EditorPostEntry{ID: r.ID, UpdatedAt: r.UpdatedAt}
	}
	return out, nil
}

