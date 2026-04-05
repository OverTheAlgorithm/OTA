package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
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
	InitiatedAt   time.Time `json:"initiated_at"`
	UID           string    `json:"uid"`
	ContextItemID uuid.UUID `json:"context_item_id"`
	RunID         uuid.UUID `json:"run_id"`
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
	earnCacheRetries   int
	earnMinDuration    time.Duration
	turnstileSecretKey string
	authMW             gin.HandlerFunc
}

func NewLevelHandler(
	service *level.Service,
	histRepo collector.HistoryRepository,
	subGetter SubscriptionGetter,
	earnCache cache.Cache,
	earnCacheRetries int,
	earnMinDuration time.Duration,
	turnstileSecretKey string,
	authMW gin.HandlerFunc,
) *LevelHandler {
	if earnCacheRetries < 1 {
		earnCacheRetries = 1
	}
	return &LevelHandler{
		service:            service,
		histRepo:           histRepo,
		subGetter:          subGetter,
		earnCache:          earnCache,
		earnCacheRetries:   earnCacheRetries,
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
		slog.Error("get level error", "error", err)
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
		slog.Warn("init-earn: bad request", "user_id", userID, "ip", c.ClientIP(), "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "context_item_id is required"})
		return
	}

	itemID, err := uuid.Parse(req.ContextItemID)
	if err != nil {
		slog.Warn("init-earn: invalid context_item_id", "item_id", req.ContextItemID, "user_id", userID)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid context_item_id"})
		return
	}

	ctx := c.Request.Context()

	// ── Gate check 1: context item must exist ────────────────────────────────
	topic, err := h.histRepo.GetContextItemByID(ctx, itemID)
	if err != nil {
		slog.Error("init-earn: DB error getting item", "item_id", itemID, "user_id", userID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if topic == nil {
		slog.Warn("init-earn: item not found", "item_id", itemID, "user_id", userID)
		c.JSON(http.StatusNotFound, gin.H{"error": "context item not found"})
		return
	}

	runID := topic.RunID

	// ── Gate check 2: must not already have earned for this run+item ─────────
	earned, err := h.service.HasEarned(ctx, userID, runID, itemID)
	if err != nil {
		slog.Error("init-earn: DB error checking has-earned", "user_id", userID, "run_id", runID, "item_id", itemID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if earned {
		slog.Info("init-earn: DUPLICATE", "user_id", userID, "item_id", itemID, "run_id", runID)
		c.JSON(http.StatusOK, gin.H{"data": gin.H{"status": "DUPLICATE"}})
		return
	}

	// ── Gate check 3: run must be from today ──────────────────────────────────
	isToday, err := h.histRepo.IsRunCreatedToday(ctx, runID)
	if err != nil {
		slog.Error("init-earn: DB error checking run date", "run_id", runID, "user_id", userID, "item_id", itemID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if !isToday {
		slog.Info("init-earn: EXPIRED", "user_id", userID, "item_id", itemID, "run_id", runID)
		c.JSON(http.StatusOK, gin.H{"data": gin.H{"status": "EXPIRED"}})
		return
	}

	// ── Gate check 4: daily coin limit ───────────────────────────────────────
	limited, err := h.service.IsAtDailyLimit(ctx, userID)
	if err != nil {
		slog.Error("init-earn: DB error checking daily limit", "user_id", userID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if limited {
		slog.Info("init-earn: DAILY_LIMIT", "user_id", userID, "item_id", itemID)
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
	key := earnCacheKey(userID, itemID)

	var cacheErr error
	for attempt := 1; attempt <= h.earnCacheRetries; attempt++ {
		if cacheErr = h.earnCache.Set(key, pending, ttl); cacheErr == nil {
			break
		}
		slog.Warn("init-earn: cache set failed", "attempt", attempt, "max", h.earnCacheRetries, "user_id", userID, "item_id", itemID, "error", cacheErr)
		if attempt < h.earnCacheRetries {
			time.Sleep(50 * time.Millisecond * time.Duration(attempt))
		}
	}
	if cacheErr != nil {
		slog.Error("init-earn: cache set failed after retries", "retries", h.earnCacheRetries, "user_id", userID, "item_id", itemID, "error", cacheErr)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	slog.Info("init-earn: PENDING", "user_id", userID, "item_id", itemID, "run_id", runID, "required_seconds", int(h.earnMinDuration.Seconds()))
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
		slog.Warn("earn: bad request", "user_id", userID, "ip", c.ClientIP(), "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "context_item_id and turnstile_token are required"})
		return
	}

	itemID, err := uuid.Parse(req.ContextItemID)
	if err != nil {
		slog.Warn("earn: invalid context_item_id", "item_id", req.ContextItemID, "user_id", userID)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid context_item_id"})
		return
	}

	// ── Cache dwell check ─────────────────────────────────────────────────────
	key := earnCacheKey(userID, itemID)
	pending, ok := cache.GetTyped[EarnPending](h.earnCache, key)
	if !ok {
		slog.Warn("earn: EARN_NOT_INITIATED (no cache entry)", "user_id", userID, "item_id", itemID, "ip", c.ClientIP())
		c.JSON(http.StatusBadRequest, gin.H{"error": "EARN_NOT_INITIATED"})
		return
	}
	elapsed := time.Since(pending.InitiatedAt)
	if elapsed < h.earnMinDuration {
		slog.Warn("earn: TOO_EARLY", "elapsed", elapsed.Round(time.Millisecond), "min_duration", h.earnMinDuration, "user_id", userID, "item_id", itemID)
		c.JSON(http.StatusBadRequest, gin.H{"error": "TOO_EARLY"})
		return
	}

	// ── Turnstile validation (Layer 3) ────────────────────────────────────────
	slog.Info("earn: verifying turnstile", "user_id", userID, "token_len", len(req.TurnstileToken))
	if err := h.verifyTurnstileToken(req.TurnstileToken, c.ClientIP()); err != nil {
		slog.Warn("earn: turnstile verification failed", "user_id", userID, "item_id", itemID, "ip", c.ClientIP(), "error", err)
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
		slog.Error("earn: DB error checking run date", "user_id", userID, "run_id", runID, "item_id", itemID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if !isToday {
		h.earnCache.Delete(key)
		slog.Info("earn: EXPIRED (re-validate)", "user_id", userID, "item_id", itemID, "run_id", runID)
		c.JSON(http.StatusOK, gin.H{"data": earnResponse{Attempted: true, Reason: "EXPIRED"}})
		return
	}

	topic, err := h.histRepo.GetContextItemByID(ctx, itemID)
	if err != nil {
		slog.Error("earn: DB error getting item", "item_id", itemID, "user_id", userID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if topic == nil {
		slog.Warn("earn: item not found", "item_id", itemID, "user_id", userID)
		c.JSON(http.StatusNotFound, gin.H{"error": "context item not found"})
		return
	}

	subs, err := h.subGetter.GetSubscriptions(ctx, userID)
	if err != nil {
		slog.Error("earn: DB error getting subscriptions", "user_id", userID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	preferred := level.IsPreferredTopic(topic.Priority, topic.Category, subs)

	result, err := h.service.EarnCoin(ctx, userID, runID, itemID, preferred)
	if err != nil {
		slog.Error("earn: EarnCoin service error", "user_id", userID, "run_id", runID, "item_id", itemID, "preferred", preferred, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	// Remove the pending entry so a second call is rejected.
	h.earnCache.Delete(key)

	slog.Info("earn: result", "user_id", userID, "item_id", itemID, "run_id", runID, "earned", result.Earned, "reason", result.Reason, "coins", result.CoinsEarned, "leveled_up", result.LeveledUp, "elapsed", elapsed.Round(time.Millisecond))

	c.JSON(http.StatusOK, gin.H{"data": earnResponse{
		Attempted:   true,
		Earned:      result.Earned,
		Reason:      result.Reason,
		CoinsEarned: result.CoinsEarned,
		LeveledUp:   result.LeveledUp,
		NewLevel:    result.Level,
	}})
}

// BatchEarnStatus handles POST /api/v1/level/batch-earn-status
// Returns the earn status for a batch of context item IDs (max 50).
func (h *LevelHandler) BatchEarnStatus(c *gin.Context) {
	userID := c.GetString("userID")

	var req struct {
		ContextItemIDs []string `json:"context_item_ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "context_item_ids is required"})
		return
	}
	if len(req.ContextItemIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "at least one item required"})
		return
	}
	if len(req.ContextItemIDs) > 50 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "max 50 items"})
		return
	}

	// Parse UUIDs
	itemIDs := make([]uuid.UUID, 0, len(req.ContextItemIDs))
	for _, idStr := range req.ContextItemIDs {
		id, err := uuid.Parse(idStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid id: %s", idStr)})
			return
		}
		itemIDs = append(itemIDs, id)
	}

	ctx := c.Request.Context()

	// 1. Get item metadata (category, priority, run_id)
	itemMap, err := h.histRepo.GetItemCategoryMap(ctx, itemIDs)
	if err != nil {
		slog.Error("batch-earn-status: GetItemCategoryMap error", "user_id", userID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	// 2. Get earned item IDs
	earnedIDs, err := h.service.GetEarnedItemIDs(ctx, userID, itemIDs)
	if err != nil {
		slog.Error("batch-earn-status: GetEarnedItemIDs error", "user_id", userID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	earnedSet := make(map[uuid.UUID]bool, len(earnedIDs))
	for _, id := range earnedIDs {
		earnedSet[id] = true
	}

	// 3. Check which run_ids are from today
	uniqueRunIDs := make(map[uuid.UUID]bool)
	for _, meta := range itemMap {
		uniqueRunIDs[meta.RunID] = false
	}
	for runID := range uniqueRunIDs {
		isToday, err := h.histRepo.IsRunCreatedToday(ctx, runID)
		if err != nil {
			slog.Warn("batch-earn-status: IsRunCreatedToday error", "run_id", runID, "error", err)
			continue
		}
		uniqueRunIDs[runID] = isToday
	}

	// 4. Check daily limit
	atDailyLimit, err := h.service.IsAtDailyLimit(ctx, userID)
	if err != nil {
		slog.Warn("batch-earn-status: IsAtDailyLimit error", "user_id", userID, "error", err)
		atDailyLimit = false
	}

	// 5. Get subscriptions
	subs, err := h.subGetter.GetSubscriptions(ctx, userID)
	if err != nil {
		slog.Warn("batch-earn-status: GetSubscriptions error", "user_id", userID, "error", err)
		subs = nil
	}

	// Build response
	type itemStatus struct {
		ID     string `json:"id"`
		Status string `json:"status"`
		Coins  int    `json:"coins"`
	}

	results := make([]itemStatus, 0, len(itemIDs))
	for _, id := range itemIDs {
		meta, exists := itemMap[id]
		if !exists {
			results = append(results, itemStatus{ID: id.String(), Status: "NOT_FOUND", Coins: 0})
			continue
		}

		if earnedSet[id] {
			results = append(results, itemStatus{ID: id.String(), Status: "DUPLICATE", Coins: 0})
			continue
		}

		isToday := uniqueRunIDs[meta.RunID]
		if !isToday {
			results = append(results, itemStatus{ID: id.String(), Status: "EXPIRED", Coins: 0})
			continue
		}

		if atDailyLimit {
			results = append(results, itemStatus{ID: id.String(), Status: "DAILY_LIMIT", Coins: 0})
			continue
		}

		preferred := level.IsPreferredTopic(meta.Priority, meta.Category, subs)
		coins := level.CalcCoins(preferred)
		results = append(results, itemStatus{ID: id.String(), Status: "PENDING", Coins: coins})
	}

	c.JSON(http.StatusOK, gin.H{"data": results})
}

func (h *LevelHandler) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("", h.authMW, h.GetLevel)                          // GET  /api/v1/level
	group.POST("/init-earn", h.authMW, h.InitEarn)                // POST /api/v1/level/init-earn
	group.POST("/earn", h.authMW, h.EarnCoin)                     // POST /api/v1/level/earn
	group.POST("/batch-earn-status", h.authMW, h.BatchEarnStatus) // POST /api/v1/level/batch-earn-status
}

// ── Turnstile Helper ──────────────────────────────────────────────────────────

// isTurnstileTestKey returns true when the configured secret is a known test/dev key.
func (h *LevelHandler) isTurnstileTestKey() bool {
	return h.turnstileSecretKey == "dummy-secret-key" ||
		strings.HasPrefix(h.turnstileSecretKey, "1x000000000000000000000000000000")
}

// verifyTurnstileToken calls the Cloudflare SiteVerify API to validate the token.
func (h *LevelHandler) verifyTurnstileToken(token string, remoteIP string) error {
	if h.isTurnstileTestKey() {
		return nil
	}

	endpoint := "https://challenges.cloudflare.com/turnstile/v0/siteverify"

	// Create x-www-form-urlencoded body (url.Values handles proper encoding)
	form := url.Values{
		"secret":   {h.turnstileSecretKey},
		"response": {token},
	}
	if remoteIP != "" {
		form.Set("remoteip", remoteIP)
	}

	req, err := http.NewRequest("POST", endpoint, strings.NewReader(form.Encode()))
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
