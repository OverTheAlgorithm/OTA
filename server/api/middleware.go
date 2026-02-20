package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"ota/auth"
	"ota/domain/user"
)

// LoggerMiddleware replaces gin.Default()'s logger. It adds user ID to every
// log line by best-effort parsing the JWT from the request cookie/header.
func LoggerMiddleware(jwtManager *auth.JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Best-effort: extract user ID from token without blocking the request.
		userID := "-"
		if tokenStr, err := c.Cookie("ota_token"); err == nil && tokenStr != "" {
			if claims, err := jwtManager.Validate(tokenStr); err == nil {
				userID = claims.UserID
			}
		} else if header := c.GetHeader("Authorization"); strings.HasPrefix(header, "Bearer ") {
			tokenStr = strings.TrimPrefix(header, "Bearer ")
			if claims, err := jwtManager.Validate(tokenStr); err == nil {
				userID = claims.UserID
			}
		}

		c.Next()

		fmt.Fprintf(gin.DefaultWriter, "[GIN] %s | %3d | %12s | %s | %s | %-7s %s\n",
			time.Now().Format("2006/01/02 - 15:04:05"),
			c.Writer.Status(),
			time.Since(start),
			c.ClientIP(),
			userID,
			c.Request.Method,
			c.Request.URL.Path,
		)
	}
}

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

// AdminMiddleware must run after AuthMiddleware (requires userID in context).
// Always checks DB so that role changes take effect immediately without re-login.
func AdminMiddleware(userRepo user.Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("userID")
		u, err := userRepo.FindByID(c.Request.Context(), userID)
		if err != nil || u.Role != "admin" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}

		c.Next()
	}
}
