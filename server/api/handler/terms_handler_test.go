package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"ota/api/handler"
	"ota/domain/terms"
)

// ─── Mock Terms Repository ──────────────────────────────────────────────────

type mockTermsRepoForHandler struct {
	allTerms       []terms.Term
	activeTerms    []terms.Term
	createErr      error
	updateActiveErr error
}

func (m *mockTermsRepoForHandler) Create(_ context.Context, t terms.Term) (terms.Term, error) {
	if m.createErr != nil {
		return terms.Term{}, m.createErr
	}
	t.ID = "new-id"
	return t, nil
}

func (m *mockTermsRepoForHandler) ListAll(_ context.Context) ([]terms.Term, error) {
	return m.allTerms, nil
}

func (m *mockTermsRepoForHandler) ListActive(_ context.Context) ([]terms.Term, error) {
	return m.activeTerms, nil
}

func (m *mockTermsRepoForHandler) FindActiveRequired(_ context.Context) ([]terms.Term, error) {
	return nil, nil
}

func (m *mockTermsRepoForHandler) SaveConsents(_ context.Context, _ string, _ []string) error {
	return nil
}

func (m *mockTermsRepoForHandler) UpdateActive(_ context.Context, termID string, active bool) error {
	if m.updateActiveErr != nil {
		return m.updateActiveErr
	}
	for i, t := range m.allTerms {
		if t.ID == termID {
			m.allTerms[i].Active = active
			return nil
		}
	}
	return fmt.Errorf("term not found")
}

func (m *mockTermsRepoForHandler) GetUserConsents(_ context.Context, _ string) ([]terms.UserTermConsent, error) {
	return nil, nil
}

// ─── Tests ──────────────────────────────────────────────────────────────────

func TestTermsHandler_ListActive(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &mockTermsRepoForHandler{
		activeTerms: []terms.Term{
			{ID: "1", Title: "Privacy", Active: true, Required: true, Version: "1"},
			{ID: "2", Title: "Marketing", Active: true, Required: false, Version: "1"},
		},
	}
	svc := terms.NewService(repo)
	h := handler.NewTermsHandler(svc)

	r := gin.New()
	group := r.Group("/api/v1/terms")
	h.RegisterRoutes(group)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/terms/active", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp struct {
		Data []terms.Term `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Data) != 2 {
		t.Fatalf("expected 2 terms, got %d", len(resp.Data))
	}
}

func TestTermsHandler_ListActive_Empty(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &mockTermsRepoForHandler{activeTerms: nil}
	svc := terms.NewService(repo)
	h := handler.NewTermsHandler(svc)

	r := gin.New()
	group := r.Group("/api/v1/terms")
	h.RegisterRoutes(group)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/terms/active", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp struct {
		Data []terms.Term `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Data == nil {
		t.Fatal("expected non-nil empty array, got nil")
	}
	if len(resp.Data) != 0 {
		t.Fatalf("expected 0 terms, got %d", len(resp.Data))
	}
}

func TestTermsAdminHandler_ListAll(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &mockTermsRepoForHandler{
		allTerms: []terms.Term{
			{ID: "1", Title: "Privacy", Active: true, Version: "1"},
			{ID: "2", Title: "Old TOS", Active: false, Version: "1"},
		},
	}
	svc := terms.NewService(repo)
	h := handler.NewTermsAdminHandler(svc)

	r := gin.New()
	group := r.Group("/api/v1/admin/terms")
	h.RegisterRoutes(group)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/admin/terms", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp struct {
		Data []terms.Term `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Data) != 2 {
		t.Fatalf("expected 2 terms (including inactive), got %d", len(resp.Data))
	}
}

func TestTermsAdminHandler_Create_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &mockTermsRepoForHandler{}
	svc := terms.NewService(repo)
	h := handler.NewTermsAdminHandler(svc)

	r := gin.New()
	group := r.Group("/api/v1/admin/terms")
	h.RegisterRoutes(group)

	active := true
	required := true
	body, _ := json.Marshal(map[string]any{
		"title":    "개인정보 처리방침",
		"url":      "https://notion.so/privacy",
		"version":  "1.2",
		"active":   active,
		"required": required,
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/admin/terms", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTermsAdminHandler_Create_MissingFields(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name string
		body map[string]any
	}{
		{"missing title", map[string]any{"url": "https://example.com", "version": "1", "active": true, "required": true}},
		{"missing url", map[string]any{"title": "TOS", "version": "1", "active": true, "required": true}},
		{"missing version", map[string]any{"title": "TOS", "url": "https://example.com", "active": true, "required": true}},
		{"missing active", map[string]any{"title": "TOS", "url": "https://example.com", "version": "1", "required": true}},
		{"missing required", map[string]any{"title": "TOS", "url": "https://example.com", "version": "1", "active": true}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockTermsRepoForHandler{}
			svc := terms.NewService(repo)
			h := handler.NewTermsAdminHandler(svc)

			r := gin.New()
			group := r.Group("/api/v1/admin/terms")
			h.RegisterRoutes(group)

			body, _ := json.Marshal(tt.body)
			w := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/api/v1/admin/terms", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestTermsAdminHandler_UpdateActive_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &mockTermsRepoForHandler{
		allTerms: []terms.Term{{ID: "t-1", Title: "Privacy", Active: true}},
	}
	svc := terms.NewService(repo)
	h := handler.NewTermsAdminHandler(svc)

	r := gin.New()
	group := r.Group("/api/v1/admin/terms")
	h.RegisterRoutes(group)

	body, _ := json.Marshal(map[string]any{"active": false})
	w := httptest.NewRecorder()
	req := httptest.NewRequest("PATCH", "/api/v1/admin/terms/t-1/active", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if repo.allTerms[0].Active {
		t.Fatal("expected term to be deactivated")
	}
}

func TestTermsAdminHandler_UpdateActive_MissingField(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &mockTermsRepoForHandler{}
	svc := terms.NewService(repo)
	h := handler.NewTermsAdminHandler(svc)

	r := gin.New()
	group := r.Group("/api/v1/admin/terms")
	h.RegisterRoutes(group)

	body, _ := json.Marshal(map[string]any{})
	w := httptest.NewRecorder()
	req := httptest.NewRequest("PATCH", "/api/v1/admin/terms/t-1/active", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestTermsAdminHandler_UpdateActive_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &mockTermsRepoForHandler{}
	svc := terms.NewService(repo)
	h := handler.NewTermsAdminHandler(svc)

	r := gin.New()
	group := r.Group("/api/v1/admin/terms")
	h.RegisterRoutes(group)

	body, _ := json.Marshal(map[string]any{"active": true})
	w := httptest.NewRecorder()
	req := httptest.NewRequest("PATCH", "/api/v1/admin/terms/nonexistent/active", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTermsAdminHandler_Create_DuplicateError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &mockTermsRepoForHandler{createErr: fmt.Errorf("duplicate key value violates unique constraint")}
	svc := terms.NewService(repo)
	h := handler.NewTermsAdminHandler(svc)

	r := gin.New()
	group := r.Group("/api/v1/admin/terms")
	h.RegisterRoutes(group)

	active := true
	required := true
	body, _ := json.Marshal(map[string]any{
		"title":    "Privacy",
		"url":      "https://example.com",
		"version":  "1",
		"active":   active,
		"required": required,
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/admin/terms", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for duplicate, got %d: %s", w.Code, w.Body.String())
	}
}
