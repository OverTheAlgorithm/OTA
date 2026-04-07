package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"ota/domain/apperr"
	"ota/domain/push"
	"ota/scheduler"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	maxDataSizeBytes = 4 * 1024 // 4KB
	maxTitleLen      = 100
	maxBodyLen       = 500
	maxLinkLen       = 500
)

// PushAdminHandler handles admin CRUD and execution of scheduled push notifications.
type PushAdminHandler struct {
	scheduledSvc  *push.ScheduledService
	pushScheduler *scheduler.PushScheduler
}

// NewPushAdminHandler creates a new PushAdminHandler.
func NewPushAdminHandler(svc *push.ScheduledService, ps *scheduler.PushScheduler) *PushAdminHandler {
	return &PushAdminHandler{scheduledSvc: svc, pushScheduler: ps}
}

type scheduledPushRequest struct {
	Title       string         `json:"title"`
	Body        string         `json:"body"`
	Link        string         `json:"link"`
	Data        map[string]any `json:"data"`
	ScheduledAt *time.Time     `json:"scheduled_at"`
}

func (r *scheduledPushRequest) validate(requireFutureScheduledAt bool) (string, bool) {
	if r.Title == "" {
		return "title is required", false
	}
	if len(r.Title) > maxTitleLen {
		return "title must be 100 characters or fewer", false
	}
	if r.Body == "" {
		return "body is required", false
	}
	if len(r.Body) > maxBodyLen {
		return "body must be 500 characters or fewer", false
	}
	if len(r.Link) > maxLinkLen {
		return "link must be 500 characters or fewer", false
	}
	if r.Data != nil {
		encoded, err := json.Marshal(r.Data)
		if err != nil {
			return "data is invalid JSON", false
		}
		if len(encoded) > maxDataSizeBytes {
			return "data must be 4KB or fewer", false
		}
	}
	if requireFutureScheduledAt && r.ScheduledAt != nil && !r.ScheduledAt.After(time.Now()) {
		return "scheduled_at must be in the future", false
	}
	return "", true
}

// RegisterRoutes registers all push admin routes under the given group.
func (h *PushAdminHandler) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("", h.ListScheduledPushes)
	group.POST("", h.CreateScheduledPush)
	group.PUT("/:id", h.UpdateScheduledPush)
	group.DELETE("/:id", h.DeleteScheduledPush)
	group.POST("/:id/send", h.ExecuteScheduledPush)
}

// ListScheduledPushes handles GET /api/v1/admin/push
func (h *PushAdminHandler) ListScheduledPushes(c *gin.Context) {
	var statusFilter *string
	if s := c.Query("status"); s != "" {
		statusFilter = &s
	}

	pushes, err := h.scheduledSvc.List(c.Request.Context(), statusFilter)
	if err != nil {
		slog.Error("list scheduled pushes error", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": pushes})
}

// CreateScheduledPush handles POST /api/v1/admin/push
func (h *PushAdminHandler) CreateScheduledPush(c *gin.Context) {
	var req scheduledPushRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if msg, ok := req.validate(true); !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}

	createdBy := c.GetString("userID")
	created, err := h.scheduledSvc.Create(c.Request.Context(), req.Title, req.Body, req.Link, req.Data, req.ScheduledAt, createdBy)
	if err != nil {
		slog.Error("create scheduled push error", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	if req.ScheduledAt != nil {
		if err := h.pushScheduler.Schedule(created); err != nil {
			slog.Error("schedule push timer error", "id", created.ID, "error", err)
		}
	}

	c.JSON(http.StatusCreated, gin.H{"data": created})
}

// UpdateScheduledPush handles PUT /api/v1/admin/push/:id
func (h *PushAdminHandler) UpdateScheduledPush(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req scheduledPushRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if msg, ok := req.validate(false); !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}

	updated, err := h.scheduledSvc.Update(c.Request.Context(), id, req.Title, req.Body, req.Link, req.Data, req.ScheduledAt)
	if err != nil {
		slog.Error("update scheduled push error", "id", id, "error", err)
		var nfe *apperr.NotFoundError
		switch {
		case errors.As(err, &nfe):
			c.JSON(http.StatusNotFound, gin.H{"error": nfe.Error()})
		case err.Error() == "push notification is not in pending status":
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		}
		return
	}

	// Cancel old timer and re-schedule with new time.
	h.pushScheduler.Unschedule(id)
	if req.ScheduledAt != nil {
		if err := h.pushScheduler.Schedule(updated); err != nil {
			slog.Error("reschedule push timer error", "id", id, "error", err)
		}
	}

	c.JSON(http.StatusOK, gin.H{"data": updated})
}

// DeleteScheduledPush handles DELETE /api/v1/admin/push/:id
func (h *PushAdminHandler) DeleteScheduledPush(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	h.pushScheduler.Unschedule(id)

	if err := h.scheduledSvc.Delete(c.Request.Context(), id); err != nil {
		slog.Error("delete scheduled push error", "id", id, "error", err)
		var nfe *apperr.NotFoundError
		switch {
		case errors.As(err, &nfe):
			c.JSON(http.StatusNotFound, gin.H{"error": nfe.Error()})
		case err.Error() == "push notification is not in pending status":
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": "ok"})
}

// ExecuteScheduledPush handles POST /api/v1/admin/push/:id/send
func (h *PushAdminHandler) ExecuteScheduledPush(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	// Cancel scheduled timer before sending to prevent double-send.
	h.pushScheduler.Unschedule(id)

	// Send only to the requesting admin's devices (test send).
	userID := c.GetString("userID")
	if err := h.scheduledSvc.ExecuteNow(c.Request.Context(), id, userID); err != nil {
		slog.Error("execute scheduled push error", "id", id, "error", err)
		var nfe *apperr.NotFoundError
		if errors.As(err, &nfe) {
			c.JSON(http.StatusNotFound, gin.H{"error": nfe.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": "ok"})
}
