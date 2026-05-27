package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"ota/domain/editor"
)

// EditorPickHandler serves the public "에디터 픽" list and detail pages.
type EditorPickHandler struct {
	repo editor.Repository
}

// NewEditorPickHandler is intentionally typed against the full
// editor.Repository interface to avoid an extra adapter — only the read
// methods are called.
func NewEditorPickHandler(repo editor.Repository) *EditorPickHandler {
	return &EditorPickHandler{repo: repo}
}

func (h *EditorPickHandler) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("", h.List)
	// `/search` must be registered before `/:id` so Gin's router does not
	// treat "search" as a post ID.
	group.GET("/search", h.Search)
	group.GET("/:id", h.Get)
}

// Search handles GET /api/v1/editor-picks/search?q=&limit=&offset=.
// Mirrors the topic search contract: query is required, trimmed, max 100
// chars. Title matches outrank body matches; ties break by recency.
func (h *EditorPickHandler) Search(c *gin.Context) {
	q := strings.TrimSpace(c.Query("q"))
	if q == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query string 'q' is required"})
		return
	}
	if utf8.RuneCountInString(q) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query string 'q' too long (max 100 chars)"})
		return
	}

	limit, offset := parsePageParams(c, 10, 50)

	cards, hasMore, err := h.repo.SearchPublishedCards(c.Request.Context(), q, limit, offset)
	if err != nil {
		slog.Error("editor-picks search error", "query", q, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if cards == nil {
		cards = []editor.PublicCard{}
	}
	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"items":    cards,
			"has_more": hasMore,
			"query":    q,
		},
	})
}

// List handles GET /api/v1/editor-picks?limit=10&offset=0.
func (h *EditorPickHandler) List(c *gin.Context) {
	limit, offset := parsePageParams(c, 10, 50)

	cards, err := h.repo.ListPublishedCards(c.Request.Context(), limit, offset)
	if err != nil {
		slog.Error("editor-picks list error", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "글 목록을 불러올 수 없습니다"})
		return
	}
	total, err := h.repo.CountPublished(c.Request.Context())
	if err != nil {
		slog.Error("editor-picks count error", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "글 수를 불러올 수 없습니다"})
		return
	}

	if cards == nil {
		cards = []editor.PublicCard{}
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"items":  cards,
			"total":  total,
			"limit":  limit,
			"offset": offset,
		},
	})
}

// Get handles GET /api/v1/editor-picks/:id (published only).
func (h *EditorPickHandler) Get(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "글 ID가 필요합니다"})
		return
	}

	post, err := h.repo.GetPublishedByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, editor.ErrPostNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "글을 찾을 수 없습니다"})
			return
		}
		slog.Error("editor-pick get error", "id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "글을 불러올 수 없습니다"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": post})
}
