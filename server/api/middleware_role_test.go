package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"ota/auth"
	"ota/domain/user"
)

// fakeUserRepo satisfies user.Repository just enough for the middleware tests.
// Only FindByID is exercised; the other methods panic so accidental use is loud.
type fakeUserRepo struct {
	user user.User
	err  error
}

func (f *fakeUserRepo) FindByID(ctx context.Context, id string) (user.User, error) {
	if f.err != nil {
		return user.User{}, f.err
	}
	if f.user.ID != id {
		return user.User{}, errors.New("user not found")
	}
	return f.user, nil
}
func (f *fakeUserRepo) UpsertByKakaoID(context.Context, int64, string, string, string) (user.User, error) {
	panic("not used")
}
func (f *fakeUserRepo) FindByKakaoID(context.Context, int64) (user.User, bool, error) {
	panic("not used")
}
func (f *fakeUserRepo) FindByEmail(context.Context, string) (user.User, error) {
	panic("not used")
}
func (f *fakeUserRepo) UpdateEmail(context.Context, string, string) error { panic("not used") }
func (f *fakeUserRepo) DeleteByID(context.Context, string) error          { panic("not used") }
func (f *fakeUserRepo) UpdateRole(context.Context, string, string) error  { panic("not used") }

func TestRequireRoleMiddleware(t *testing.T) {
	jwtMgr := auth.NewJWTManager("test-secret")

	tests := []struct {
		name       string
		userRole   string
		repoErr    error
		minRole    string
		wantStatus int
	}{
		{"user requesting user", user.RoleUser, nil, user.RoleUser, http.StatusOK},
		{"user requesting editor blocked", user.RoleUser, nil, user.RoleEditor, http.StatusForbidden},
		{"editor requesting editor", user.RoleEditor, nil, user.RoleEditor, http.StatusOK},
		{"editor requesting admin blocked", user.RoleEditor, nil, user.RoleAdmin, http.StatusForbidden},
		{"admin requesting editor allowed", user.RoleAdmin, nil, user.RoleEditor, http.StatusOK},
		{"admin requesting admin", user.RoleAdmin, nil, user.RoleAdmin, http.StatusOK},
		{"repo error returns forbidden", user.RoleAdmin, errors.New("db down"), user.RoleAdmin, http.StatusForbidden},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeUserRepo{
				user: user.User{ID: "u-1", Role: tc.userRole},
				err:  tc.repoErr,
			}

			r := gin.New()
			r.Use(func(c *gin.Context) { c.Set("userID", "u-1"); c.Next() })
			r.Use(RequireRoleMiddleware(repo, tc.minRole))
			r.GET("/ping", func(c *gin.Context) {
				role, _ := c.Get("role")
				c.JSON(http.StatusOK, gin.H{"role": role})
			})

			token, _ := jwtMgr.Generate("u-1", tc.userRole)
			req := httptest.NewRequest(http.MethodGet, "/ping", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (body: %s)", w.Code, tc.wantStatus, w.Body.String())
			}
		})
	}
}

func TestAdminMiddleware_DelegatesToRequireRole(t *testing.T) {
	// AdminMiddleware should reject anyone below admin and allow admins.
	repo := &fakeUserRepo{user: user.User{ID: "u-1", Role: user.RoleEditor}}
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("userID", "u-1"); c.Next() })
	r.Use(AdminMiddleware(repo))
	r.GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))
	if w.Code != http.StatusForbidden {
		t.Fatalf("editor should be forbidden by AdminMiddleware, got %d", w.Code)
	}

	repo.user.Role = user.RoleAdmin
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("admin should pass AdminMiddleware, got %d", w.Code)
	}
}
