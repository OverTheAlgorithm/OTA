package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"ota/domain/level"
)

// MypageHandler handles user mypage endpoints.
type MypageHandler struct {
	levelService *level.Service
	authMW       gin.HandlerFunc
}

func NewMypageHandler(levelService *level.Service, authMW gin.HandlerFunc) *MypageHandler {
	return &MypageHandler{levelService: levelService, authMW: authMW}
}

func (h *MypageHandler) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/coin-history", h.authMW, h.GetCoinHistory)
}

// GetCoinHistory returns a paginated timeline of all coin balance changes.
func (h *MypageHandler) GetCoinHistory(c *gin.Context) {
	userID := c.GetString("userID")

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	txns, err := h.levelService.GetCoinHistory(c.Request.Context(), userID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "코인 내역을 불러올 수 없습니다"})
		return
	}
	if txns == nil {
		txns = []level.CoinTransaction{}
	}

	c.JSON(http.StatusOK, gin.H{
		"data":     txns,
		"has_more": len(txns) == limit,
	})
}
