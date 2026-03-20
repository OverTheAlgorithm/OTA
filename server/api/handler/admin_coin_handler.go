package handler

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"ota/domain/level"
	"ota/domain/user"
)

// AdminCoinHandler handles admin coin adjustment endpoints.
type AdminCoinHandler struct {
	userRepo     user.Repository
	levelService *level.Service
}

func NewAdminCoinHandler(userRepo user.Repository, levelService *level.Service) *AdminCoinHandler {
	return &AdminCoinHandler{userRepo: userRepo, levelService: levelService}
}

// SearchUser handles GET /api/v1/admin/coins/search?type=id|email&q=...
func (h *AdminCoinHandler) SearchUser(c *gin.Context) {
	searchType := c.Query("type")
	query := strings.TrimSpace(c.Query("q"))

	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "검색어를 입력해주세요"})
		return
	}

	var (
		u   user.User
		err error
	)

	switch searchType {
	case "id":
		u, err = h.userRepo.FindByID(c.Request.Context(), query)
	case "email":
		u, err = h.userRepo.FindByEmail(c.Request.Context(), query)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "검색 타입은 id 또는 email이어야 합니다"})
		return
	}

	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": "해당 유저를 찾을 수 없습니다"})
			return
		}
		slog.Error("admin search user error", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "검색 중 오류가 발생했습니다"})
		return
	}

	// Also fetch level info
	levelInfo, err := h.levelService.GetLevel(c.Request.Context(), u.ID)
	if err != nil {
		slog.Error("admin get user level error", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "레벨 정보를 불러올 수 없습니다"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"user":  u,
			"level": levelInfo,
		},
	})
}

type adjustCoinsRequest struct {
	UserID   string `json:"user_id" binding:"required"`
	NewCoins int    `json:"new_coins" binding:"min=0"`
	Memo     string `json:"memo" binding:"required"`
}

// AdjustCoins handles POST /api/v1/admin/coins/adjust
func (h *AdminCoinHandler) AdjustCoins(c *gin.Context) {
	adminID := c.GetString("userID")

	var req adjustCoinsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id, new_coins(>=0), memo는 필수입니다"})
		return
	}

	memo := strings.TrimSpace(req.Memo)
	if memo == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "비고(memo)는 필수입니다"})
		return
	}

	// Verify target user exists
	targetUser, err := h.userRepo.FindByID(c.Request.Context(), req.UserID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": "해당 유저를 찾을 수 없습니다"})
			return
		}
		slog.Error("admin adjust coins find user error", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "유저 조회 중 오류가 발생했습니다"})
		return
	}

	delta, levelInfo, err := h.levelService.AdjustCoins(c.Request.Context(), req.UserID, req.NewCoins, memo, adminID)
	if err != nil {
		slog.Error("admin adjust coins error", "admin_id", adminID, "target_id", req.UserID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "코인 수정 중 오류가 발생했습니다"})
		return
	}

	slog.Info("admin coin adjustment", "admin_id", adminID, "target_id", targetUser.ID, "nickname", targetUser.Nickname, "delta", delta, "new_coins", req.NewCoins)

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"user_id":   req.UserID,
			"delta":     delta,
			"new_coins": req.NewCoins,
			"level":     levelInfo,
		},
	})
}

func (h *AdminCoinHandler) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/search", h.SearchUser)
	group.POST("/adjust", h.AdjustCoins)
}
