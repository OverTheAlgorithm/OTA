package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"ota/domain/poll"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type mockPollSvcHandler struct {
	getResp *poll.PollForUser
	getErr  error
	voteErr error
}

func (m *mockPollSvcHandler) GetForUser(ctx context.Context, userID string, id uuid.UUID) (*poll.PollForUser, error) {
	return m.getResp, m.getErr
}
func (m *mockPollSvcHandler) Vote(ctx context.Context, userID string, id uuid.UUID, idx int) error {
	return m.voteErr
}

func pollTestRouter(h *PollHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h.RegisterRoutes(r.Group("/api/v1/polls"))
	return r
}

func pollTestAuthMW(c *gin.Context)    { c.Set("userID", "test-user"); c.Next() }
func pollTestOptAuthMW(c *gin.Context) { c.Next() }

func TestPollHandler_Get_NotFound(t *testing.T) {
	h := NewPollHandler(&mockPollSvcHandler{getResp: nil}, pollTestAuthMW, pollTestOptAuthMW)
	r := pollTestRouter(h)
	req := httptest.NewRequest("GET", "/api/v1/polls/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404 got %d", w.Code)
	}
}

func TestPollHandler_Get_InvalidUUID400(t *testing.T) {
	h := NewPollHandler(&mockPollSvcHandler{}, pollTestAuthMW, pollTestOptAuthMW)
	r := pollTestRouter(h)
	req := httptest.NewRequest("GET", "/api/v1/polls/not-a-uuid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400 got %d", w.Code)
	}
}

func TestPollHandler_Get_Success(t *testing.T) {
	resp := &poll.PollForUser{
		ID: uuid.New(), ContextItemID: uuid.New(),
		Question: "Q", Options: []string{"a", "b"},
		Tallies: []poll.VoteTally{{0, 3}, {1, 2}}, TotalVotes: 5,
	}
	h := NewPollHandler(&mockPollSvcHandler{getResp: resp}, pollTestAuthMW, pollTestOptAuthMW)
	r := pollTestRouter(h)
	req := httptest.NewRequest("GET", "/api/v1/polls/"+resp.ContextItemID.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 got %d: %s", w.Code, w.Body.String())
	}
}

func TestPollHandler_Vote_InvalidOption400(t *testing.T) {
	h := NewPollHandler(&mockPollSvcHandler{voteErr: poll.ErrInvalidOption}, pollTestAuthMW, pollTestOptAuthMW)
	r := pollTestRouter(h)
	body, _ := json.Marshal(map[string]int{"option_index": 99})
	req := httptest.NewRequest("POST", "/api/v1/polls/"+uuid.New().String()+"/vote", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400 got %d", w.Code)
	}
}

func TestPollHandler_Vote_NotFound404(t *testing.T) {
	h := NewPollHandler(&mockPollSvcHandler{voteErr: poll.ErrNotFound}, pollTestAuthMW, pollTestOptAuthMW)
	r := pollTestRouter(h)
	body, _ := json.Marshal(map[string]int{"option_index": 0})
	req := httptest.NewRequest("POST", "/api/v1/polls/"+uuid.New().String()+"/vote", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404 got %d", w.Code)
	}
}

func TestPollHandler_Vote_Duplicate409(t *testing.T) {
	h := NewPollHandler(&mockPollSvcHandler{
		voteErr: poll.ErrAlreadyVoted,
		getResp: &poll.PollForUser{Tallies: []poll.VoteTally{}},
	}, pollTestAuthMW, pollTestOptAuthMW)
	r := pollTestRouter(h)
	body, _ := json.Marshal(map[string]int{"option_index": 0})
	req := httptest.NewRequest("POST", "/api/v1/polls/"+uuid.New().String()+"/vote", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("want 409 got %d", w.Code)
	}
}

func TestPollHandler_Vote_Success200(t *testing.T) {
	h := NewPollHandler(&mockPollSvcHandler{
		voteErr: nil,
		getResp: &poll.PollForUser{Options: []string{"a", "b"}, Tallies: []poll.VoteTally{{0, 1}, {1, 0}}, TotalVotes: 1},
	}, pollTestAuthMW, pollTestOptAuthMW)
	r := pollTestRouter(h)
	body, _ := json.Marshal(map[string]int{"option_index": 0})
	req := httptest.NewRequest("POST", "/api/v1/polls/"+uuid.New().String()+"/vote", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 got %d: %s", w.Code, w.Body.String())
	}
}
