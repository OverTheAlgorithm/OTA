package handler

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"ota/cache"
	"ota/domain/collector"
	"ota/domain/level"
)

// EarnPending holds the state stored in the cache while waiting for dwell-time confirmation.
type EarnPending struct {
	InitiatedAt   time.Time
	UID           string
	ContextItemID uuid.UUID
	RunID         uuid.UUID
}

// earnCacheKey returns the canonical cache key for a given user and context item.
func earnCacheKey(uid string, contextItemID uuid.UUID) string {
	return fmt.Sprintf("earn:%s:%s", uid, contextItemID)
}

// LevelHandler handles coin-earning and level queries.
type LevelHandler struct {
	service         *level.Service
	histRepo        collector.HistoryRepository
	subGetter       SubscriptionGetter
	earnCache       cache.Cache
	earnMinDuration time.Duration
	authMW          gin.HandlerFunc
}

func NewLevelHandler(
	service *level.Service,
	histRepo collector.HistoryRepository,
	subGetter SubscriptionGetter,
	earnCache cache.Cache,
	earnMinDuration time.Duration,
	authMW gin.HandlerFunc,
) *LevelHandler {
	return &LevelHandler{
		service:         service,
		histRepo:        histRepo,
		subGetter:       subGetter,
		earnCache:       earnCache,
		earnMinDuration: earnMinDuration,
		authMW:          authMW,
	}
}

// GetLevel handles GET /api/v1/level
func (h *LevelHandler) GetLevel(c *gin.Context) {
	userID := c.GetString("userID")
	info, err := h.service.GetLevel(c.Request.Context(), userID)
	if err != nil {
		log.Printf("get level error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": info})
}

// InitEarn handles POST /api/v1/level/init-earn
// Public endpoint — called when user first visits a topic page.
// Validates all earn-eligibility conditions and, on success, records a
// pending entry in the cache so that the subsequent /earn call can verify
// the user's dwell time.
func (h *LevelHandler) InitEarn(c *gin.Context) {
	var req struct {
		UID           string `json:"uid" binding:"required"`
		ContextItemID string `json:"context_item_id" binding:"required"`
		RunID         string `json:"run_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "uid, context_item_id, and run_id are required"})
		return
	}

	itemID, err := uuid.Parse(req.ContextItemID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid context_item_id"})
		return
	}
	runID, err := uuid.Parse(req.RunID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid run_id"})
		return
	}

	ctx := c.Request.Context()

	// ── Gate check 1: run must be from today ──────────────────────────────────
	isToday, err := h.histRepo.IsRunCreatedToday(ctx, runID)
	if err != nil {
		log.Printf("init-earn: check run date error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if !isToday {
		c.JSON(http.StatusOK, gin.H{"data": gin.H{"status": "EXPIRED"}})
		return
	}

	// ── Gate check 2: context item must exist ────────────────────────────────
	topic, err := h.histRepo.GetContextItemByID(ctx, itemID)
	if err != nil {
		log.Printf("init-earn: get context item error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if topic == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "context item not found"})
		return
	}

	// ── Gate check 3: must not already have earned for this run+item ─────────
	earned, err := h.service.HasEarned(ctx, req.UID, runID, itemID)
	if err != nil {
		log.Printf("init-earn: has earned error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if earned {
		c.JSON(http.StatusOK, gin.H{"data": gin.H{"status": "DUPLICATE"}})
		return
	}

	// ── Gate check 4: daily coin limit ───────────────────────────────────────
	limited, err := h.service.IsAtDailyLimit(ctx, req.UID)
	if err != nil {
		log.Printf("init-earn: daily limit check error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if limited {
		c.JSON(http.StatusOK, gin.H{"data": gin.H{"status": "DAILY_LIMIT"}})
		return
	}

	// ── All checks passed — store/reset pending entry in cache ───────────────
	pending := EarnPending{
		InitiatedAt:   time.Now(),
		UID:           req.UID,
		ContextItemID: itemID,
		RunID:         runID,
	}
	ttl := h.earnMinDuration * 2
	h.earnCache.Set(earnCacheKey(req.UID, itemID), pending, ttl)

	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"status":           "PENDING",
		"required_seconds": int(h.earnMinDuration.Seconds()),
	}})
}

// EarnCoin handles POST /api/v1/level/earn
// Public endpoint — final confirmation step after the user has dwelled long
// enough. Verifies cache presence and elapsed time before awarding coins.
func (h *LevelHandler) EarnCoin(c *gin.Context) {
	var req struct {
		UID           string `json:"uid" binding:"required"`
		ContextItemID string `json:"context_item_id" binding:"required"`
		RunID         string `json:"run_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "uid, context_item_id, and run_id are required"})
		return
	}

	itemID, err := uuid.Parse(req.ContextItemID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid context_item_id"})
		return
	}
	runID, err := uuid.Parse(req.RunID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid run_id"})
		return
	}

	// ── Cache dwell check ─────────────────────────────────────────────────────
	key := earnCacheKey(req.UID, itemID)
	raw, ok := h.earnCache.Get(key)
	if !ok {
		// No init-earn record — either never called or already consumed/expired.
		c.JSON(http.StatusBadRequest, gin.H{"error": "TOO_EARLY"})
		return
	}
	pending, ok := raw.(EarnPending)
	if !ok || time.Since(pending.InitiatedAt) < h.earnMinDuration {
		c.JSON(http.StatusBadRequest, gin.H{"error": "TOO_EARLY"})
		return
	}

	ctx := c.Request.Context()

	type earnResponse struct {
		Attempted   bool   `json:"attempted"`
		Earned      bool   `json:"earned"`
		Reason      string `json:"reason"`
		CoinsEarned int    `json:"coins_earned"`
		LeveledUp   bool   `json:"leveled_up"`
		NewLevel    int    `json:"new_level"`
	}

	// ── Re-validate eligibility (state may have changed since init) ──────────
	isToday, err := h.histRepo.IsRunCreatedToday(ctx, runID)
	if err != nil {
		log.Printf("earn coin: check run date error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if !isToday {
		h.earnCache.Delete(key)
		c.JSON(http.StatusOK, gin.H{"data": earnResponse{Attempted: true, Reason: "EXPIRED"}})
		return
	}

	topic, err := h.histRepo.GetContextItemByID(ctx, itemID)
	if err != nil {
		log.Printf("earn coin: get context item error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if topic == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "context item not found"})
		return
	}

	subs, err := h.subGetter.GetSubscriptions(ctx, req.UID)
	if err != nil {
		log.Printf("earn coin: get subscriptions error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	preferred := level.IsPreferredCategory(topic.Category, subs)

	result, err := h.service.EarnCoin(ctx, req.UID, runID, itemID, preferred)
	if err != nil {
		log.Printf("earn coin error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	// Remove the pending entry so a second call is rejected.
	h.earnCache.Delete(key)

	c.JSON(http.StatusOK, gin.H{"data": earnResponse{
		Attempted:   true,
		Earned:      result.Earned,
		Reason:      result.Reason,
		CoinsEarned: result.CoinsEarned,
		LeveledUp:   result.LeveledUp,
		NewLevel:    result.Level,
	}})
}

func (h *LevelHandler) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("", h.authMW, h.GetLevel)  // GET  /api/v1/level
	group.POST("/init-earn", h.InitEarn) // POST /api/v1/level/init-earn (public)
	group.POST("/earn", h.EarnCoin)      // POST /api/v1/level/earn (public)
}
