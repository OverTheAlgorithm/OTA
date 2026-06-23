package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"ota/domain/communitytrend"
)

// CommunityTrendAdminHandler exposes admin CRUD for communities, axes, and tags.
type CommunityTrendAdminHandler struct {
	svc *communitytrend.Service
}

func NewCommunityTrendAdminHandler(svc *communitytrend.Service) *CommunityTrendAdminHandler {
	return &CommunityTrendAdminHandler{svc: svc}
}

// RegisterRoutes registers admin routes under /api/v1/admin/community-trend.
func (h *CommunityTrendAdminHandler) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/communities", h.ListCommunities)
	group.POST("/communities", h.CreateCommunity)
	group.PATCH("/communities/:id", h.UpdateCommunity)
	group.DELETE("/communities/:id", h.DeleteCommunity)
	group.PUT("/communities/:id/meta-tags", h.SetMetaTags)

	group.GET("/axes", h.ListAxes)
	group.POST("/axes", h.CreateAxis)

	group.GET("/tags", h.ListTags) // optional ?axis_id=
	group.POST("/tags", h.CreateTag)
	group.PATCH("/tags/:id", h.UpdateTag)
	group.DELETE("/tags/:id", h.DeleteTag)
}

func parseCTIDParam(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "잘못된 ID입니다"})
		return 0, false
	}
	return id, true
}

// --- communities ---

func (h *CommunityTrendAdminHandler) ListCommunities(c *gin.Context) {
	list, err := h.svc.ListCommunities(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "커뮤니티 목록을 불러올 수 없습니다"})
		return
	}
	if list == nil {
		list = []communitytrend.Community{}
	}
	c.JSON(http.StatusOK, gin.H{"data": list})
}

type createCommunityRequest struct {
	Key     string `json:"key"`
	Name    string `json:"name"`
	HomeURL string `json:"home_url"`
}

func (h *CommunityTrendAdminHandler) CreateCommunity(c *gin.Context) {
	var req createCommunityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "잘못된 요청 형식입니다"})
		return
	}
	created, err := h.svc.CreateCommunity(c.Request.Context(), communitytrend.Community{
		Key: req.Key, Name: req.Name, HomeURL: req.HomeURL, Enabled: true,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": created})
}

type updateCommunityRequest struct {
	Name    string `json:"name"`
	HomeURL string `json:"home_url"`
	Enabled *bool  `json:"enabled"`
}

func (h *CommunityTrendAdminHandler) UpdateCommunity(c *gin.Context) {
	id, ok := parseCTIDParam(c)
	if !ok {
		return
	}
	var req updateCommunityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "잘못된 요청 형식입니다"})
		return
	}
	if req.Enabled == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "활성 상태는 필수입니다"})
		return
	}
	updated, err := h.svc.UpdateCommunity(c.Request.Context(), id, req.Name, req.HomeURL, *req.Enabled)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": updated})
}

func (h *CommunityTrendAdminHandler) DeleteCommunity(c *gin.Context) {
	id, ok := parseCTIDParam(c)
	if !ok {
		return
	}
	if err := h.svc.DeleteCommunity(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

type setMetaTagsRequest struct {
	TagIDs []int `json:"tag_ids"`
}

func (h *CommunityTrendAdminHandler) SetMetaTags(c *gin.Context) {
	id, ok := parseCTIDParam(c)
	if !ok {
		return
	}
	var req setMetaTagsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "잘못된 요청 형식입니다"})
		return
	}
	if err := h.svc.SetMetaTags(c.Request.Context(), id, req.TagIDs); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

// --- axes ---

func (h *CommunityTrendAdminHandler) ListAxes(c *gin.Context) {
	list, err := h.svc.ListAxes(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "축 목록을 불러올 수 없습니다"})
		return
	}
	if list == nil {
		list = []communitytrend.Axis{}
	}
	c.JSON(http.StatusOK, gin.H{"data": list})
}

type createAxisRequest struct {
	Key          string `json:"key"`
	Label        string `json:"label"`
	DisplayOrder int    `json:"display_order"`
}

func (h *CommunityTrendAdminHandler) CreateAxis(c *gin.Context) {
	var req createAxisRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "잘못된 요청 형식입니다"})
		return
	}
	created, err := h.svc.CreateAxis(c.Request.Context(), communitytrend.Axis{
		Key: req.Key, Label: req.Label, DisplayOrder: req.DisplayOrder,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": created})
}

// --- tags ---

func (h *CommunityTrendAdminHandler) ListTags(c *gin.Context) {
	ctx := c.Request.Context()
	if raw := c.Query("axis_id"); raw != "" {
		axisID, err := strconv.Atoi(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "잘못된 axis_id입니다"})
			return
		}
		list, err := h.svc.ListTagsByAxis(ctx, axisID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "태그 목록을 불러올 수 없습니다"})
			return
		}
		if list == nil {
			list = []communitytrend.Tag{}
		}
		c.JSON(http.StatusOK, gin.H{"data": list})
		return
	}
	list, err := h.svc.ListTags(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "태그 목록을 불러올 수 없습니다"})
		return
	}
	if list == nil {
		list = []communitytrend.Tag{}
	}
	c.JSON(http.StatusOK, gin.H{"data": list})
}

type createTagRequest struct {
	AxisID      int    `json:"axis_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (h *CommunityTrendAdminHandler) CreateTag(c *gin.Context) {
	var req createTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "잘못된 요청 형식입니다"})
		return
	}
	created, err := h.svc.CreateTag(c.Request.Context(), communitytrend.Tag{
		AxisID: req.AxisID, Name: req.Name, Description: req.Description, CreatedBy: "admin",
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": created})
}

type updateTagRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (h *CommunityTrendAdminHandler) UpdateTag(c *gin.Context) {
	id, ok := parseCTIDParam(c)
	if !ok {
		return
	}
	var req updateTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "잘못된 요청 형식입니다"})
		return
	}
	updated, err := h.svc.UpdateTag(c.Request.Context(), id, req.Name, req.Description)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": updated})
}

func (h *CommunityTrendAdminHandler) DeleteTag(c *gin.Context) {
	id, ok := parseCTIDParam(c)
	if !ok {
		return
	}
	if err := h.svc.DeleteTag(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}
