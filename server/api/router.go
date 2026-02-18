package api

import (
	"github.com/gin-gonic/gin"
)

type RouteModule struct {
	GroupName   string
	Handler     RouteRegistrar
	Middlewares []gin.HandlerFunc
}

type RouteRegistrar interface {
	RegisterRoutes(group *gin.RouterGroup)
}

func NewRouter(apiPrefix, version string, frontendURL string, modules []RouteModule) *gin.Engine {
	r := gin.Default()
	r.Use(CORSMiddleware(frontendURL))

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
