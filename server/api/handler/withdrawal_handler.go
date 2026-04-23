package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"ota/domain/apperr"
	"ota/domain/withdrawal"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// IdempotencyStore deduplicates requests using a distributed key-value store.
// SetNX sets key only if it doesn't exist and returns true when the key was newly
// created (i.e., the request is not a duplicate).
type IdempotencyStore interface {
	SetNX(ctx context.Context, key string, value string, ttl time.Duration) (bool, error)
}

const idempotencyTTL = 24 * time.Hour

type WithdrawalHandler struct {
	service     *withdrawal.Service
	authMW      gin.HandlerFunc
	idempotency IdempotencyStore
}

func NewWithdrawalHandler(service *withdrawal.Service, authMW gin.HandlerFunc) *WithdrawalHandler {
	return &WithdrawalHandler{service: service, authMW: authMW}
}

// WithIdempotencyStore sets the idempotency store used to deduplicate withdrawal requests.
func (h *WithdrawalHandler) WithIdempotencyStore(store IdempotencyStore) *WithdrawalHandler {
	h.idempotency = store
	return h
}

// ── Bank Account ────────────────────────────────────────────────────────────

func (h *WithdrawalHandler) GetBankAccount(c *gin.Context) {
	userID := c.GetString("userID")
	account, err := h.service.GetBankAccount(c.Request.Context(), userID)
	if err != nil {
		slog.Error("get bank account error", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": account}) // null if not registered
}

func (h *WithdrawalHandler) SaveBankAccount(c *gin.Context) {
	userID := c.GetString("userID")
	var req struct {
		BankName      string `json:"bank_name" binding:"required"`
		AccountNumber string `json:"account_number" binding:"required"`
		AccountHolder string `json:"account_holder" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bank_name, account_number, account_holder are required"})
		return
	}

	account := withdrawal.BankAccount{
		UserID:        userID,
		BankName:      strings.TrimSpace(req.BankName),
		AccountNumber: strings.TrimSpace(req.AccountNumber),
		AccountHolder: strings.TrimSpace(req.AccountHolder),
	}
	if err := h.service.SaveBankAccount(c.Request.Context(), account); err != nil {
		slog.Error("save bank account error", "error", err)
		var ve *apperr.ValidationError
		if errors.As(err, &ve) {
			c.JSON(http.StatusBadRequest, gin.H{"error": ve.Error()})
			return
		}
		slog.Error("save bank account error (non-validation)", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "계좌 저장에 실패했습니다"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": "ok"})
}

// ── User Withdrawal ─────────────────────────────────────────────────────────

func (h *WithdrawalHandler) RequestWithdrawal(c *gin.Context) {
	idempotencyKey := c.GetHeader("Idempotency-Key")
	if idempotencyKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Idempotency-Key header is required"})
		return
	}

	if h.idempotency != nil {
		redisKey := "idempotency:" + idempotencyKey
		isNew, err := h.idempotency.SetNX(c.Request.Context(), redisKey, "1", idempotencyTTL)
		if err != nil {
			slog.Error("idempotency check failed", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
		if !isNew {
			c.JSON(http.StatusConflict, gin.H{"error": "duplicate request"})
			return
		}
	}

	userID := c.GetString("userID")
	var req struct {
		Amount int `json:"amount" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "amount is required"})
		return
	}

	w, err := h.service.RequestWithdrawal(c.Request.Context(), userID, req.Amount)
	if err != nil {
		slog.Error("request withdrawal error", "error", err)
		var me *apperr.MinimumAmountError
		var ve *apperr.ValidationError
		switch {
		case errors.Is(err, apperr.ErrBankAccountRequired):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.As(err, &me):
			c.JSON(http.StatusBadRequest, gin.H{"error": me.Error()})
		case errors.As(err, &ve):
			c.JSON(http.StatusBadRequest, gin.H{"error": ve.Error()})
		case errors.Is(err, apperr.ErrInsufficientBalance):
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": w})
}

func (h *WithdrawalHandler) GetHistory(c *gin.Context) {
	userID := c.GetString("userID")
	limit, offset := parsePageParams(c, 20, 50)

	items, hasMore, err := h.service.GetUserHistory(c.Request.Context(), userID, limit, offset)
	if err != nil {
		slog.Error("get withdrawal history error", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items, "has_more": hasMore})
}

func (h *WithdrawalHandler) CancelWithdrawal(c *gin.Context) {
	userID := c.GetString("userID")
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if err := h.service.CancelWithdrawal(c.Request.Context(), userID, id); err != nil {
		slog.Error("cancel withdrawal error", "error", err)
		var ce *apperr.ConflictError
		switch {
		case errors.Is(err, apperr.ErrUnauthorized):
			c.JSON(http.StatusForbidden, gin.H{"error": "not authorized"})
		case errors.As(err, &ce):
			c.JSON(http.StatusBadRequest, gin.H{"error": ce.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": "ok"})
}

// ── Info ─────────────────────────────────────────────────────────────────────

func (h *WithdrawalHandler) GetInfo(c *gin.Context) {
	userID := c.GetString("userID")
	info, err := h.service.GetPreCheckInfo(c.Request.Context(), userID)
	if err != nil {
		slog.Error("get pre-check info error", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": info})
}

func (h *WithdrawalHandler) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/info", h.authMW, h.GetInfo)
	group.GET("/bank-account", h.authMW, h.GetBankAccount)
	group.PUT("/bank-account", h.authMW, h.SaveBankAccount)
	group.POST("/request", h.authMW, h.RequestWithdrawal)
	group.GET("/history", h.authMW, h.GetHistory)
	group.POST("/:id/cancel", h.authMW, h.CancelWithdrawal)
}
