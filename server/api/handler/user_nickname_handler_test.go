package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"

	userDomain "ota/domain/user"
)

// fakeNicknameRepo implements userDomain.Repository just enough to drive
// the two nickname routes. Methods we don't exercise panic so accidental
// calls are loud.
type fakeNicknameRepo struct {
	mu    sync.Mutex
	users map[string]userDomain.User

	updateNicknameErr error
	acknowledgeErr    error
}

func newFakeNicknameRepo() *fakeNicknameRepo {
	return &fakeNicknameRepo{users: map[string]userDomain.User{}}
}

func (f *fakeNicknameRepo) seed(id, nickname, state string) {
	f.users[id] = userDomain.User{ID: id, Nickname: nickname, NicknameState: state}
}

func (f *fakeNicknameRepo) FindByID(_ context.Context, id string) (userDomain.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.users[id]
	if !ok {
		return userDomain.User{}, errors.New("user not found")
	}
	return u, nil
}

func (f *fakeNicknameRepo) UpdateNickname(_ context.Context, id, nickname string) error {
	if f.updateNicknameErr != nil {
		return f.updateNicknameErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.users[id]
	if !ok {
		return errors.New("user not found")
	}
	u.Nickname = nickname
	u.NicknameState = userDomain.NicknameStateCustom
	f.users[id] = u
	return nil
}

func (f *fakeNicknameRepo) AcknowledgeNicknameWarning(_ context.Context, id string) error {
	if f.acknowledgeErr != nil {
		return f.acknowledgeErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.users[id]
	if !ok {
		return nil // idempotent
	}
	if u.NicknameState == userDomain.NicknameStateDefault {
		u.NicknameState = userDomain.NicknameStateAcknowledged
		f.users[id] = u
	}
	return nil
}

// The rest of the userDomain.Repository surface area is not exercised.
func (f *fakeNicknameRepo) UpsertByKakaoID(context.Context, int64, string, string, string) (userDomain.User, error) {
	panic("not used")
}
func (f *fakeNicknameRepo) FindByKakaoID(context.Context, int64) (userDomain.User, bool, error) {
	panic("not used")
}
func (f *fakeNicknameRepo) FindByEmail(context.Context, string) (userDomain.User, error) {
	panic("not used")
}
func (f *fakeNicknameRepo) UpdateEmail(context.Context, string, string) error    { panic("not used") }
func (f *fakeNicknameRepo) UpdateRole(context.Context, string, string) error     { panic("not used") }
func (f *fakeNicknameRepo) UpdatePenName(context.Context, string, string) error  { panic("not used") }
func (f *fakeNicknameRepo) DeleteByID(context.Context, string) error             { panic("not used") }

func setupNicknameRouter(t *testing.T, callerID string, repo *fakeNicknameRepo) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	h := &UserDeliveryChannelsHandler{userRepo: repo}
	r := gin.New()
	r.Use(func(c *gin.Context) {
		if callerID != "" {
			c.Set("userID", callerID)
		}
		c.Next()
	})
	group := r.Group("/api/v1/user")
	h.RegisterRoutes(group)
	return r
}

func doNicknameRequest(t *testing.T, r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	r.ServeHTTP(w, req)
	return w
}

func TestUpdateNickname_AnonymousRejected(t *testing.T) {
	repo := newFakeNicknameRepo()
	r := setupNicknameRouter(t, "", repo)
	w := doNicknameRequest(t, r, http.MethodPut, "/api/v1/user/nickname", `{"nickname":"newname"}`)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestUpdateNickname_EmptyRejected(t *testing.T) {
	repo := newFakeNicknameRepo()
	repo.seed("alice", "kakao_alice", userDomain.NicknameStateDefault)
	r := setupNicknameRouter(t, "alice", repo)
	w := doNicknameRequest(t, r, http.MethodPut, "/api/v1/user/nickname", `{"nickname":"   "}`)
	// gin's binding:"required" treats whitespace as present; service
	// validates emptiness and returns 400.
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 (body=%s)", w.Code, w.Body.String())
	}
}

func TestUpdateNickname_TooShortRejected(t *testing.T) {
	repo := newFakeNicknameRepo()
	repo.seed("alice", "kakao_alice", userDomain.NicknameStateDefault)
	r := setupNicknameRouter(t, "alice", repo)
	w := doNicknameRequest(t, r, http.MethodPut, "/api/v1/user/nickname", `{"nickname":"a"}`)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestUpdateNickname_TooLongRejected(t *testing.T) {
	repo := newFakeNicknameRepo()
	repo.seed("alice", "kakao_alice", userDomain.NicknameStateDefault)
	r := setupNicknameRouter(t, "alice", repo)
	tooLong := strings.Repeat("a", userDomain.MaxNicknameLen+1)
	body, _ := json.Marshal(map[string]string{"nickname": tooLong})
	w := doNicknameRequest(t, r, http.MethodPut, "/api/v1/user/nickname", string(body))
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestUpdateNickname_AdvancesStateToCustom(t *testing.T) {
	repo := newFakeNicknameRepo()
	repo.seed("alice", "kakao_alice", userDomain.NicknameStateDefault)
	r := setupNicknameRouter(t, "alice", repo)
	w := doNicknameRequest(t, r, http.MethodPut, "/api/v1/user/nickname", `{"nickname":"customAlice"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Data userDomain.User `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Data.Nickname != "customAlice" {
		t.Errorf("nickname = %q, want customAlice", resp.Data.Nickname)
	}
	if resp.Data.NicknameState != userDomain.NicknameStateCustom {
		t.Errorf("state = %q, want custom", resp.Data.NicknameState)
	}
}

func TestUpdateNickname_TrimsWhitespace(t *testing.T) {
	repo := newFakeNicknameRepo()
	repo.seed("alice", "kakao_alice", userDomain.NicknameStateDefault)
	r := setupNicknameRouter(t, "alice", repo)
	body, _ := json.Marshal(map[string]string{"nickname": "  bobcat  "})
	w := doNicknameRequest(t, r, http.MethodPut, "/api/v1/user/nickname", string(body))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	if repo.users["alice"].Nickname != "bobcat" {
		t.Errorf("stored nickname = %q, want trimmed 'bobcat'", repo.users["alice"].Nickname)
	}
}

func TestUpdateNickname_InvalidJSONRejected(t *testing.T) {
	repo := newFakeNicknameRepo()
	repo.seed("alice", "kakao_alice", userDomain.NicknameStateDefault)
	r := setupNicknameRouter(t, "alice", repo)
	w := doNicknameRequest(t, r, http.MethodPut, "/api/v1/user/nickname", `not json`)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestDismissNicknameWarning_AnonymousRejected(t *testing.T) {
	repo := newFakeNicknameRepo()
	r := setupNicknameRouter(t, "", repo)
	w := doNicknameRequest(t, r, http.MethodPost, "/api/v1/user/nickname-warning/dismiss", "")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestDismissNicknameWarning_AdvancesDefaultToAcknowledged(t *testing.T) {
	repo := newFakeNicknameRepo()
	repo.seed("alice", "kakao_alice", userDomain.NicknameStateDefault)
	r := setupNicknameRouter(t, "alice", repo)
	w := doNicknameRequest(t, r, http.MethodPost, "/api/v1/user/nickname-warning/dismiss", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	if repo.users["alice"].NicknameState != userDomain.NicknameStateAcknowledged {
		t.Errorf("state = %q, want acknowledged", repo.users["alice"].NicknameState)
	}
}

func TestDismissNicknameWarning_NoOpOnCustom(t *testing.T) {
	repo := newFakeNicknameRepo()
	repo.seed("alice", "alice_custom", userDomain.NicknameStateCustom)
	r := setupNicknameRouter(t, "alice", repo)
	w := doNicknameRequest(t, r, http.MethodPost, "/api/v1/user/nickname-warning/dismiss", "")
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if repo.users["alice"].NicknameState != userDomain.NicknameStateCustom {
		t.Errorf("state regressed to %q from custom", repo.users["alice"].NicknameState)
	}
}

func TestDismissNicknameWarning_Idempotent(t *testing.T) {
	repo := newFakeNicknameRepo()
	repo.seed("alice", "kakao_alice", userDomain.NicknameStateAcknowledged)
	r := setupNicknameRouter(t, "alice", repo)

	w1 := doNicknameRequest(t, r, http.MethodPost, "/api/v1/user/nickname-warning/dismiss", "")
	if w1.Code != http.StatusOK {
		t.Errorf("first dismiss status = %d, want 200", w1.Code)
	}
	w2 := doNicknameRequest(t, r, http.MethodPost, "/api/v1/user/nickname-warning/dismiss", "")
	if w2.Code != http.StatusOK {
		t.Errorf("second dismiss status = %d, want 200 (idempotent)", w2.Code)
	}
	if repo.users["alice"].NicknameState != userDomain.NicknameStateAcknowledged {
		t.Errorf("state = %q, want still acknowledged", repo.users["alice"].NicknameState)
	}
}

// staticBuffer is a tiny helper for cases when we want a non-nil reader for
// a body-less request without depending on bytes.NewReader elsewhere.
var _ = bytes.NewReader([]byte(nil))
