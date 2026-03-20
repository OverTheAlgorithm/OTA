package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RedisPinger is a minimal interface for checking Redis connectivity.
type RedisPinger interface {
	Ping(ctx context.Context) error
}

// HealthHandler serves liveness and readiness probes.
type HealthHandler struct {
	pool        *pgxpool.Pool
	redisPinger RedisPinger // optional; nil = Redis check skipped
}

func NewHealthHandler(pool *pgxpool.Pool) *HealthHandler {
	return &HealthHandler{pool: pool}
}

// WithRedisPinger attaches a Redis pinger for readiness checks.
func (h *HealthHandler) WithRedisPinger(p RedisPinger) *HealthHandler {
	h.redisPinger = p
	return h
}

// Live handles GET /health — liveness probe, always 200 if process is up.
func (h *HealthHandler) Live(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "ok",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// Ready handles GET /health/ready — readiness probe.
// Checks DB connectivity + pool stats and Redis (if configured).
// Returns 503 if any critical dependency is unhealthy.
func (h *HealthHandler) Ready(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	type depStatus struct {
		Status string `json:"status"`
		Error  string `json:"error,omitempty"`
	}

	deps := make(map[string]any)
	healthy := true

	// -- Database ------------------------------------------------------------
	dbStatus := depStatus{Status: "ok"}
	if err := h.pool.Ping(ctx); err != nil {
		dbStatus.Status = "unavailable"
		dbStatus.Error = err.Error()
		healthy = false
	}
	stats := h.pool.Stat()
	deps["database"] = gin.H{
		"status":            dbStatus.Status,
		"error":             dbStatus.Error,
		"total_connections": stats.TotalConns(),
		"idle_connections":  stats.IdleConns(),
		"in_use":            stats.TotalConns() - stats.IdleConns(),
		"max_connections":   stats.MaxConns(),
	}

	// -- Redis ---------------------------------------------------------------
	if h.redisPinger != nil {
		redisStatus := depStatus{Status: "ok"}
		if err := h.redisPinger.Ping(ctx); err != nil {
			redisStatus.Status = "unavailable"
			redisStatus.Error = err.Error()
			healthy = false
		}
		deps["redis"] = redisStatus
	}

	status := http.StatusOK
	statusStr := "ready"
	if !healthy {
		status = http.StatusServiceUnavailable
		statusStr = "unavailable"
	}

	c.JSON(status, gin.H{
		"status":      statusStr,
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
		"dependencies": deps,
	})
}
