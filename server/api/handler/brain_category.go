package handler

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"ota/domain/collector"
)

type BrainCategoryHandler struct {
	repo collector.BrainCategoryRepository
}

func NewBrainCategoryHandler(repo collector.BrainCategoryRepository) *BrainCategoryHandler {
	return &BrainCategoryHandler{repo: repo}
}

// ListAll returns all brain categories ordered by display_order.
// Public endpoint — used by frontend to render groupings.
func (h *BrainCategoryHandler) ListAll(c *gin.Context) {
	categories, err := h.repo.GetAll(c.Request.Context())
	if err != nil {
		log.Printf("list brain categories error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": categories})
}

func (h *BrainCategoryHandler) Create(c *gin.Context) {
	var bc collector.BrainCategory
	if err := c.ShouldBindJSON(&bc); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if bc.Key == "" || bc.Label == "" || bc.Emoji == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "key, emoji, and label are required"})
		return
	}
	if err := h.repo.Create(c.Request.Context(), bc); err != nil {
		log.Printf("create brain category error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create brain category"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": bc})
}

func (h *BrainCategoryHandler) Update(c *gin.Context) {
	key := c.Param("key")
	var bc collector.BrainCategory
	if err := c.ShouldBindJSON(&bc); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	bc.Key = key
	if err := h.repo.Update(c.Request.Context(), bc); err != nil {
		log.Printf("update brain category error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update brain category"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": bc})
}

func (h *BrainCategoryHandler) Delete(c *gin.Context) {
	key := c.Param("key")
	if err := h.repo.Delete(c.Request.Context(), key); err != nil {
		log.Printf("delete brain category error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete brain category"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// RegisterRoutes registers public brain category routes.
func (h *BrainCategoryHandler) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("", h.ListAll)
}

// RegisterAdminRoutes registers admin CRUD routes for brain categories.
func (h *BrainCategoryHandler) RegisterAdminRoutes(group *gin.RouterGroup) {
	group.GET("", h.ListAll)
	group.POST("", h.Create)
	group.PUT("/:key", h.Update)
	group.DELETE("/:key", h.Delete)
}
