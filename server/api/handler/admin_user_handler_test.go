package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"ota/api/handler"
	"ota/domain/user"
)

// ─── Mock User Repository (admin handler) ───────────────────────────────────

type mockUserRepoAdminUser struct {
	usersByID    map[string]user.User
	usersByEmail map[string]user.User
	updates      []roleUpdate
	updateErr    error
}

type roleUpdate struct {
	UserID, NewRole string
}

func newMockUserRepoAdminUser() *mockUserRepoAdminUser {
	return &mockUserRepoAdminUser{
		usersByID:    make(map[string]user.User),
		usersByEmail: make(map[string]user.User),
	}
}

func (m *mockUserRepoAdminUser) add(u user.User) {
	m.usersByID[u.ID] = u
	if u.Email != "" {
		m.usersByEmail[u.Email] = u
	}
}

func (m *mockUserRepoAdminUser) UpsertByKakaoID(context.Context, int64, string, string, string) (user.User, error) {
	panic("not used")
}
func (m *mockUserRepoAdminUser) FindByID(_ context.Context, id string) (user.User, error) {
	u, ok := m.usersByID[id]
	if !ok {
		return user.User{}, errors.New("user not found")
	}
	return u, nil
}
func (m *mockUserRepoAdminUser) FindByKakaoID(context.Context, int64) (user.User, bool, error) {
	panic("not used")
}
func (m *mockUserRepoAdminUser) FindByEmail(_ context.Context, email string) (user.User, error) {
	u, ok := m.usersByEmail[email]
	if !ok {
		return user.User{}, errors.New("user not found")
	}
	return u, nil
}
func (m *mockUserRepoAdminUser) UpdateEmail(context.Context, string, string) error {
	return nil
}
func (m *mockUserRepoAdminUser) UpdateRole(_ context.Context, userID, newRole string) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	u, ok := m.usersByID[userID]
	if !ok {
		return errors.New("user not found")
	}
	u.Role = newRole
	m.usersByID[userID] = u
	if u.Email != "" {
		m.usersByEmail[u.Email] = u
	}
	m.updates = append(m.updates, roleUpdate{userID, newRole})
	return nil
}
func (m *mockUserRepoAdminUser) UpdatePenName(context.Context, string, string) error  { return nil }
func (m *mockUserRepoAdminUser) UpdateNickname(context.Context, string, string) error { return nil }
func (m *mockUserRepoAdminUser) AcknowledgeNicknameWarning(context.Context, string) error {
	return nil
}
func (m *mockUserRepoAdminUser) DeleteByID(context.Context, string) error { return nil }

// ─── Mock Role Change Repository ────────────────────────────────────────────

type mockRoleChangeRepo struct {
	entries []user.RoleChangeLog
	logErr  error
}

func (m *mockRoleChangeRepo) Log(_ context.Context, entry user.RoleChangeLog) (user.RoleChangeLog, error) {
	if m.logErr != nil {
		return user.RoleChangeLog{}, m.logErr
	}
	entry.ID = "log-id"
	m.entries = append(m.entries, entry)
	return entry, nil
}

func (m *mockRoleChangeRepo) ListByUser(_ context.Context, userID string, _, _ int) ([]user.RoleChangeLog, error) {
	var out []user.RoleChangeLog
	for _, e := range m.entries {
		if e.UserID == userID {
			out = append(out, e)
		}
	}
	return out, nil
}

// ─── Test helpers ───────────────────────────────────────────────────────────

func setupAdminUserHandler(repo *mockUserRepoAdminUser, audit *mockRoleChangeRepo, callerID string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := handler.NewAdminUserHandler(repo, audit)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("userID", callerID); c.Next() })
	group := r.Group("/admin/users")
	h.RegisterRoutes(group)
	return r
}

func postJSON(t *testing.T, r http.Handler, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// ─── Tests ──────────────────────────────────────────────────────────────────

func TestAdminUserHandler_SearchUser_ByID(t *testing.T) {
	repo := newMockUserRepoAdminUser()
	repo.add(user.User{ID: "u-1", Email: "a@x.com", Role: user.RoleUser})
	r := setupAdminUserHandler(repo, &mockRoleChangeRepo{}, "admin-1")

	req := httptest.NewRequest(http.MethodGet, "/admin/users/search?type=id&q=u-1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
}

func TestAdminUserHandler_SearchUser_NotFound(t *testing.T) {
	repo := newMockUserRepoAdminUser()
	r := setupAdminUserHandler(repo, &mockRoleChangeRepo{}, "admin-1")

	req := httptest.NewRequest(http.MethodGet, "/admin/users/search?type=email&q=nope@x.com", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestAdminUserHandler_SearchUser_InvalidType(t *testing.T) {
	r := setupAdminUserHandler(newMockUserRepoAdminUser(), &mockRoleChangeRepo{}, "admin-1")

	req := httptest.NewRequest(http.MethodGet, "/admin/users/search?type=phone&q=1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestAdminUserHandler_UpdateRole_Promote(t *testing.T) {
	repo := newMockUserRepoAdminUser()
	repo.add(user.User{ID: "u-1", Email: "a@x.com", Role: user.RoleUser})
	audit := &mockRoleChangeRepo{}
	r := setupAdminUserHandler(repo, audit, "admin-1")

	w := postJSON(t, r, "/admin/users/role", map[string]string{
		"user_id":  "u-1",
		"new_role": user.RoleEditor,
		"memo":     "promote test",
	})

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	if repo.usersByID["u-1"].Role != user.RoleEditor {
		t.Errorf("role not updated, got %s", repo.usersByID["u-1"].Role)
	}
	if len(audit.entries) != 1 {
		t.Fatalf("expected one audit entry, got %d", len(audit.entries))
	}
	if audit.entries[0].BeforeRole != user.RoleUser || audit.entries[0].AfterRole != user.RoleEditor {
		t.Errorf("audit entry mismatch: %+v", audit.entries[0])
	}
}

func TestAdminUserHandler_UpdateRole_RejectSelfChange(t *testing.T) {
	repo := newMockUserRepoAdminUser()
	repo.add(user.User{ID: "admin-1", Role: user.RoleAdmin})
	r := setupAdminUserHandler(repo, &mockRoleChangeRepo{}, "admin-1")

	w := postJSON(t, r, "/admin/users/role", map[string]string{
		"user_id":  "admin-1",
		"new_role": user.RoleEditor,
	})

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestAdminUserHandler_UpdateRole_InvalidRole(t *testing.T) {
	repo := newMockUserRepoAdminUser()
	repo.add(user.User{ID: "u-1", Role: user.RoleUser})
	r := setupAdminUserHandler(repo, &mockRoleChangeRepo{}, "admin-1")

	w := postJSON(t, r, "/admin/users/role", map[string]string{
		"user_id":  "u-1",
		"new_role": "superadmin",
	})

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestAdminUserHandler_UpdateRole_NoChangeIsIdempotent(t *testing.T) {
	repo := newMockUserRepoAdminUser()
	repo.add(user.User{ID: "u-1", Role: user.RoleEditor})
	audit := &mockRoleChangeRepo{}
	r := setupAdminUserHandler(repo, audit, "admin-1")

	w := postJSON(t, r, "/admin/users/role", map[string]string{
		"user_id":  "u-1",
		"new_role": user.RoleEditor,
	})

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if len(audit.entries) != 0 {
		t.Errorf("no audit entry expected when role unchanged, got %d", len(audit.entries))
	}
}

func TestAdminUserHandler_RoleHistory(t *testing.T) {
	repo := newMockUserRepoAdminUser()
	repo.add(user.User{ID: "u-1", Role: user.RoleEditor})
	actor := "admin-1"
	audit := &mockRoleChangeRepo{
		entries: []user.RoleChangeLog{
			{ID: "log-1", UserID: "u-1", BeforeRole: user.RoleUser, AfterRole: user.RoleEditor, ActorID: &actor},
		},
	}
	r := setupAdminUserHandler(repo, audit, "admin-1")

	req := httptest.NewRequest(http.MethodGet, "/admin/users/u-1/role-history", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp struct {
		Data []user.RoleChangeLog `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("expected one history entry, got %d", len(resp.Data))
	}
}
