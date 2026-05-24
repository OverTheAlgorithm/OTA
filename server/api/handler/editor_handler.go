package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"ota/domain/editor"
	"ota/domain/user"
)

// EditorHandler exposes CRUD endpoints for editor posts. All routes assume the
// caller has been authenticated and authorised as editor+ by middleware.
type EditorHandler struct {
	svc      *editor.Service
	upload   *EditorUploadHandler
	userRepo user.Repository
}

func NewEditorHandler(svc *editor.Service) *EditorHandler {
	return &EditorHandler{svc: svc}
}

// WithUploadHandler attaches the image upload handler so it can be registered
// alongside the CRUD routes from a single RouteModule entry.
func (h *EditorHandler) WithUploadHandler(u *EditorUploadHandler) *EditorHandler {
	h.upload = u
	return h
}

// WithUserRepo enables the profile sub-routes (pen-name editor) that need
// direct access to the users table. Optional so existing unit tests can
// construct the handler without a user repository.
func (h *EditorHandler) WithUserRepo(repo user.Repository) *EditorHandler {
	h.userRepo = repo
	return h
}

func (h *EditorHandler) RegisterRoutes(group *gin.RouterGroup) {
	group.POST("/posts", h.Create)
	group.GET("/posts", h.List)
	group.GET("/posts/:id", h.Get)
	group.PUT("/posts/:id", h.Update)
	group.DELETE("/posts/:id", h.Delete)
	if h.userRepo != nil {
		group.PUT("/profile/pen-name", h.UpdatePenName)
	}
	if h.upload != nil {
		h.upload.RegisterRoutes(group)
	}
}

type penNameRequest struct {
	PenName string `json:"pen_name"`
}

// UpdatePenName handles PUT /api/v1/editor/profile/pen-name. The caller is
// already known to be editor+ via the route's middleware. An empty (or
// whitespace-only) value clears the pen name so the byline falls back to the
// caller's nickname.
func (h *EditorHandler) UpdatePenName(c *gin.Context) {
	callerID := c.GetString("userID")

	var req penNameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "요청 형식이 올바르지 않습니다"})
		return
	}

	normalised, err := user.NormalisePenName(req.PenName)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.userRepo.UpdatePenName(c.Request.Context(), callerID, normalised); err != nil {
		if errors.Is(err, user.ErrPenNameTaken) {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		slog.Error("update pen name", "user_id", callerID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "필명 변경 중 오류가 발생했습니다"})
		return
	}

	u, err := h.userRepo.FindByID(c.Request.Context(), callerID)
	if err != nil {
		slog.Error("reload user after pen name update", "user_id", callerID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "유저 정보를 다시 불러올 수 없습니다"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": u})
}

type editorPostRequest struct {
	Title       string `json:"title" binding:"required"`
	ContentHTML string `json:"content_html" binding:"required"`
	Status      string `json:"status" binding:"required"`
}

// Create handles POST /api/v1/editor/posts.
func (h *EditorHandler) Create(c *gin.Context) {
	authorID := c.GetString("userID")

	var req editorPostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title, content_html, status는 필수입니다"})
		return
	}

	post, err := h.svc.Create(c.Request.Context(), editor.CreateParams{
		AuthorID:    authorID,
		Title:       req.Title,
		ContentHTML: req.ContentHTML,
		Status:      req.Status,
	})
	if err != nil {
		respondEditorError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": post})
}

// List handles GET /api/v1/editor/posts (own posts; admin sees all).
func (h *EditorHandler) List(c *gin.Context) {
	callerID := c.GetString("userID")
	callerRole := c.GetString("role")

	posts, err := h.svc.ListForCaller(c.Request.Context(), callerID, callerRole)
	if err != nil {
		slog.Error("editor list error", "user_id", callerID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "글 목록을 불러올 수 없습니다"})
		return
	}
	if posts == nil {
		posts = []editor.Post{}
	}
	c.JSON(http.StatusOK, gin.H{"data": posts})
}

// Get handles GET /api/v1/editor/posts/:id (owner or admin).
func (h *EditorHandler) Get(c *gin.Context) {
	id := c.Param("id")
	callerID := c.GetString("userID")
	callerRole := c.GetString("role")

	post, err := h.svc.GetForEdit(c.Request.Context(), id, callerID, callerRole)
	if err != nil {
		respondEditorError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": post})
}

// Update handles PUT /api/v1/editor/posts/:id.
func (h *EditorHandler) Update(c *gin.Context) {
	id := c.Param("id")
	callerID := c.GetString("userID")
	callerRole := c.GetString("role")

	var req editorPostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title, content_html, status는 필수입니다"})
		return
	}

	post, err := h.svc.Update(c.Request.Context(), id, callerID, callerRole, editor.UpdateParams{
		Title:       req.Title,
		ContentHTML: req.ContentHTML,
		Status:      req.Status,
	})
	if err != nil {
		respondEditorError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": post})
}

// Delete handles DELETE /api/v1/editor/posts/:id.
func (h *EditorHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	callerID := c.GetString("userID")
	callerRole := c.GetString("role")

	if err := h.svc.Delete(c.Request.Context(), id, callerID, callerRole); err != nil {
		respondEditorError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"deleted": true}})
}

// respondEditorError translates domain errors into HTTP responses. Anything
// unrecognised becomes a 500 with a generic message.
func respondEditorError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, editor.ErrPostNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, editor.ErrNotAuthorized):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	case errors.Is(err, editor.ErrTitleRequired),
		errors.Is(err, editor.ErrTitleTooLong),
		errors.Is(err, editor.ErrContentEmpty),
		errors.Is(err, editor.ErrContentTooLong),
		errors.Is(err, editor.ErrInvalidStatus):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		slog.Error("editor handler unexpected error", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "처리 중 오류가 발생했습니다"})
	}
}
