package handler

import (
	"net/http"

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

	limit, offset := parsePageParams(c, 20, 100)

	txns, err := h.levelService.GetCoinHistory(c.Request.Context(), userID, limit+1, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "코인 내역을 불러올 수 없습니다"})
		return
	}
	hasMore := len(txns) > limit
	if hasMore {
		txns = txns[:limit]
	}
	if txns == nil {
		txns = []level.CoinTransaction{}
	}

	c.JSON(http.StatusOK, gin.H{
		"data":     txns,
		"has_more": hasMore,
	})
}
