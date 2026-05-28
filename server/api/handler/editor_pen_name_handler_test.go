package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"

	"ota/api/handler"
	"ota/domain/editor"
	"ota/domain/user"
)

// fakePenNameRepo is a minimal user.Repository for the pen-name endpoint
// tests. Only UpdatePenName and FindByID matter — the rest panic so accidental
// reach-through is obvious.
type fakePenNameRepo struct {
	mu       sync.Mutex
	users    map[string]user.User
	updateFn func(id, pen string) error // optional override (e.g. force conflict)
}

func newFakePenNameRepo(seed user.User) *fakePenNameRepo {
	return &fakePenNameRepo{users: map[string]user.User{seed.ID: seed}}
}

func (f *fakePenNameRepo) FindByID(_ context.Context, id string) (user.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.users[id]
	if !ok {
		return user.User{}, errors.New("user not found")
	}
	return u, nil
}

func (f *fakePenNameRepo) UpdatePenName(_ context.Context, id, penName string) error {
	if f.updateFn != nil {
		return f.updateFn(id, penName)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.users[id]
	if !ok {
		return errors.New("user not found")
	}
	u.PenName = penName
	f.users[id] = u
	return nil
}

func (f *fakePenNameRepo) UpsertByKakaoID(context.Context, int64, string, string, string) (user.User, error) {
	panic("not used")
}
func (f *fakePenNameRepo) FindByKakaoID(context.Context, int64) (user.User, bool, error) {
	panic("not used")
}
func (f *fakePenNameRepo) FindByEmail(context.Context, string) (user.User, error) {
	panic("not used")
}
func (f *fakePenNameRepo) UpdateEmail(context.Context, string, string) error { panic("not used") }
func (f *fakePenNameRepo) UpdateRole(context.Context, string, string) error  { panic("not used") }
func (f *fakePenNameRepo) UpdateNickname(context.Context, string, string) error {
	panic("not used")
}
func (f *fakePenNameRepo) AcknowledgeNicknameWarning(context.Context, string) error {
	panic("not used")
}
func (f *fakePenNameRepo) DeleteByID(context.Context, string) error { panic("not used") }

func setupPenNameRouter(t *testing.T, callerID string, repo *fakePenNameRepo) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	svc := editor.NewService(newInMemoryEditorRepo())
	h := handler.NewEditorHandler(svc).WithUserRepo(repo)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", callerID)
		c.Set("role", user.RoleEditor)
		c.Next()
	})
	h.RegisterRoutes(r.Group("/editor"))
	return r
}

func TestEditorHandler_UpdatePenName_Success(t *testing.T) {
	repo := newFakePenNameRepo(user.User{ID: "u-1", Nickname: "닉네임"})
	r := setupPenNameRouter(t, "u-1", repo)

	w := doRequest(r, http.MethodPut, "/editor/profile/pen-name", map[string]string{
		"pen_name": "  필명러  ",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data user.User `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Data.PenName != "필명러" {
		t.Errorf("PenName = %q, want %q", resp.Data.PenName, "필명러")
	}
}

func TestEditorHandler_UpdatePenName_WhitespaceClears(t *testing.T) {
	repo := newFakePenNameRepo(user.User{ID: "u-1", Nickname: "닉네임", PenName: "기존"})
	r := setupPenNameRouter(t, "u-1", repo)

	w := doRequest(r, http.MethodPut, "/editor/profile/pen-name", map[string]string{
		"pen_name": "  \n\t ",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if repo.users["u-1"].PenName != "" {
		t.Errorf("pen name should be cleared, got %q", repo.users["u-1"].PenName)
	}
}

func TestEditorHandler_UpdatePenName_TooShort(t *testing.T) {
	repo := newFakePenNameRepo(user.User{ID: "u-1", Nickname: "닉네임"})
	r := setupPenNameRouter(t, "u-1", repo)

	w := doRequest(r, http.MethodPut, "/editor/profile/pen-name", map[string]string{
		"pen_name": "한",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 — body = %s", w.Code, w.Body.String())
	}
}

func TestEditorHandler_UpdatePenName_Conflict(t *testing.T) {
	repo := newFakePenNameRepo(user.User{ID: "u-1", Nickname: "닉네임"})
	repo.updateFn = func(string, string) error { return user.ErrPenNameTaken }
	r := setupPenNameRouter(t, "u-1", repo)

	w := doRequest(r, http.MethodPut, "/editor/profile/pen-name", map[string]string{
		"pen_name": "겹침",
	})
	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", w.Code)
	}
}
