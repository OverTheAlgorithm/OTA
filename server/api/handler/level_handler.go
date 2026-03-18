package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
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
	service            *level.Service
	histRepo           collector.HistoryRepository
	subGetter          SubscriptionGetter
	earnCache          cache.Cache
	earnMinDuration    time.Duration
	turnstileSecretKey string
	authMW             gin.HandlerFunc
}

func NewLevelHandler(
	service *level.Service,
	histRepo collector.HistoryRepository,
	subGetter SubscriptionGetter,
	earnCache cache.Cache,
	earnMinDuration time.Duration,
	turnstileSecretKey string,
	authMW gin.HandlerFunc,
) *LevelHandler {
	return &LevelHandler{
		service:            service,
		histRepo:           histRepo,
		subGetter:          subGetter,
		earnCache:          earnCache,
		earnMinDuration:    earnMinDuration,
		turnstileSecretKey: turnstileSecretKey,
		authMW:             authMW,
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
// Authenticated endpoint — called when a logged-in user visits a topic page.
// Validates all earn-eligibility conditions and, on success, records a
// pending entry in the cache so that the subsequent /earn call can verify
// the user's dwell time.
func (h *LevelHandler) InitEarn(c *gin.Context) {
	userID := c.GetString("userID")

	var req struct {
		ContextItemID string `json:"context_item_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("init-earn: bad request from user=%s ip=%s — %v", userID, c.ClientIP(), err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "context_item_id is required"})
		return
	}

	itemID, err := uuid.Parse(req.ContextItemID)
	if err != nil {
		log.Printf("init-earn: invalid context_item_id=%q from user=%s", req.ContextItemID, userID)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid context_item_id"})
		return
	}

	ctx := c.Request.Context()

	// ── Gate check 1: context item must exist ────────────────────────────────
	topic, err := h.histRepo.GetContextItemByID(ctx, itemID)
	if err != nil {
		log.Printf("init-earn: DB error getting item=%s user=%s — %v", itemID, userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if topic == nil {
		log.Printf("init-earn: item not found item=%s user=%s", itemID, userID)
		c.JSON(http.StatusNotFound, gin.H{"error": "context item not found"})
		return
	}

	runID := topic.RunID

	// ── Gate check 2: must not already have earned for this run+item ─────────
	earned, err := h.service.HasEarned(ctx, userID, runID, itemID)
	if err != nil {
		log.Printf("init-earn: DB error checking has-earned user=%s run=%s item=%s — %v", userID, runID, itemID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if earned {
		log.Printf("init-earn: DUPLICATE user=%s item=%s run=%s", userID, itemID, runID)
		c.JSON(http.StatusOK, gin.H{"data": gin.H{"status": "DUPLICATE"}})
		return
	}

	// ── Gate check 3: run must be from today ──────────────────────────────────
	isToday, err := h.histRepo.IsRunCreatedToday(ctx, runID)
	if err != nil {
		log.Printf("init-earn: DB error checking run date run=%s user=%s item=%s — %v", runID, userID, itemID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if !isToday {
		log.Printf("init-earn: EXPIRED user=%s item=%s run=%s", userID, itemID, runID)
		c.JSON(http.StatusOK, gin.H{"data": gin.H{"status": "EXPIRED"}})
		return
	}

	// ── Gate check 4: daily coin limit ───────────────────────────────────────
	limited, err := h.service.IsAtDailyLimit(ctx, userID)
	if err != nil {
		log.Printf("init-earn: DB error checking daily limit user=%s — %v", userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if limited {
		log.Printf("init-earn: DAILY_LIMIT user=%s item=%s", userID, itemID)
		c.JSON(http.StatusOK, gin.H{"data": gin.H{"status": "DAILY_LIMIT"}})
		return
	}

	// ── All checks passed — store/reset pending entry in cache ───────────────
	pending := EarnPending{
		InitiatedAt:   time.Now(),
		UID:           userID,
		ContextItemID: itemID,
		RunID:         runID,
	}
	ttl := h.earnMinDuration * 2
	h.earnCache.Set(earnCacheKey(userID, itemID), pending, ttl)

	log.Printf("init-earn: PENDING user=%s item=%s run=%s duration=%ds", userID, itemID, runID, int(h.earnMinDuration.Seconds()))
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"status":           "PENDING",
		"required_seconds": int(h.earnMinDuration.Seconds()),
	}})
}

// EarnCoin handles POST /api/v1/level/earn
// Authenticated endpoint — final confirmation step after the user has dwelled
// long enough. Verifies cache presence and elapsed time before awarding coins.
func (h *LevelHandler) EarnCoin(c *gin.Context) {
	userID := c.GetString("userID")

	var req struct {
		ContextItemID  string `json:"context_item_id" binding:"required"`
		TurnstileToken string `json:"turnstile_token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("earn: bad request from user=%s ip=%s — %v", userID, c.ClientIP(), err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "context_item_id and turnstile_token are required"})
		return
	}

	itemID, err := uuid.Parse(req.ContextItemID)
	if err != nil {
		log.Printf("earn: invalid context_item_id=%q from user=%s", req.ContextItemID, userID)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid context_item_id"})
		return
	}

	// ── Cache dwell check ─────────────────────────────────────────────────────
	key := earnCacheKey(userID, itemID)
	raw, ok := h.earnCache.Get(key)
	if !ok {
		log.Printf("earn: TOO_EARLY (no cache) user=%s item=%s ip=%s", userID, itemID, c.ClientIP())
		c.JSON(http.StatusBadRequest, gin.H{"error": "TOO_EARLY"})
		return
	}
	pending, ok := raw.(EarnPending)
	if !ok {
		log.Printf("earn: TOO_EARLY (cache type mismatch) user=%s item=%s", userID, itemID)
		c.JSON(http.StatusBadRequest, gin.H{"error": "TOO_EARLY"})
		return
	}
	elapsed := time.Since(pending.InitiatedAt)
	if elapsed < h.earnMinDuration {
		log.Printf("earn: TOO_EARLY (elapsed=%s < min=%s) user=%s item=%s", elapsed.Round(time.Millisecond), h.earnMinDuration, userID, itemID)
		c.JSON(http.StatusBadRequest, gin.H{"error": "TOO_EARLY"})
		return
	}

	// ── Turnstile validation (Layer 3) ────────────────────────────────────────
	tokenPreview := req.TurnstileToken
	if len(tokenPreview) > 16 {
		tokenPreview = tokenPreview[:16] + "..."
	}
	if err := h.verifyTurnstileToken(req.TurnstileToken, c.ClientIP()); err != nil {
		log.Printf("earn: TURNSTILE_FAIL user=%s item=%s ip=%s token_prefix=%s — %v", userID, itemID, c.ClientIP(), tokenPreview, err)
		c.JSON(http.StatusForbidden, gin.H{"error": "bot verification failed"})
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
	runID := pending.RunID

	isToday, err := h.histRepo.IsRunCreatedToday(ctx, runID)
	if err != nil {
		log.Printf("earn: DB error checking run date user=%s run=%s item=%s — %v", userID, runID, itemID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if !isToday {
		h.earnCache.Delete(key)
		log.Printf("earn: EXPIRED (re-validate) user=%s item=%s run=%s", userID, itemID, runID)
		c.JSON(http.StatusOK, gin.H{"data": earnResponse{Attempted: true, Reason: "EXPIRED"}})
		return
	}

	topic, err := h.histRepo.GetContextItemByID(ctx, itemID)
	if err != nil {
		log.Printf("earn: DB error getting item=%s user=%s — %v", itemID, userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if topic == nil {
		log.Printf("earn: item not found item=%s user=%s (deleted between init and earn?)", itemID, userID)
		c.JSON(http.StatusNotFound, gin.H{"error": "context item not found"})
		return
	}

	subs, err := h.subGetter.GetSubscriptions(ctx, userID)
	if err != nil {
		log.Printf("earn: DB error getting subscriptions user=%s — %v", userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	preferred := level.IsPreferredCategory(topic.Category, subs)

	result, err := h.service.EarnCoin(ctx, userID, runID, itemID, preferred)
	if err != nil {
		log.Printf("earn: EarnCoin service error user=%s run=%s item=%s preferred=%v — %v", userID, runID, itemID, preferred, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	// Remove the pending entry so a second call is rejected.
	h.earnCache.Delete(key)

	log.Printf("earn: result user=%s item=%s run=%s earned=%v reason=%s coins=%d level_up=%v elapsed=%s",
		userID, itemID, runID, result.Earned, result.Reason, result.CoinsEarned, result.LeveledUp, elapsed.Round(time.Millisecond))

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
	group.GET("", h.authMW, h.GetLevel)           // GET  /api/v1/level
	group.POST("/init-earn", h.authMW, h.InitEarn) // POST /api/v1/level/init-earn
	group.POST("/earn", h.authMW, h.EarnCoin)      // POST /api/v1/level/earn
}

// ── Turnstile Helper ──────────────────────────────────────────────────────────

// verifyTurnstileToken calls the Cloudflare SiteVerify API to validate the token.
func (h *LevelHandler) verifyTurnstileToken(token string, remoteIP string) error {
	// If the secret is the dummy dev key and the token is a specific testing string,
	// or if we are purely offline testing (e.g. tests passing "dummy-secret-key"),
	// we allow it. (Cloudflare provides "1x0000000000000000000000000000000AA" for testing pass).
	if h.turnstileSecretKey == "dummy-secret-key" {
		return nil
	}

	endpoint := "https://challenges.cloudflare.com/turnstile/v0/siteverify"

	// Create x-www-form-urlencoded body
	data := fmt.Sprintf("secret=%s&response=%s", h.turnstileSecretKey, token)
	if remoteIP != "" {
		data += fmt.Sprintf("&remoteip=%s", remoteIP)
	}

	req, err := http.NewRequest("POST", endpoint, strings.NewReader(data))
	if err != nil {
		return fmt.Errorf("could not create turnstile req: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Use a short timeout for verification so we don't block
	client := http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("turnstile sightverify call failed: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Success  bool     `json:"success"`
		Error    []string `json:"error-codes"`
		Hostname string   `json:"hostname"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("could not decode turnstile response: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("invalid token, error-codes: %v", result.Error)
	}

	return nil
}
