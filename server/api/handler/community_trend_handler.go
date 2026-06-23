package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"ota/domain/communitytrend"
)

// CommunityTrendAdminHandler exposes admin CRUD for communities, axes, tags,
// and the daily tagging worksheet flow.
type CommunityTrendAdminHandler struct {
	svc         *communitytrend.Service
	ws          *communitytrend.WorksheetService
	suggestions communitytrend.SuggestionStore
}

func NewCommunityTrendAdminHandler(svc *communitytrend.Service, ws *communitytrend.WorksheetService, suggestions communitytrend.SuggestionStore) *CommunityTrendAdminHandler {
	return &CommunityTrendAdminHandler{svc: svc, ws: ws, suggestions: suggestions}
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

	group.GET("/worksheets", h.ListWorksheets)           // ?date=YYYY-MM-DD
	group.GET("/worksheets/suggestion", h.GetSuggestion) // ?community_id=&date=
	group.POST("/worksheets/confirm", h.ConfirmWorksheet)
}

// GetSuggestion returns the transient AI suggestion for a community-day, if any.
func (h *CommunityTrendAdminHandler) GetSuggestion(c *gin.Context) {
	cid, err := strconv.Atoi(c.Query("community_id"))
	if err != nil || cid <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "community_id가 필요합니다"})
		return
	}
	date, err := time.Parse(ctDateLayout, c.Query("date"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date 형식은 YYYY-MM-DD 입니다"})
		return
	}
	if h.suggestions == nil {
		c.JSON(http.StatusOK, gin.H{"data": nil})
		return
	}
	s, ok, err := h.suggestions.Get(c.Request.Context(), cid, date)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "제안을 불러올 수 없습니다"})
		return
	}
	if !ok {
		c.JSON(http.StatusOK, gin.H{"data": nil})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"output":      s.Output,
		"total_posts": s.TotalPosts,
	}})
}

const ctDateLayout = "2006-01-02"

// ListWorksheets returns the per-community worksheet status board for a date.
func (h *CommunityTrendAdminHandler) ListWorksheets(c *gin.Context) {
	raw := c.Query("date")
	if raw == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date 파라미터(YYYY-MM-DD)가 필요합니다"})
		return
	}
	date, err := time.Parse(ctDateLayout, raw)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date 형식은 YYYY-MM-DD 입니다"})
		return
	}
	list, err := h.ws.ListByDate(c.Request.Context(), date)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "워크시트를 불러올 수 없습니다"})
		return
	}
	if list == nil {
		list = []communitytrend.Worksheet{}
	}
	c.JSON(http.StatusOK, gin.H{"data": list})
}

type confirmWorksheetRequest struct {
	CommunityID int                       `json:"community_id"`
	StatDate    string                    `json:"stat_date"`
	Mode        string                    `json:"mode"`
	Source      string                    `json:"source"`
	TotalPosts  int                       `json:"total_posts"`
	Counts      []communitytrend.TagCount `json:"counts"`
}

// ConfirmWorksheet writes the day's confirmed tag counts atomically.
func (h *CommunityTrendAdminHandler) ConfirmWorksheet(c *gin.Context) {
	var req confirmWorksheetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "잘못된 요청 형식입니다"})
		return
	}
	date, err := time.Parse(ctDateLayout, req.StatDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "stat_date 형식은 YYYY-MM-DD 입니다"})
		return
	}

	var confirmedBy *string
	if uid := c.GetString("userID"); uid != "" {
		confirmedBy = &uid
	}

	// For the auto path, pull the fresh-item fingerprints from the stored AI
	// suggestion so confirming marks them seen (dedup carries to future days).
	var fingerprints []string
	if h.suggestions != nil {
		if s, ok, _ := h.suggestions.Get(c.Request.Context(), req.CommunityID, date); ok {
			fingerprints = s.Fingerprints
		}
	}

	err = h.ws.Confirm(c.Request.Context(), communitytrend.Confirmation{
		CommunityID:  req.CommunityID,
		StatDate:     date,
		Mode:         req.Mode,
		Source:       req.Source,
		TotalPosts:   req.TotalPosts,
		Counts:       req.Counts,
		ConfirmedBy:  confirmedBy,
		Fingerprints: fingerprints,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
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
