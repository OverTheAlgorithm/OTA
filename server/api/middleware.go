package api

import (
	"net/http"
	"strings"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"ota/auth"
	"ota/domain/user"
)

func CORSMiddleware(frontendURL string) gin.HandlerFunc {
	return cors.New(cors.Config{
		AllowOrigins:     []string{frontendURL},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
		MaxAge:           3600,
	})
}

func AuthMiddleware(jwtManager *auth.JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr, err := c.Cookie("ota_token")
		if err != nil || tokenStr == "" {
			header := c.GetHeader("Authorization")
			if strings.HasPrefix(header, "Bearer ") {
				tokenStr = strings.TrimPrefix(header, "Bearer ")
			}
		}

		if tokenStr == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		claims, err := jwtManager.Validate(tokenStr)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		c.Set("userID", claims.UserID)
		c.Set("role", claims.Role)
		c.Next()
	}
}

// AdminMiddleware must run after AuthMiddleware (requires userID and role in context).
// Fast path: reject immediately if JWT role is not "admin".
// DB check: if JWT says admin, verify role against DB to catch revocations.
func AdminMiddleware(userRepo user.Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get("role")
		if roleStr, ok := role.(string); !ok || roleStr != "admin" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}

		// Confirm current role from DB — prevents stale JWT from granting access after demotion
		userID := c.GetString("userID")
		u, err := userRepo.FindByID(c.Request.Context(), userID)
		if err != nil || u.Role != "admin" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}

		c.Next()
	}
}
