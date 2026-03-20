package handler

import (
	"context"
	"strconv"

	"github.com/gin-gonic/gin"
)

// SubscriptionGetter retrieves a user's category subscriptions.
type SubscriptionGetter interface {
	GetSubscriptions(ctx context.Context, userID string) ([]string, error)
}

// parsePageParams extracts and validates limit and offset query parameters.
// If a value is missing or invalid, the provided defaults are used.
// limit is clamped to [1, maxLimit].
func parsePageParams(c *gin.Context, defaultLimit, maxLimit int) (limit, offset int) {
	limit = defaultLimit
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= maxLimit {
			limit = n
		}
	}
	offset = 0
	if v := c.Query("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	return limit, offset
}
