package api

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"ota/auth"
	"ota/domain/user"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	limiter "github.com/ulule/limiter/v3"
)

// RequestIDMiddleware ensures every request has a unique X-Request-ID header.
// It reads the incoming header if present (for upstream propagation), otherwise
// generates a new UUID. The ID is stored in the Gin context and set on the
// response header so clients and downstream services can correlate log entries.
func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Request-ID")
		if id == "" {
			id = uuid.New().String()
		}
		c.Set("request_id", id)
		c.Header("X-Request-ID", id)
		c.Next()
	}
}

// LoggerMiddleware replaces gin.Default()'s logger. It adds user ID and
// request ID to every log line by best-effort parsing the JWT from the
// request cookie/header.
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

		requestID := c.GetString("request_id")
		fmt.Fprintf(gin.DefaultWriter, "[GIN] %s | %3d | %12s | %s | %s | %-7s %s | req=%s\n",
			time.Now().Format("2006/01/02 - 15:04:05"),
			c.Writer.Status(),
			time.Since(start),
			c.ClientIP(),
			userID,
			c.Request.Method,
			c.Request.URL.Path,
			requestID,
		)
	}
}

func CORSMiddleware(frontendURL string) gin.HandlerFunc {
	// Extract allowed base domains from FRONTEND_URL (comma-separated).
	// Each entry like "https://wizletter.com" allows the domain itself
	// and all its subdomains (e.g. https://www.wizletter.com).
	type allowedDomain struct {
		scheme string
		host   string
	}
	var domains []allowedDomain
	for _, raw := range strings.Split(frontendURL, ",") {
		u := strings.TrimSpace(raw)
		if u == "" {
			continue
		}
		scheme := "https"
		host := u
		if strings.HasPrefix(u, "https://") {
			host = u[len("https://"):]
		} else if strings.HasPrefix(u, "http://") {
			scheme = "http"
			host = u[len("http://"):]
		}
		domains = append(domains, allowedDomain{scheme: scheme, host: host})
	}

	parsedDomains := ""
	for _, d := range domains {
		parsedDomains += "\n" + d.scheme + "://" + d.host + "\n"
	}
	slog.Info(fmt.Sprintf("[CORS middleware] Frontend URL: %s Parsed Domains: %s", frontendURL, parsedDomains))

	return cors.New(cors.Config{
		AllowOriginFunc: func(origin string) bool {
			for _, d := range domains {
				allowed := d.scheme + "://" + d.host
				if origin == allowed {
					return true
				}
				// Allow subdomains: origin ends with ".host" and has correct scheme
				suffix := "." + d.host
				if strings.HasPrefix(origin, d.scheme+"://") && strings.HasSuffix(origin, suffix) {
					return true
				}
			}
			slog.Warn(fmt.Sprintf("Not allowed origin request detected: %s", origin))
			return false
		},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Content-Type", "Authorization", "Idempotency-Key"},
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

// OptionalAuthMiddleware parses the JWT and sets userID/role in context if valid,
// but does NOT abort — the request proceeds regardless of auth status.
// Used for public endpoints that bundle user-specific data when logged in.
func OptionalAuthMiddleware(jwtManager *auth.JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr, _ := c.Cookie("ota_token")
		if tokenStr == "" {
			if h := c.GetHeader("Authorization"); strings.HasPrefix(h, "Bearer ") {
				tokenStr = strings.TrimPrefix(h, "Bearer ")
			}
		}
		if tokenStr != "" {
			if claims, err := jwtManager.Validate(tokenStr); err == nil {
				c.Set("userID", claims.UserID)
				c.Set("role", claims.Role)
			}
		}
		c.Next()
	}
}

// RequireRoleMiddleware enforces that the authenticated user holds a role at
// least as privileged as minRole. It must run after AuthMiddleware (which sets
// userID in the Gin context). The user's role is re-read from the database on
// every request so revocations take effect immediately without re-login.
//
// On success it stores the resolved user role under the "role" context key so
// downstream handlers can branch on it without an extra lookup.
func RequireRoleMiddleware(userRepo user.Repository, minRole string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("userID")
		u, err := userRepo.FindByID(c.Request.Context(), userID)
		if err != nil || !user.HasRoleAtLeast(u.Role, minRole) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		c.Set("role", u.Role)
		c.Next()
	}
}

// AdminMiddleware is a thin alias for RequireRoleMiddleware with the admin
// floor. It exists for callers that pre-date the role hierarchy and to make
// route registration self-documenting.
func AdminMiddleware(userRepo user.Repository) gin.HandlerFunc {
	return RequireRoleMiddleware(userRepo, user.RoleAdmin)
}

// EditorMiddleware allows requests from editors or admins.
func EditorMiddleware(userRepo user.Repository) gin.HandlerFunc {
	return RequireRoleMiddleware(userRepo, user.RoleEditor)
}

// CSRFMiddleware validates the Origin or Referer header for state-mutating
// requests (POST/PUT/PATCH/DELETE) to prevent cross-site request forgery.
// Requests with neither header are allowed (non-browser clients / curl).
// On mismatch a 403 is returned immediately.
func CSRFMiddleware(frontendURL string) gin.HandlerFunc {
	mutatingMethods := map[string]bool{
		http.MethodPost:   true,
		http.MethodPut:    true,
		http.MethodPatch:  true,
		http.MethodDelete: true,
	}

	return func(c *gin.Context) {
		if !mutatingMethods[c.Request.Method] {
			c.Next()
			return
		}

		origin := c.GetHeader("Origin")
		if origin != "" {
			if strings.TrimRight(origin, "/") != strings.TrimRight(frontendURL, "/") {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "origin not allowed"})
				return
			}
			c.Next()
			return
		}

		referer := c.GetHeader("Referer")
		if referer != "" {
			if !strings.HasPrefix(referer, frontendURL) {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "origin not allowed"})
				return
			}
			c.Next()
			return
		}

		// No Origin or Referer — allow (non-browser client)
		c.Next()
	}
}

// RateLimitMiddleware applies a sliding-window per-key rate limit.
// Authenticated users are keyed by user ID; anonymous requests by client IP.
// On internal limiter errors the request is allowed (fail-open).
func RateLimitMiddleware(ratePerMin int, jwtManager *auth.JWTManager, store limiter.Store) gin.HandlerFunc {
	rate := limiter.Rate{
		Period: time.Minute,
		Limit:  int64(ratePerMin),
	}
	instance := limiter.New(store, rate)

	return func(c *gin.Context) {
		key := resolveRateLimitKey(c, jwtManager)

		ctx, err := instance.Get(c.Request.Context(), key)
		if err != nil {
			slog.Error("rate-limit: limiter error (fail-open)", "key", key, "error", err)
			c.Next()
			return
		}

		if ctx.Reached {
			slog.Warn("rate-limit: request blocked", "key", key, "method", c.Request.Method, "path", c.Request.URL.Path, "limit", ctx.Limit, "remaining", ctx.Remaining)
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "too many requests"})
			return
		}

		// Log warning when approaching the limit (< 20% remaining)
		if ctx.Remaining < ctx.Limit/5 {
			slog.Warn("rate-limit: approaching limit", "key", key, "remaining", ctx.Remaining, "limit", ctx.Limit, "path", c.Request.URL.Path)
		}

		c.Next()
	}
}

// resolveRateLimitKey returns "user:<id>" for authenticated requests,
// "ip:<addr>" otherwise.
func resolveRateLimitKey(c *gin.Context, jwtManager *auth.JWTManager) string {
	tokenStr, err := c.Cookie("ota_token")
	if err != nil || tokenStr == "" {
		if header := c.GetHeader("Authorization"); strings.HasPrefix(header, "Bearer ") {
			tokenStr = strings.TrimPrefix(header, "Bearer ")
		}
	}

	if tokenStr != "" {
		if claims, err := jwtManager.Validate(tokenStr); err == nil {
			return "user:" + claims.UserID
		}
	}

	return "ip:" + c.ClientIP()
}
