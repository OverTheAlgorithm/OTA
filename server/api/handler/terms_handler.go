package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"ota/domain/terms"
)

// TermsHandler exposes public and admin endpoints for terms management.
type TermsHandler struct {
	svc *terms.Service
}

// NewTermsHandler creates a new terms handler.
func NewTermsHandler(svc *terms.Service) *TermsHandler {
	return &TermsHandler{svc: svc}
}

// RegisterRoutes registers public terms routes under /api/v1/terms.
func (h *TermsHandler) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/active", h.ListActive)
}

// ListActive returns all active terms for the consent screen.
func (h *TermsHandler) ListActive(c *gin.Context) {
	list, err := h.svc.GetActiveTerms(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "약관 목록을 불러올 수 없습니다"})
		return
	}
	if list == nil {
		list = []terms.Term{}
	}
	c.JSON(http.StatusOK, gin.H{"data": list})
}

// TermsAdminHandler handles admin-only terms endpoints.
type TermsAdminHandler struct {
	svc *terms.Service
}

// NewTermsAdminHandler creates a new admin terms handler.
func NewTermsAdminHandler(svc *terms.Service) *TermsAdminHandler {
	return &TermsAdminHandler{svc: svc}
}

// RegisterRoutes registers admin terms routes under /api/v1/admin/terms.
func (h *TermsAdminHandler) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("", h.ListAll)
	group.POST("", h.Create)
	group.PATCH("/:id/active", h.UpdateActive)
}

// ListAll returns all terms regardless of active status.
func (h *TermsAdminHandler) ListAll(c *gin.Context) {
	list, err := h.svc.ListAllTerms(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "약관 목록을 불러올 수 없습니다"})
		return
	}
	if list == nil {
		list = []terms.Term{}
	}
	c.JSON(http.StatusOK, gin.H{"data": list})
}

type updateActiveRequest struct {
	Active *bool `json:"active"`
}

// UpdateActive toggles the active status of a term.
func (h *TermsAdminHandler) UpdateActive(c *gin.Context) {
	termID := c.Param("id")
	if termID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "약관 ID는 필수입니다"})
		return
	}

	var req updateActiveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "잘못된 요청 형식입니다"})
		return
	}
	if req.Active == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "활성 상태는 필수입니다"})
		return
	}

	if err := h.svc.UpdateTermActive(c.Request.Context(), termID, *req.Active); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

type createTermRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	URL         string `json:"url"`
	Active      *bool  `json:"active"`
	Required    *bool  `json:"required"`
	Version     string `json:"version"`
}

// Create inserts a new immutable term record.
func (h *TermsAdminHandler) Create(c *gin.Context) {
	var req createTermRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "잘못된 요청 형식입니다"})
		return
	}

	if req.Title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "제목은 필수입니다"})
		return
	}
	if req.URL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "URL은 필수입니다"})
		return
	}
	if req.Version == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "버전은 필수입니다"})
		return
	}
	if req.Active == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "활성 상태는 필수입니다"})
		return
	}
	if req.Required == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "필수 여부는 필수입니다"})
		return
	}

	t := terms.Term{
		Title:       req.Title,
		Description: req.Description,
		URL:         req.URL,
		Active:      *req.Active,
		Required:    *req.Required,
		Version:     req.Version,
	}

	created, err := h.svc.CreateTerm(c.Request.Context(), t)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": created})
}
