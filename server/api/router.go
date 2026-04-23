package api

import (
	"github.com/gin-gonic/gin"
	limiter "github.com/ulule/limiter/v3"
	"ota/auth"
)

type RouteModule struct {
	GroupName   string
	Handler     RouteRegistrar
	Middlewares []gin.HandlerFunc
}

type RouteRegistrar interface {
	RegisterRoutes(group *gin.RouterGroup)
}

func NewRouter(apiPrefix, version string, frontendURL string, jwtManager *auth.JWTManager, ratePerMin int, rateLimitStore limiter.Store, modules []RouteModule) *gin.Engine {
	r := gin.New()

	// C1 fix: Trust only the X-Real-Client-IP header set by Caddy reverse proxy.
	// Caddy overwrites this header with the actual client IP ({remote_host}),
	// so attacker-supplied values are discarded. X-Forwarded-For is ignored entirely.
	_ = r.SetTrustedProxies(nil)
	r.TrustedPlatform = "X-Real-Client-IP"

	r.Use(gin.Recovery())
	r.Use(RequestIDMiddleware())
	r.Use(LoggerMiddleware(jwtManager))
	r.Use(CORSMiddleware(frontendURL))
	r.Use(CSRFMiddleware(frontendURL))
	r.Use(RateLimitMiddleware(ratePerMin, jwtManager, rateLimitStore))

	api := r.Group(apiPrefix + "/" + version)

	for _, module := range modules {
		group := api.Group("/" + module.GroupName)
		for _, mw := range module.Middlewares {
			group.Use(mw)
		}
		module.Handler.RegisterRoutes(group)
	}

	return r
}
