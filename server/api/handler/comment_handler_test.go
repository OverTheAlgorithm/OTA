package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"ota/cache"
	"ota/domain/comment"
)

// memRepoForHandler is a minimal in-memory comment.Repository used by the
// handler tests. It exists in this file because the existing comment
// service tests in domain/comment already cover repo contract edge cases.
type memRepoForHandler struct {
	roots    map[uuid.UUID]comment.Comment
	replies  map[uuid.UUID]comment.Comment
	lastRank map[uuid.UUID]string
}

func newMemRepoForHandler() *memRepoForHandler {
	return &memRepoForHandler{
		roots:    map[uuid.UUID]comment.Comment{},
		replies:  map[uuid.UUID]comment.Comment{},
		lastRank: map[uuid.UUID]string{},
	}
}

func (m *memRepoForHandler) InsertRoot(_ context.Context, c comment.Comment) (comment.Comment, error) {
	c.AuthorNickname = "alice"
	m.roots[c.ID] = c
	return c, nil
}
func (m *memRepoForHandler) InsertReply(_ context.Context, c comment.Comment) (comment.Comment, error) {
	c.AuthorNickname = "alice"
	m.replies[c.ID] = c
	m.lastRank[c.GroupID] = c.RankKey
	return c, nil
}
func (m *memRepoForHandler) GetByID(_ context.Context, id uuid.UUID) (comment.Comment, error) {
	if c, ok := m.roots[id]; ok {
		c.AuthorNickname = "alice"
		return c, nil
	}
	if c, ok := m.replies[id]; ok {
		c.AuthorNickname = "alice"
		return c, nil
	}
	return comment.Comment{}, comment.ErrCommentNotFound
}
func (m *memRepoForHandler) ListRoots(_ context.Context, target comment.TargetType, targetID uuid.UUID, _ comment.SortOrder, _ string, _ int) (comment.RootPage, error) {
	items := []comment.Comment{}
	for _, c := range m.roots {
		if c.TargetType == target && c.TargetID == targetID && c.Depth == 0 {
			c.AuthorNickname = "alice"
			items = append(items, c)
		}
	}
	return comment.RootPage{Items: items}, nil
}
func (m *memRepoForHandler) ListReplies(_ context.Context, groupID uuid.UUID, _ string, _ int) (comment.ReplyPage, error) {
	items := []comment.Comment{}
	for _, c := range m.replies {
		if c.GroupID == groupID && c.Depth == 1 {
			c.AuthorNickname = "alice"
			items = append(items, c)
		}
	}
	return comment.ReplyPage{Items: items}, nil
}
func (m *memRepoForHandler) LastReplyRankKey(_ context.Context, groupID uuid.UUID) (string, error) {
	return m.lastRank[groupID], nil
}
func (m *memRepoForHandler) CountReplies(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]int, error) {
	out := map[uuid.UUID]int{}
	for _, id := range ids {
		count := 0
		for _, c := range m.replies {
			if c.GroupID == id {
				count++
			}
		}
		out[id] = count
	}
	return out, nil
}
func (m *memRepoForHandler) UpdateContent(_ context.Context, id uuid.UUID, content string) error {
	if c, ok := m.roots[id]; ok {
		c.Content = content
		m.roots[id] = c
		return nil
	}
	if c, ok := m.replies[id]; ok {
		c.Content = content
		m.replies[id] = c
		return nil
	}
	return comment.ErrCommentNotFound
}
func (m *memRepoForHandler) SoftDelete(_ context.Context, id uuid.UUID) error {
	if _, ok := m.roots[id]; ok {
		delete(m.roots, id)
		return nil
	}
	if _, ok := m.replies[id]; ok {
		delete(m.replies, id)
		return nil
	}
	return comment.ErrCommentNotFound
}
func (m *memRepoForHandler) ApplyCounters(_ context.Context, _ uuid.UUID, _, _ int) error { return nil }
func (m *memRepoForHandler) UpsertReactions(_ context.Context, _ uuid.UUID, _ []comment.ReactionRow) error {
	return nil
}

// staticAuth sets userID/role in context to fixed values; tests for
// anonymous flows use a no-op middleware that sets nothing.
func staticAuth(userID, role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if userID != "" {
			c.Set("userID", userID)
		}
		if role != "" {
			c.Set("role", role)
		}
		c.Next()
	}
}

func setupHandler(t *testing.T, callerID string) (*gin.Engine, *comment.Service, uuid.UUID) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	repo := newMemRepoForHandler()
	store := cache.NewMemoryReactionStore()
	svc := comment.NewService(repo, store, map[comment.TargetType]comment.TargetValidator{
		comment.TargetTopic:      alwaysExists{},
		comment.TargetEditorPick: alwaysExists{},
	})
	authMW := staticAuth(callerID, "user")
	optMW := staticAuth(callerID, "user")
	h := NewCommentHandler(svc, authMW, optMW, nil)

	r := gin.New()
	group := r.Group("/api/v1/comments")
	h.RegisterRoutes(group)

	var caller uuid.UUID
	if callerID != "" {
		caller, _ = uuid.Parse(callerID)
	}
	return r, svc, caller
}

type alwaysExists struct{}

func (alwaysExists) Exists(_ context.Context, _ uuid.UUID) (bool, error) { return true, nil }

func doJSON(r *gin.Engine, method, path string, body any) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req, _ := http.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func TestHandler_AnonymousCannotCreate(t *testing.T) {
	r, _, _ := setupHandler(t, "")
	w := doJSON(r, http.MethodPost, "/api/v1/comments", map[string]any{
		"target_type": "topic",
		"target_id":   uuid.New().String(),
		"content":     "hi",
	})
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestHandler_CreateRootSucceeds(t *testing.T) {
	userID := uuid.New().String()
	r, _, _ := setupHandler(t, userID)
	target := uuid.New().String()
	w := doJSON(r, http.MethodPost, "/api/v1/comments", map[string]any{
		"target_type": "topic",
		"target_id":   target,
		"content":     "hello",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Data commentResponse `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, w.Body.String())
	}
	if resp.Data.Depth != 0 || resp.Data.Content != "hello" {
		t.Errorf("response wrong: %+v", resp.Data)
	}
}

func TestHandler_InvalidTargetRejected(t *testing.T) {
	userID := uuid.New().String()
	r, _, _ := setupHandler(t, userID)
	w := doJSON(r, http.MethodPost, "/api/v1/comments", map[string]any{
		"target_type": "post",
		"target_id":   uuid.New().String(),
		"content":     "hi",
	})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400, body=%s", w.Code, w.Body.String())
	}
}

func TestHandler_ListRequiresValidTarget(t *testing.T) {
	r, _, _ := setupHandler(t, "")
	w := doJSON(r, http.MethodGet, "/api/v1/comments?target_type=post&target_id="+uuid.New().String(), nil)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestHandler_DeleteByNonOwnerForbidden(t *testing.T) {
	owner := uuid.New().String()
	intruder := uuid.New().String()

	// Build with owner so the comment is created in the owner's repo.
	_, svc, _ := setupHandler(t, owner)
	target := uuid.New()
	cm, _ := svc.Create(context.Background(), uuid.MustParse(owner), comment.TargetTopic, target, nil, "x")

	// Ownership enforcement lives in the service; assert via the service
	// rather than swapping handlers (which would lose repo state).
	if err := svc.Delete(context.Background(), uuid.MustParse(intruder), cm.ID); err == nil {
		t.Error("expected ErrNotOwner via service")
	}
}

func TestHandler_ReactRequiresAuth(t *testing.T) {
	r, _, _ := setupHandler(t, "")
	w := doJSON(r, http.MethodPost, "/api/v1/comments/"+uuid.New().String()+"/react", map[string]any{
		"reaction": 1,
	})
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestHandler_ReactInvalidValueRejected(t *testing.T) {
	user := uuid.New().String()
	r, svc, callerID := setupHandler(t, user)
	cm, _ := svc.Create(context.Background(), callerID, comment.TargetTopic, uuid.New(), nil, "x")
	w := doJSON(r, http.MethodPost, "/api/v1/comments/"+cm.ID.String()+"/react", map[string]any{"reaction": 5})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400, body=%s", w.Code, w.Body.String())
	}
}

func TestHandler_ListRespondsWithReplyCount(t *testing.T) {
	user := uuid.New().String()
	r, svc, callerID := setupHandler(t, user)
	target := uuid.New()
	root, _ := svc.Create(context.Background(), callerID, comment.TargetTopic, target, nil, "root")
	_, _ = svc.Create(context.Background(), callerID, comment.TargetTopic, target, &root.ID, "r1")
	_, _ = svc.Create(context.Background(), callerID, comment.TargetTopic, target, &root.ID, "r2")

	w := doJSON(r, http.MethodGet, "/api/v1/comments?target_type=topic&target_id="+target.String(), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Data listResponse `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Data.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(resp.Data.Items))
	}
	if resp.Data.Items[0].ReplyCount != 2 {
		t.Errorf("reply_count = %d, want 2", resp.Data.Items[0].ReplyCount)
	}
}

func TestHandler_ListIncludesMyReactionWhenAuthed(t *testing.T) {
	user := uuid.New().String()
	r, svc, callerID := setupHandler(t, user)
	target := uuid.New()
	root, _ := svc.Create(context.Background(), callerID, comment.TargetTopic, target, nil, "root")
	_, _ = svc.React(context.Background(), callerID, root.ID, comment.ReactionLike)

	w := doJSON(r, http.MethodGet, "/api/v1/comments?target_type=topic&target_id="+target.String(), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Data listResponse `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Data.Items) != 1 || resp.Data.Items[0].MyReaction != int(comment.ReactionLike) {
		t.Errorf("my_reaction = %+v, want like", resp.Data.Items)
	}
	if resp.Data.Items[0].LikesCount != 1 {
		t.Errorf("likes_count = %d, want 1", resp.Data.Items[0].LikesCount)
	}
}
