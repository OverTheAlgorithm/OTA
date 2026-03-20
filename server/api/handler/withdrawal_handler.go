package handler

import (
	"log"
	"net/http"
	"strings"

	"ota/domain/withdrawal"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type WithdrawalHandler struct {
	service *withdrawal.Service
	authMW  gin.HandlerFunc
}

func NewWithdrawalHandler(service *withdrawal.Service, authMW gin.HandlerFunc) *WithdrawalHandler {
	return &WithdrawalHandler{service: service, authMW: authMW}
}

// ── Bank Account ────────────────────────────────────────────────────────────

func (h *WithdrawalHandler) GetBankAccount(c *gin.Context) {
	userID := c.GetString("userID")
	account, err := h.service.GetBankAccount(c.Request.Context(), userID)
	if err != nil {
		log.Printf("get bank account error: %v", err)
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
		log.Printf("save bank account error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": "ok"})
}

// ── User Withdrawal ─────────────────────────────────────────────────────────

func (h *WithdrawalHandler) RequestWithdrawal(c *gin.Context) {
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
		log.Printf("request withdrawal error: %v", err)
		// Return user-friendly error for known validation failures
		errMsg := err.Error()
		if strings.Contains(errMsg, "minimum") || strings.Contains(errMsg, "insufficient") || strings.Contains(errMsg, "bank account") {
			c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": w})
}

func (h *WithdrawalHandler) GetHistory(c *gin.Context) {
	userID := c.GetString("userID")
	limit, offset := parsePageParams(c, 20, 50)

	items, hasMore, err := h.service.GetUserHistory(c.Request.Context(), userID, limit, offset)
	if err != nil {
		log.Printf("get withdrawal history error: %v", err)
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
		log.Printf("cancel withdrawal error: %v", err)
		errMsg := err.Error()
		if strings.Contains(errMsg, "not authorized") {
			c.JSON(http.StatusForbidden, gin.H{"error": "not authorized"})
			return
		}
		if strings.Contains(errMsg, "can only cancel") {
			c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": "ok"})
}

// ── Info ─────────────────────────────────────────────────────────────────────

func (h *WithdrawalHandler) GetInfo(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"min_withdrawal_amount": h.service.GetMinWithdrawalAmount(),
	}})
}

func (h *WithdrawalHandler) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/info", h.authMW, h.GetInfo)
	group.GET("/bank-account", h.authMW, h.GetBankAccount)
	group.PUT("/bank-account", h.authMW, h.SaveBankAccount)
	group.POST("/request", h.authMW, h.RequestWithdrawal)
	group.GET("/history", h.authMW, h.GetHistory)
	group.POST("/:id/cancel", h.authMW, h.CancelWithdrawal)
}
