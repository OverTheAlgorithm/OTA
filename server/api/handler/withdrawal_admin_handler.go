package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"ota/domain/apperr"
	"ota/domain/withdrawal"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type WithdrawalAdminHandler struct {
	service *withdrawal.Service
}

func NewWithdrawalAdminHandler(service *withdrawal.Service) *WithdrawalAdminHandler {
	return &WithdrawalAdminHandler{service: service}
}

// ListWithdrawals handles GET /api/v1/admin/withdrawals
func (h *WithdrawalAdminHandler) ListWithdrawals(c *gin.Context) {
	limit := 20
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}
	offset := 0
	if v := c.Query("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	status := c.Query("status") // "" = all

	items, total, err := h.service.ListAll(c.Request.Context(), withdrawal.ListFilter{
		Status: status,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		slog.Error("list withdrawals error", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items, "total": total})
}

// GetWithdrawalDetail handles GET /api/v1/admin/withdrawals/:id
func (h *WithdrawalAdminHandler) GetWithdrawalDetail(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	detail, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		slog.Error("get withdrawal detail error", "error", err)
		var nfe *apperr.NotFoundError
		if errors.As(err, &nfe) {
			c.JSON(http.StatusNotFound, gin.H{"error": nfe.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if detail == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "withdrawal not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": detail})
}

// ApproveWithdrawal handles POST /api/v1/admin/withdrawals/:id/approve
func (h *WithdrawalAdminHandler) ApproveWithdrawal(c *gin.Context) {
	adminID := c.GetString("userID")
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req struct {
		Note string `json:"note" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "note is required"})
		return
	}

	if err := h.service.ApproveWithdrawal(c.Request.Context(), adminID, id, req.Note); err != nil {
		slog.Error("approve withdrawal error", "error", err)
		var ve *apperr.ValidationError
		var ce *apperr.ConflictError
		switch {
		case errors.As(err, &ve):
			c.JSON(http.StatusBadRequest, gin.H{"error": ve.Error()})
		case errors.As(err, &ce):
			c.JSON(http.StatusConflict, gin.H{"error": ce.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": "ok"})
}

// RejectWithdrawal handles POST /api/v1/admin/withdrawals/:id/reject
func (h *WithdrawalAdminHandler) RejectWithdrawal(c *gin.Context) {
	adminID := c.GetString("userID")
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req struct {
		Note string `json:"note" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "note (rejection reason) is required"})
		return
	}

	if err := h.service.RejectWithdrawal(c.Request.Context(), adminID, id, req.Note); err != nil {
		slog.Error("reject withdrawal error", "error", err)
		var ve *apperr.ValidationError
		var ce *apperr.ConflictError
		switch {
		case errors.As(err, &ve):
			c.JSON(http.StatusBadRequest, gin.H{"error": ve.Error()})
		case errors.As(err, &ce):
			c.JSON(http.StatusConflict, gin.H{"error": ce.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": "ok"})
}

// UpdateNote handles PUT /api/v1/admin/withdrawals/transitions/:id/note
func (h *WithdrawalAdminHandler) UpdateNote(c *gin.Context) {
	adminID := c.GetString("userID")
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid transition id"})
		return
	}

	var req struct {
		Note string `json:"note" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "note is required"})
		return
	}

	if err := h.service.UpdateNote(c.Request.Context(), adminID, id, req.Note); err != nil {
		slog.Error("update note error", "error", err)
		var ve *apperr.ValidationError
		switch {
		case errors.As(err, &ve):
			c.JSON(http.StatusBadRequest, gin.H{"error": ve.Error()})
		case errors.Is(err, apperr.ErrUnauthorized):
			c.JSON(http.StatusForbidden, gin.H{"error": "can only edit your own notes"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": "ok"})
}

func (h *WithdrawalAdminHandler) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("", h.ListWithdrawals)
	group.GET("/:id", h.GetWithdrawalDetail)
	group.POST("/:id/approve", h.ApproveWithdrawal)
	group.POST("/:id/reject", h.RejectWithdrawal)
	group.PUT("/transitions/:id/note", h.UpdateNote)
}
